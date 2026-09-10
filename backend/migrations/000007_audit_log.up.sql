-- Append-only audit log (PRD 6.5). Logs changes, not writes, and stores deltas, not
-- snapshots -- callers pass only the keys that changed. Partitioning by academic year
-- (PRD 3.3) is deferred past Phase 1; this table will need to move to a partitioned
-- layout before volume makes it necessary, tracked as follow-up work.
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    actor_user_id UUID REFERENCES users(id),
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    before_values JSONB,
    after_values JSONB,
    reason TEXT,
    ip_address INET,
    device_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_log_school ON audit_log(school_id);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
