-- Phase 2 scope only (PRD section 9): a read-only dues window fed by a simple
-- import of the school's existing fee position, so parents have a reason to open
-- the app before the full fee engine (collection, allocation, receipts) arrives
-- in Phase 3. One row per student, replaced wholesale on each re-import -- this is
-- a snapshot, not a ledger.
CREATE TABLE fee_dues_snapshot (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    -- Money as integer paise, never floating point (PRD 3.3), even in this
    -- simplified snapshot -- the rule has no read-only exception.
    outstanding_amount_paise BIGINT NOT NULL,
    due_date DATE,
    last_payment_date DATE,
    last_payment_amount_paise BIGINT,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    imported_by UUID REFERENCES users(id),
    UNIQUE (school_id, student_id)
);
CREATE INDEX idx_fee_dues_snapshot_school ON fee_dues_snapshot(school_id);
