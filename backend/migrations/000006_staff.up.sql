CREATE TABLE staff (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    designation TEXT,
    qualification TEXT,
    date_of_joining DATE,
    emis_staff_id TEXT,
    is_teaching BOOLEAN NOT NULL DEFAULT true,
    left_at DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_staff_school ON staff(school_id);

CREATE TABLE section_teachers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    section_id UUID NOT NULL REFERENCES sections(id) ON DELETE RESTRICT,
    staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE RESTRICT,
    is_class_teacher BOOLEAN NOT NULL DEFAULT false,
    subject TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_section_teachers_school ON section_teachers(school_id);
CREATE INDEX idx_section_teachers_section ON section_teachers(section_id);
CREATE INDEX idx_section_teachers_staff ON section_teachers(staff_id);
