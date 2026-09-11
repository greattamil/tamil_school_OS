-- Phase 3 (PRD 4.4): the real fee engine. Supersedes the Phase 2 read-only
-- fee_dues_snapshot (000013) as the source of truth once a school has real
-- fee_assignments -- that table is left in place for schools/students that
-- don't yet (see fees/repository.go's fallback), not dropped.
--
-- PRD 4.4 opens with "This module carries the highest trust risk in the
-- product. Every requirement here is a correctness requirement" -- the
-- design choices below (integer paise everywhere, immutable payments with
-- compensating entries, a receipt sequence that's never reused even when
-- voided, versioned structures that don't retroactively alter issued
-- demands) all trace directly back to that sentence.

CREATE TYPE fee_statutory_category AS ENUM (
    'tuition', 'special', 'laboratory', 'library', 'computer',
    'development', 'transport', 'examination', 'other'
);

-- PRD 4.4.1: "Fee heads defined per academic year, assignable per class."
CREATE TABLE fee_heads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    statutory_category fee_statutory_category NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (school_id, academic_year_id, name)
);
CREATE INDEX idx_fee_heads_school ON fee_heads(school_id);
CREATE INDEX idx_fee_heads_year ON fee_heads(academic_year_id);

-- PRD 4.4.1: "Fee structures are versioned; changing a structure mid-year
-- does not retroactively alter issued demands." A structure is per class per
-- year; only one version is "active" (the one new assignments are generated
-- from) at a time, but every prior version stays exactly as it was, since
-- fee_assignments below references the specific structure version it was
-- generated from, not "the current structure for this class".
CREATE TABLE fee_structures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
    version INT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID REFERENCES users(id),
    superseded_at TIMESTAMPTZ,
    UNIQUE (school_id, academic_year_id, class_id, version)
);
CREATE INDEX idx_fee_structures_school ON fee_structures(school_id);
CREATE INDEX idx_fee_structures_class_year ON fee_structures(class_id, academic_year_id);
-- Backs "find the one active structure for this class+year" without a
-- second WHERE clause scan; enforced as a real constraint (not just
-- application discipline) since a duplicate active version would make
-- structure resolution at assignment time ambiguous.
CREATE UNIQUE INDEX idx_fee_structures_one_active
    ON fee_structures(school_id, academic_year_id, class_id) WHERE is_active;

-- PRD 4.4.1: "Instalment schedule with due dates per head." One row per
-- (head, instalment) within a structure -- a head with one instalment is the
-- common case (Classes 1-10 style single annual due date), multiple rows the
-- termwise case.
CREATE TABLE fee_structure_instalments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    structure_id UUID NOT NULL REFERENCES fee_structures(id) ON DELETE RESTRICT,
    fee_head_id UUID NOT NULL REFERENCES fee_heads(id) ON DELETE RESTRICT,
    label TEXT NOT NULL, -- 'Term 1', 'Annual', etc.
    amount_paise BIGINT NOT NULL CHECK (amount_paise >= 0),
    due_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_fee_structure_instalments_school ON fee_structure_instalments(school_id);
CREATE INDEX idx_fee_structure_instalments_structure ON fee_structure_instalments(structure_id);

CREATE TYPE concession_type AS ENUM ('sibling', 'staff_ward', 'merit', 'management', 'rte');
CREATE TYPE concession_status AS ENUM ('active', 'review_required', 'cancelled', 'converted');

