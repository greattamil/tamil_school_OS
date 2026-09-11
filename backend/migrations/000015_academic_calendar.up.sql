-- PRD 3.1: "An academic_calendar_days entity holds one row per school per date
-- with a day_type ... All working-day totals, attendance percentages and
-- register exports compute from these rows, never from the calendar week."
--
-- One row per (school, date) rather than per (school, academic_year, date) --
-- a date belongs to exactly one calendar day regardless of which academic year
-- happens to be active, and a school only ever has one year active at a time
-- (PRD 3.1's three-state lifecycle), so the extra key would add nothing but a
-- second place year-boundary dates could disagree with the years table itself.
CREATE TYPE calendar_day_type AS ENUM (
    'regular_working', 'holiday', 'compensatory_working', 'half_day', 'exam_day'
);

CREATE TABLE academic_calendar_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    date DATE NOT NULL,
    day_type calendar_day_type NOT NULL,
    note TEXT,
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, date)
);
CREATE INDEX idx_academic_calendar_days_school_date ON academic_calendar_days(school_id, date);

ALTER TABLE academic_calendar_days ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON academic_calendar_days
    USING (school_id = current_school_id()) WITH CHECK (school_id = current_school_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON academic_calendar_days TO app_user;
