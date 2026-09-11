-- PRD 4.4.3.1: "If no notification channel exists, the honest fallback is a
-- link plus a parent-submitted UTR that the office verifies against the bank
-- statement -- cheaper than a gateway but not automatic, and it must be
-- presented to the school as such." No school's actual bank capability is
-- confirmed in this codebase, so this fallback -- not the automatic-
-- reconciliation path -- is what's buildable here.
CREATE TYPE upi_request_status AS ENUM ('pending', 'utr_submitted', 'verified', 'rejected');

-- One row per generated UPI intent link. reference is the `tr` value in the
-- link (PRD 4.4.3.1: "The tr value is generated per demand, stored against
-- the fee line items it covers"); allocations are declared up front (what
-- the office intends this payment to cover) so verification only has to
-- confirm the money arrived, not re-decide the split.
CREATE TABLE upi_payment_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    reference TEXT NOT NULL,
    amount_paise BIGINT NOT NULL CHECK (amount_paise > 0),
    intent_link TEXT NOT NULL,
    status upi_request_status NOT NULL DEFAULT 'pending',
    submitted_utr TEXT,
    submitted_at TIMESTAMPTZ,
    verified_payment_id UUID REFERENCES payments(id) ON DELETE RESTRICT,
    verified_by UUID REFERENCES users(id),
    verified_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, reference)
);
CREATE INDEX idx_upi_payment_requests_school ON upi_payment_requests(school_id);
CREATE INDEX idx_upi_payment_requests_student ON upi_payment_requests(student_id);
CREATE INDEX idx_upi_payment_requests_pending ON upi_payment_requests(school_id, status) WHERE status IN ('pending', 'utr_submitted');

-- Declared allocations for a UPI request -- same shape as payment_allocations,
-- kept as a separate pre-declaration so verification just replays them into
-- a real CollectPayment call rather than re-deciding the split at the moment
-- money is confirmed.
CREATE TABLE upi_payment_request_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    request_id UUID NOT NULL REFERENCES upi_payment_requests(id) ON DELETE RESTRICT,
    line_item_id UUID NOT NULL REFERENCES fee_line_items(id) ON DELETE RESTRICT,
    amount_paise BIGINT NOT NULL CHECK (amount_paise > 0)
);
CREATE INDEX idx_upi_payment_request_allocations_school ON upi_payment_request_allocations(school_id);
CREATE INDEX idx_upi_payment_request_allocations_request ON upi_payment_request_allocations(request_id);

ALTER TABLE upi_payment_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE upi_payment_request_allocations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON upi_payment_requests
    USING (school_id = current_school_id()) WITH CHECK (school_id = current_school_id());
CREATE POLICY tenant_isolation ON upi_payment_request_allocations
    USING (school_id = current_school_id()) WITH CHECK (school_id = current_school_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON upi_payment_requests TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON upi_payment_request_allocations TO app_user;

-- The school's own UPI VPA and display name for the intent link (PRD
-- 4.4.3.1's `pa`/`pn` parameters) -- not known for any school in this
-- codebase yet, so NULL by default; request generation refuses to proceed
-- without it rather than emitting a link to nowhere.
ALTER TABLE fee_settings ADD COLUMN upi_vpa TEXT;
ALTER TABLE fee_settings ADD COLUMN upi_payee_name TEXT;