-- PRD 4.4.1: concessions, with the sibling/staff-ward dependency-tracking
-- design spelled out at length in the PRD because both "continue silently"
-- and "cancel automatically" are explicitly wrong. elder_student_id is a
-- student_id, never an enrollment_id -- enrollments close every June at
-- promotion, and keying the dependency to one would flag every sibling
-- concession in the school as review_required at the single busiest moment
-- of the office's year.
CREATE TABLE concessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    concession_type concession_type NOT NULL,
    -- NULL fee_head_id means "applies to every head in the student's
    -- assignment" (the common case for a sibling/staff-ward percentage off
    -- tuition specifically would instead set fee_head_id).
    fee_head_id UUID REFERENCES fee_heads(id) ON DELETE RESTRICT,
    -- Exactly one of these is set (enforced below): a concession is either a
    -- flat percentage off or a fixed amount off, never both at once.
    percentage NUMERIC(5,2) CHECK (percentage IS NULL OR (percentage > 0 AND percentage <= 100)),
    flat_amount_paise BIGINT CHECK (flat_amount_paise IS NULL OR flat_amount_paise >= 0),
    -- Sibling/staff-ward dependency (PRD 4.4.1). NULL for merit/management/rte.
    elder_student_id UUID REFERENCES students(id) ON DELETE RESTRICT,
    dependent_staff_id UUID REFERENCES staff(id) ON DELETE RESTRICT,
    status concession_status NOT NULL DEFAULT 'active',
    approver_id UUID NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    resolution_note TEXT,
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT concession_exactly_one_amount_kind CHECK (
        (percentage IS NOT NULL AND flat_amount_paise IS NULL) OR
        (percentage IS NULL AND flat_amount_paise IS NOT NULL)
    )
);
CREATE INDEX idx_concessions_school ON concessions(school_id);
CREATE INDEX idx_concessions_student ON concessions(student_id);
CREATE INDEX idx_concessions_elder ON concessions(elder_student_id) WHERE elder_student_id IS NOT NULL;
CREATE INDEX idx_concessions_staff ON concessions(dependent_staff_id) WHERE dependent_staff_id IS NOT NULL;

-- PRD 4.4.2: "On enrollment, applicable heads are assigned to the student,
-- producing dated line items." One assignment per student per year, pinned
-- to the specific structure version it was generated from (never a live
-- FK to "the current structure"), so a later structure edit can never
-- retroactively alter an issued demand.
CREATE TABLE fee_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    structure_id UUID REFERENCES fee_structures(id) ON DELETE RESTRICT,
    -- PRD 4.4: "RTE students tracked as a distinct category with
    -- reimbursement status" -- a property of the year's assignment, since an
    -- RTE quota seat is granted per admission/year, not permanently.
    is_rte BOOLEAN NOT NULL DEFAULT false,
    rte_reimbursement_status TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID REFERENCES users(id),
    UNIQUE (school_id, student_id, academic_year_id)
);
CREATE INDEX idx_fee_assignments_school ON fee_assignments(school_id);
CREATE INDEX idx_fee_assignments_student ON fee_assignments(student_id);

CREATE TYPE fee_line_item_status AS ENUM ('pending', 'partially_paid', 'paid', 'waived');

