-- Academic year is the spine of the system (PRD 3.1). Every tenant table from here
-- on carries school_id, enforced by RLS added in 000009.

CREATE TYPE academic_year_state AS ENUM ('active', 'soft_closing', 'locked');

CREATE TABLE academic_years (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    label TEXT NOT NULL, -- e.g. '2026-27'
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    state academic_year_state NOT NULL DEFAULT 'active',
    soft_closing_buffer_days INT NOT NULL DEFAULT 30,
    state_changed_at TIMESTAMPTZ,
    state_changed_by UUID REFERENCES users(id),
    state_change_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    UNIQUE (school_id, label)
);
CREATE INDEX idx_academic_years_school ON academic_years(school_id);

CREATE TABLE classes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    name TEXT NOT NULL, -- 'Class 6'
    sequence INT NOT NULL, -- 1..12, for display ordering
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (school_id, academic_year_id, name)
);
CREATE INDEX idx_classes_school ON classes(school_id);
CREATE INDEX idx_classes_academic_year ON classes(academic_year_id);

CREATE TABLE sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
    name TEXT NOT NULL, -- 'A'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (class_id, name)
);
CREATE INDEX idx_sections_school ON sections(school_id);
CREATE INDEX idx_sections_class ON sections(class_id);
