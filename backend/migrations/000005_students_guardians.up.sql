-- Guardian is a first-class entity (PRD 3.1), tenant-scoped, with a nullable link to
-- the global users table (set once the guardian has logged in at least once).
CREATE TABLE guardians (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    mobile TEXT NOT NULL,
    email TEXT,
    occupation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_guardians_school ON guardians(school_id);
CREATE INDEX idx_guardians_mobile ON guardians(school_id, mobile);

CREATE TABLE students (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    admission_number TEXT NOT NULL,
    name_english TEXT NOT NULL,
    name_tamil TEXT,
    -- Held separately from display name; the single most useful field for the APAAR
    -- mismatch report (PRD 4.1.1.1).
    name_as_per_aadhaar TEXT,
    date_of_birth DATE NOT NULL,
    gender TEXT NOT NULL,
    mother_tongue TEXT,
    blood_group TEXT,
    address TEXT,
    previous_school TEXT,
    rte_quota BOOLEAN NOT NULL DEFAULT false,
    photo_object_key TEXT,
    -- National identity fields (PRD 4.1.1.1). Not generated here, only stored/reconciled.
    pen_number TEXT,
    apaar_id TEXT,
    apaar_status TEXT NOT NULL DEFAULT 'not_started',
    emis_number TEXT,
    admission_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (school_id, admission_number)
);
CREATE INDEX idx_students_school ON students(school_id);

CREATE TYPE guardian_relationship AS ENUM ('father', 'mother', 'guardian', 'other');

CREATE TABLE student_guardians (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    guardian_id UUID NOT NULL REFERENCES guardians(id) ON DELETE RESTRICT,
    relationship guardian_relationship NOT NULL,
    is_primary_contact BOOLEAN NOT NULL DEFAULT false,
    is_fee_responsible BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (student_id, guardian_id)
);
CREATE INDEX idx_student_guardians_school ON student_guardians(school_id);
CREATE INDEX idx_student_guardians_student ON student_guardians(student_id);
CREATE INDEX idx_student_guardians_guardian ON student_guardians(guardian_id);

CREATE TYPE enrollment_status AS ENUM ('active', 'transferred_out', 'dropped_out', 'graduated');

-- Attendance and marks link to enrollment_id, never student_id + section_id (PRD 3.3).
-- period is a daterange with a GiST exclusion constraint: structurally impossible for
-- a student to hold two overlapping enrollments at this school.
CREATE TABLE enrollments (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
    section_id UUID NOT NULL REFERENCES sections(id) ON DELETE RESTRICT,
    roll_number TEXT,
    medium TEXT NOT NULL DEFAULT 'tamil',
    -- Single default value for Classes 1-10; from Class 11 this selects the subject
    -- group (PRD 3.1). Lives on the enrollment, not the student or section.
    group_code TEXT NOT NULL DEFAULT 'default',
    period daterange NOT NULL,
    status enrollment_status NOT NULL DEFAULT 'active',
    leaving_date DATE,
    leaving_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (id)
);
CREATE INDEX idx_enrollments_school ON enrollments(school_id);
CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_section ON enrollments(section_id);
CREATE INDEX idx_enrollments_academic_year ON enrollments(academic_year_id);

ALTER TABLE enrollments ADD CONSTRAINT no_overlapping_enrollment
  EXCLUDE USING gist (school_id WITH =, student_id WITH =, period WITH &&)
  WHERE (deleted_at IS NULL);