-- The actual dated, per-head, per-instalment demand a student owes. Generated
-- from fee_structure_instalments at assignment time (net_amount_paise already
-- has any concession applied), then never regenerated from the structure --
-- only individual overrides (PRD 4.4.2: "Individual overrides permitted with
-- reason and approver") touch an existing line item. paid_amount_paise is a
-- denormalized running total kept in step with payment_allocations inside
-- the same transaction (same pattern as attendance_entries.server_revision):
-- real-time dues reporting needs this to be a column, not a live SUM() over
-- every payment ever made, at the scale this reports at.
CREATE TABLE fee_line_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    assignment_id UUID NOT NULL REFERENCES fee_assignments(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    fee_head_id UUID NOT NULL REFERENCES fee_heads(id) ON DELETE RESTRICT,
    label TEXT NOT NULL,
    due_date DATE NOT NULL,
    gross_amount_paise BIGINT NOT NULL CHECK (gross_amount_paise >= 0),
    concession_amount_paise BIGINT NOT NULL DEFAULT 0 CHECK (concession_amount_paise >= 0),
    net_amount_paise BIGINT NOT NULL CHECK (net_amount_paise >= 0),
    paid_amount_paise BIGINT NOT NULL DEFAULT 0 CHECK (paid_amount_paise >= 0),
    status fee_line_item_status NOT NULL DEFAULT 'pending',
    override_reason TEXT,
    override_approver_id UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_fee_line_items_school ON fee_line_items(school_id);
CREATE INDEX idx_fee_line_items_student ON fee_line_items(student_id);
CREATE INDEX idx_fee_line_items_assignment ON fee_line_items(assignment_id);
CREATE INDEX idx_fee_line_items_due_date ON fee_line_items(due_date) WHERE status <> 'paid';

CREATE TYPE payment_mode AS ENUM ('cash', 'cheque', 'upi', 'card');
CREATE TYPE cheque_status AS ENUM ('received', 'deposited', 'cleared', 'bounced');

-- PRD 4.4.6: "Every fee transaction is immutable once written. Corrections
-- are new compensating entries." A payment row is never UPDATEd for its
-- money fields after creation -- cheque_status transitions and is_void are
-- the two narrow, explicitly-designed exceptions (a status field changing is
-- not the same as the transaction itself being edited), and even a void
-- keeps the original row and its receipt_number intact rather than deleting
-- or renumbering it (PRD 4.4.4: "Voided receipts remain in the sequence;
-- the number is never reused").
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    receipt_number BIGINT NOT NULL,
    mode payment_mode NOT NULL,
    -- Total amount actually tendered by the parent.
    amount_paise BIGINT NOT NULL CHECK (amount_paise > 0),
    -- How much of amount_paise was applied to dues vs held as advance (PRD
    -- 4.4.3.3) -- allocated_amount_paise + advance_amount_paise = amount_paise,
    -- enforced in the repository (the allocation rows are the real detail;
    -- these two columns exist so the receipt and reports don't need to
    -- re-derive the split from payment_allocations every time).
    allocated_amount_paise BIGINT NOT NULL DEFAULT 0 CHECK (allocated_amount_paise >= 0),
    advance_amount_paise BIGINT NOT NULL DEFAULT 0 CHECK (advance_amount_paise >= 0),
    collected_by UUID NOT NULL REFERENCES users(id),
    collected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    device_id TEXT,
    cheque_number TEXT,
    cheque_bank TEXT,
    cheque_status cheque_status,
    upi_txn_ref TEXT,
    gateway_provider TEXT,
    is_void BOOLEAN NOT NULL DEFAULT false,
    void_reason TEXT,
    voided_by UUID REFERENCES users(id),
    voided_at TIMESTAMPTZ,
    -- Set once the business day's cash drawer is closed (PRD 4.4.6.1:
    -- "Closing freezes that day's cash batch. No further cash entry, no
    -- voiding, and no back-dating into a closed day"). NULL for non-cash
    -- modes, which the drawer-closing flow doesn't govern.
    cash_drawer_closing_id UUID,
    CONSTRAINT payments_amount_split CHECK (allocated_amount_paise + advance_amount_paise <= amount_paise),
    UNIQUE (school_id, receipt_number)
);
CREATE INDEX idx_payments_school ON payments(school_id);
CREATE INDEX idx_payments_student ON payments(student_id);
CREATE INDEX idx_payments_collected_at ON payments(collected_at);
CREATE INDEX idx_payments_drawer_closing ON payments(cash_drawer_closing_id) WHERE cash_drawer_closing_id IS NOT NULL;

-- Backs the per-school receipt sequence (PRD 4.4.3.3: "Immediate printable
-- receipt") -- a single counter row per school, incremented with
-- SELECT ... FOR UPDATE so two concurrent collectors never get the same
-- number, mirroring the FOR UPDATE SKIP LOCKED discipline already used for
-- job claiming elsewhere in this codebase (this one wants ordinary FOR
-- UPDATE, not SKIP LOCKED -- a receipt number must never be skipped, only
-- ever the next writer waits their turn).
CREATE TABLE fee_receipt_sequences (
    school_id UUID PRIMARY KEY REFERENCES schools(id) ON DELETE RESTRICT,
    next_number BIGINT NOT NULL DEFAULT 1
);

-- PRD 4.4.3.2: "Payment allocation against specific line items, with partial
-- payment support." One payment can spread across several line items; one
-- line item can receive partial payments from several receipts over time.
CREATE TABLE payment_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    line_item_id UUID NOT NULL REFERENCES fee_line_items(id) ON DELETE RESTRICT,
    amount_paise BIGINT NOT NULL CHECK (amount_paise > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payment_allocations_school ON payment_allocations(school_id);
CREATE INDEX idx_payment_allocations_payment ON payment_allocations(payment_id);
CREATE INDEX idx_payment_allocations_line_item ON payment_allocations(line_item_id);

CREATE TYPE credit_transaction_reason AS ENUM ('overpayment', 'applied_to_demand', 'refund', 'manual_adjustment');

-- PRD 4.4.3.3: "A student_credit_balances ledger holds unallocated amounts
-- as a first-class balance." balance_paise is the current denormalized
-- total (same running-total pattern as fee_line_items.paid_amount_paise);
-- credit_balance_transactions below is the append-only ledger it's derived
-- from, so "who put this ₹1,400 here and when" is always answerable.
CREATE TABLE student_credit_balances (
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    balance_paise BIGINT NOT NULL DEFAULT 0 CHECK (balance_paise >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (school_id, student_id)
);

CREATE TABLE credit_balance_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    delta_paise BIGINT NOT NULL, -- positive = credited, negative = applied/refunded
    reason credit_transaction_reason NOT NULL,
    payment_id UUID REFERENCES payments(id) ON DELETE RESTRICT,
    line_item_id UUID REFERENCES fee_line_items(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID REFERENCES users(id)
);
CREATE INDEX idx_credit_balance_transactions_school ON credit_balance_transactions(school_id);
CREATE INDEX idx_credit_balance_transactions_student ON credit_balance_transactions(student_id);

-- PRD 4.4.4: "Refund creates a negative allocation, never deletes the
-- original payment." Modelled as its own table rather than literally a
-- negative-amount payment_allocations row -- a dedicated table keeps
-- "how much of this payment was ever refunded" a direct query instead of a
-- sign-convention reading of the allocations table, while still satisfying
-- the requirement's actual point: the original payment row is untouched.
CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    payment_id UUID REFERENCES payments(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    -- A refund can come out of a specific overpayment (payment_id set) or out
    -- of an accumulated credit balance with no single originating payment
    -- (payment_id NULL) -- PRD 4.4.3.3: "Credit is refundable, subject to the
    -- same authority and audit rules as any refund."
    amount_paise BIGINT NOT NULL CHECK (amount_paise > 0),
    reason TEXT NOT NULL,
    -- PRD 4.4.4: "Void requires correspondent-level authority" -- refunds
    -- carry the same bar, enforced in the repository against approver_id's
    -- role at write time, not by a DB-level role check.
    approved_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID NOT NULL REFERENCES users(id)
);
CREATE INDEX idx_refunds_school ON refunds(school_id);
CREATE INDEX idx_refunds_student ON refunds(student_id);

-- PRD 4.4.3.2: "All inbound payment notifications land in a webhook_events
-- table before any business logic runs." RLS-protected like every other
-- tenant table here -- unlike the genuinely cross-school ScanAbsenceNotifications
-- scan (fixed elsewhere this phase after RLS silently made it a no-op), a
-- payment gateway is registered per school (PRD 4.4.3: "through a gateway
-- registered in the school's name"), so its webhook URL carries the school_id
-- path segment and real tenant context is set (tenancy.WithSchoolID) before
-- this row is ever inserted -- there's no cross-school ambiguity to justify
-- the jobs-table exception here.
CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    error TEXT,
    -- Set once a matching demand/reference is found; NULL means it's sitting
    -- in the exceptions queue (PRD 4.4.3.2: "Unmatched payments ... go to an
    -- exceptions queue for the office rather than being silently dropped or
    -- auto-allocated").
    matched_payment_id UUID REFERENCES payments(id) ON DELETE RESTRICT,
    -- Idempotency is a database constraint (PRD 4.4.3.2: "a duplicate must be
    -- impossible to process twice, enforced by the database"), scoped per
    -- provider since idempotency-key formats are the provider's own scheme,
    -- not ours -- and per school, since RLS partitions everything else here
    -- by school too and two different schools' gateways could coincidentally
    -- reuse the same key format.
    UNIQUE (school_id, provider, idempotency_key)
);
CREATE INDEX idx_webhook_events_school ON webhook_events(school_id);
CREATE INDEX idx_webhook_events_unprocessed ON webhook_events(school_id, received_at) WHERE processed_at IS NULL;

CREATE TYPE cash_drawer_status AS ENUM ('open', 'closed');

-- PRD 4.4.6.1: day-end cash drawer closing. One row per collector per
-- business day; recount attempts (up to two, PRD's own number) are tracked
-- in cash_drawer_recount_attempts below so "each attempt is retained in the
-- record" even though only the final one's totals live here.
CREATE TABLE cash_drawer_closings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    collector_id UUID NOT NULL REFERENCES users(id),
    business_date DATE NOT NULL,
    status cash_drawer_status NOT NULL DEFAULT 'open',
    -- Denomination breakdown as counts, not amounts -- PRD's own list
    -- (500/200/100/50/20/10 plus coins).
    denomination_breakdown JSONB,
    counted_total_paise BIGINT,
    expected_total_paise BIGINT,
    variance_paise BIGINT,
    recount_number INT NOT NULL DEFAULT 0,
    variance_explanation TEXT,
    closed_at TIMESTAMPTZ,
    closed_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, collector_id, business_date)
);
CREATE INDEX idx_cash_drawer_closings_school ON cash_drawer_closings(school_id);
CREATE INDEX idx_cash_drawer_closings_open ON cash_drawer_closings(school_id, business_date) WHERE status = 'open';

CREATE TABLE cash_drawer_recount_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    closing_id UUID NOT NULL REFERENCES cash_drawer_closings(id) ON DELETE RESTRICT,
    attempt_number INT NOT NULL,
    denomination_breakdown JSONB NOT NULL,
    counted_total_paise BIGINT NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_by UUID NOT NULL REFERENCES users(id)
);
CREATE INDEX idx_cash_drawer_recount_attempts_closing ON cash_drawer_recount_attempts(closing_id);

-- Per-school fee configuration not tied to a single structure (PRD 4.4.3.3:
-- cheque return charge default; PRD 4.4.5: reminder scheduling/cap).
CREATE TABLE fee_settings (
    school_id UUID PRIMARY KEY REFERENCES schools(id) ON DELETE RESTRICT,
    cheque_return_charge_paise BIGINT NOT NULL DEFAULT 25000, -- Rs 250
    cash_variance_tolerance_paise BIGINT NOT NULL DEFAULT 500, -- Rs 5
    reminder_max_per_guardian_per_week INT NOT NULL DEFAULT 2,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE fee_heads ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_structures ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_structure_instalments ENABLE ROW LEVEL SECURITY;
ALTER TABLE concessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_line_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_receipt_sequences ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_allocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE student_credit_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE credit_balance_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE cash_drawer_closings ENABLE ROW LEVEL SECURITY;
ALTER TABLE cash_drawer_recount_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_events ENABLE ROW LEVEL SECURITY;

DO $$
DECLARE
  t text;
  tenant_tables CONSTANT text[] := ARRAY[
    'fee_heads', 'fee_structures', 'fee_structure_instalments', 'concessions',
    'fee_assignments', 'fee_line_items', 'payments', 'fee_receipt_sequences',
    'payment_allocations', 'student_credit_balances', 'credit_balance_transactions',
    'refunds', 'cash_drawer_closings', 'cash_drawer_recount_attempts', 'fee_settings',
    'webhook_events'
  ];
BEGIN
  FOREACH t IN ARRAY tenant_tables LOOP
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I USING (school_id = current_school_id()) WITH CHECK (school_id = current_school_id())',
      t
    );
  END LOOP;
END
$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
