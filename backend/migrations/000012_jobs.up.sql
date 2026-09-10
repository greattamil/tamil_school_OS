-- Background jobs live in a PostgreSQL table, not Redis (PRD 8.3): a container
-- restart or OOM kill against an in-memory queue silently discards queued work,
-- which for this system means unsent absence alerts and unprocessed payment
-- webhooks with nothing to indicate anything was lost. This table survives
-- everything the database does.
--
-- Deliberately NOT row-level-secured. Every other tenant table's RLS policy
-- assumes a request running inside one school's SET LOCAL app.current_school_id
-- context (PRD 6.1); the worker process that claims jobs has no such context --
-- it claims and processes jobs for every school from one long-running loop. RLS
-- would either block it entirely or require a bypass role, and PRD 6.1 point 6 is
-- explicit that application roles never get BYPASSRLS. This table is therefore
-- operational infrastructure with no direct query surface exposed to any school's
-- users (no "view my jobs" endpoint), not tenant data being isolated -- the same
-- category as the job queue's own existence, not the same category as
-- school_settings or notifications, which ARE tenant data and do carry RLS.
-- school_id lives in the payload for the handler's own use, not as an isolation
-- boundary.
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, done, failed
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error TEXT,
    run_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at TIMESTAMPTZ,
    claimed_by TEXT,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Backs the FOR UPDATE SKIP LOCKED claim query: pending jobs whose time has come.
CREATE INDEX idx_jobs_claimable ON jobs(run_after) WHERE status = 'pending';
