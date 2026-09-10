package attendance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrNotFound                        = errors.New("attendance: not found")
	ErrPastDateRequiresOfficeAuthority = errors.New("attendance: correcting a prior day requires office admin authority")
)

// istLocation is used for "what is today" throughout this package. Every school
// this product targets is in India; hardcoding the offset is simpler and more
// correct than trusting the server's configured timezone, and avoids pulling in
// the IANA tzdata dependency for a single fixed offset that never changes.
var istLocation = time.FixedZone("IST", 5*3600+30*60)

func today() time.Time {
	now := time.Now().In(istLocation)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Sync applies a batch of per-student edits for one section+date register.
// Each edit is resolved independently (PRD 4.2.5 point 3): a conflict on one
// student never blocks the other 49 from applying. All of it runs inside a
// single transaction for atomicity of the batch's outcome, not because the
// individual decisions depend on each other.
func (r *Repository) Sync(ctx context.Context, sectionID uuid.UUID, date time.Time, edits []EntryEdit) (SyncResult, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return SyncResult{}, db.ErrNoTenant
	}
	actorID, _ := tenancy.UserID(ctx)
	actorRole, _ := tenancy.Role(ctx)

	// PRD 4.2.2: "Corrections to prior days require office admin authority."
	// This is a request-level gate on who may touch a past register at all,
	// distinct from the per-entry conflict resolution below, which handles
	// concurrent edits regardless of date.
	if date.Before(today()) && actorRole == "teacher" {
		return SyncResult{}, ErrPastDateRequiresOfficeAuthority
	}

	var result SyncResult
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		registerID, academicYearID, err := findOrCreateRegister(ctx, tx, schoolID, sectionID, date)
		if err != nil {
			return fmt.Errorf("find or create register: %w", err)
		}
		result.RegisterID = registerID

		for _, edit := range edits {
			res, err := syncOneEntry(ctx, tx, schoolID, registerID, academicYearID, date, edit, actorID, actorRole)
			if err != nil {
				return fmt.Errorf("sync entry %s: %w", edit.EnrollmentID, err)
			}
			result.Results = append(result.Results, res)
		}
		return nil
	})
	if err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func findOrCreateRegister(ctx context.Context, tx pgx.Tx, schoolID, sectionID uuid.UUID, date time.Time) (registerID, academicYearID uuid.UUID, err error) {
	err = tx.QueryRow(ctx, `
		SELECT id, academic_year_id FROM attendance_registers
		WHERE school_id = $1 AND section_id = $2 AND date = $3
	`, schoolID, sectionID, date).Scan(&registerID, &academicYearID)
	if err == nil {
		return registerID, academicYearID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, err
	}

	if err := tx.QueryRow(ctx, `SELECT academic_year_id FROM classes c JOIN sections s ON s.class_id = c.id WHERE s.id = $1`, sectionID).Scan(&academicYearID); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("resolve academic year for section: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO attendance_registers (school_id, academic_year_id, section_id, date)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, schoolID, academicYearID, sectionID, date).Scan(&registerID)
	return registerID, academicYearID, err
}

func syncOneEntry(ctx context.Context, tx pgx.Tx, schoolID, registerID, academicYearID uuid.UUID, date time.Time, edit EntryEdit, actorID uuid.UUID, actorRole string) (EntryResult, error) {
	var (
		entryID       uuid.UUID
		currentRev    int
		currentRole   string
		currentStatus Status
		currentReason *Reason
		exists        bool
	)

	err := tx.QueryRow(ctx, `
		SELECT id, server_revision, recorded_role, status, reason
		FROM attendance_entries
		WHERE school_id = $1 AND enrollment_id = $2 AND date = $3
		FOR UPDATE
	`, schoolID, edit.EnrollmentID, date).Scan(&entryID, &currentRev, &currentRole, &currentStatus, &currentReason)
	switch {
	case err == nil:
		exists = true
	case errors.Is(err, pgx.ErrNoRows):
		exists = false
	default:
		return EntryResult{}, err
	}

	if exists {
		var lastCounter int64
		lookupErr := tx.QueryRow(ctx, `
			SELECT last_counter FROM attendance_entry_device_counters WHERE entry_id = $1 AND device_id = $2
		`, entryID, edit.DeviceID).Scan(&lastCounter)
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return EntryResult{}, lookupErr
		}
		if lookupErr == nil && isReplay(lastCounter, edit.LocalCounter) {
			if err := recordRejection(ctx, tx, schoolID, entryID, actorID, actorRole, edit, currentStatus, currentReason, "stale or duplicate delivery from the same device"); err != nil {
				return EntryResult{}, err
			}
			return EntryResult{
				EnrollmentID: edit.EnrollmentID, Outcome: OutcomeInvalid,
				ServerRevision: currentRev, CurrentStatus: currentStatus, CurrentReason: currentReason,
				Message: "stale or duplicate delivery from the same device",
			}, nil
		}
	}

	outcome := resolveConflict(currentRev, currentRole, edit.BaseRevision, actorRole)

	if outcome == OutcomeSuperseded {
		if err := recordRejection(ctx, tx, schoolID, entryID, actorID, actorRole, edit, currentStatus, currentReason, "a newer or higher-authority edit already applied"); err != nil {
			return EntryResult{}, err
		}
		return EntryResult{
			EnrollmentID: edit.EnrollmentID, Outcome: OutcomeSuperseded,
			ServerRevision: currentRev, CurrentStatus: currentStatus, CurrentReason: currentReason,
			Message: "a newer or higher-authority edit already applied",
		}, nil
	}

	newRevision := currentRev + 1
	if !exists {
		newRevision = 1
		err = tx.QueryRow(ctx, `
			INSERT INTO attendance_entries (school_id, register_id, enrollment_id, date, status, reason, server_revision, recorded_by, recorded_role, client_recorded_at)
			VALUES ($1,$2,$3,$4,$5,$6,1,$7,$8,$9)
			RETURNING id
		`, schoolID, registerID, edit.EnrollmentID, date, edit.Status, edit.Reason, actorID, actorRole, edit.ClientTime).Scan(&entryID)
		if err != nil {
			return EntryResult{}, err
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE attendance_entries
			SET status = $1, reason = $2, server_revision = $3, recorded_by = $4, recorded_role = $5, updated_at = now(), client_recorded_at = $6
			WHERE id = $7
		`, edit.Status, edit.Reason, newRevision, actorID, actorRole, edit.ClientTime, entryID)
		if err != nil {
			return EntryResult{}, err
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO attendance_entry_device_counters (school_id, entry_id, device_id, last_counter)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entry_id, device_id) DO UPDATE SET last_counter = EXCLUDED.last_counter, updated_at = now()
	`, schoolID, entryID, edit.DeviceID, edit.LocalCounter)
	if err != nil {
		return EntryResult{}, err
	}

	return EntryResult{
		EnrollmentID: edit.EnrollmentID, Outcome: OutcomeApplied,
		ServerRevision: newRevision, CurrentStatus: edit.Status, CurrentReason: edit.Reason,
	}, nil
}

func recordRejection(ctx context.Context, tx pgx.Tx, schoolID, entryID uuid.UUID, actorID uuid.UUID, actorRole string, edit EntryEdit, currentStatus Status, currentReason *Reason, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO attendance_sync_rejections
			(school_id, entry_id, attempted_by, attempted_role, attempted_status, attempted_reason, current_status, current_reason, rejection_reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, schoolID, entryID, actorID, actorRole, edit.Status, edit.Reason, currentStatus, currentReason, reason)
	return err
}
