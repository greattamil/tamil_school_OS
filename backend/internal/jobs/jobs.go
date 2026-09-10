// Package jobs is the durable, PostgreSQL-backed background job queue (PRD 8.3).
// Redis is deliberately not used here: a container restart or OOM kill against an
// in-memory queue silently discards queued work, and for this system that means
// unsent absence alerts and unprocessed payment webhooks with nothing to indicate
// anything was lost. This table survives everything the database does, and
// Enqueue can run inside the same transaction as the write that produced the job
// (see EnqueueTx), so "the payment was recorded but no receipt was ever queued"
// cannot happen.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Job struct {
	ID       string
	Kind     string
	Payload  []byte
	Attempts int
}

// EnqueueTx inserts a job using tx, so it commits or rolls back with whatever
// write produced it. This is the transactional-enqueue property PRD 8.3 calls
// out: a payment and its receipt notification either both happen or neither does.
func EnqueueTx(ctx context.Context, tx pgx.Tx, kind string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO jobs (kind, payload) VALUES ($1, $2)`, kind, body)
	return err
}

// Enqueue inserts a job outside any existing transaction (its own single-statement
// transaction). Prefer EnqueueTx when the job is a direct consequence of another
// write.
func Enqueue(ctx context.Context, pool *pgxpool.Pool, kind string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO jobs (kind, payload) VALUES ($1, $2)`, kind, body)
	return err
}

type Handler func(ctx context.Context, job Job) error

// Worker claims and processes jobs with SELECT ... FOR UPDATE SKIP LOCKED (PRD
// 8.3), so multiple worker processes can run concurrently without claiming the
// same job twice and without blocking on rows another worker already holds.
type Worker struct {
	pool     *pgxpool.Pool
	id       string
	handlers map[string]Handler
}

func NewWorker(pool *pgxpool.Pool, id string) *Worker {
	return &Worker{pool: pool, id: id, handlers: make(map[string]Handler)}
}

func (w *Worker) Register(kind string, h Handler) {
	w.handlers[kind] = h
}

// RunOnce claims and processes up to batchSize claimable jobs, returning how many
// it processed. Intended to be called in a loop with a short sleep between empty
// batches; kept as a single-pass method rather than an internal loop so callers
// (and tests) control the polling cadence.
func (w *Worker) RunOnce(ctx context.Context, batchSize int) (int, error) {
	// SELECT ... FOR UPDATE SKIP LOCKED only holds its lock for the life of a
	// transaction. Claiming and marking-processing must therefore happen as one
	// atomic statement (a single implicit transaction) -- two separate
	// statements would let the lock release between them, and a second worker
	// could claim the same job before the first marks it processing.
	rows, err := w.pool.Query(ctx, `
		WITH claimed AS (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_after <= now()
			ORDER BY run_after
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs SET status = 'processing', claimed_at = now(), claimed_by = $2
		WHERE id IN (SELECT id FROM claimed)
		RETURNING id, kind, payload, attempts
	`, batchSize, w.id)
	if err != nil {
		return 0, fmt.Errorf("claim jobs: %w", err)
	}

	type claimed struct {
		id       string
		kind     string
		payload  []byte
		attempts int
	}
	var toProcess []claimed
	for rows.Next() {
		var j claimed
		if err := rows.Scan(&j.id, &j.kind, &j.payload, &j.attempts); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan claimed job: %w", err)
		}
		toProcess = append(toProcess, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, j := range toProcess {
		handler, ok := w.handlers[j.kind]
		if !ok {
			w.fail(ctx, j.id, j.attempts+1, fmt.Sprintf("no handler registered for kind %q", j.kind))
			continue
		}

		err := handler(ctx, Job{ID: j.id, Kind: j.kind, Payload: j.payload, Attempts: j.attempts})
		if err != nil {
			w.fail(ctx, j.id, j.attempts+1, err.Error())
			continue
		}

		if _, err := w.pool.Exec(ctx, `UPDATE jobs SET status = 'done', completed_at = now() WHERE id = $1`, j.id); err != nil {
			return len(toProcess), fmt.Errorf("mark job done: %w", err)
		}
	}

	return len(toProcess), nil
}

// fail records the failure and either schedules a retry with backoff or, past
// max_attempts, moves the job to a terminal failed state -- visible on an
// operations screen, never silently dropped (PRD 8.3).
func (w *Worker) fail(ctx context.Context, jobID string, attempts int, errMsg string) {
	backoff := time.Duration(attempts*attempts) * time.Second
	_, _ = w.pool.Exec(ctx, `
		UPDATE jobs SET
			attempts = $1,
			last_error = $2,
			status = CASE WHEN $1 >= max_attempts THEN 'failed' ELSE 'pending' END,
			run_after = now() + $3::interval
		WHERE id = $4
	`, attempts, errMsg, backoff.String(), jobID)
}
