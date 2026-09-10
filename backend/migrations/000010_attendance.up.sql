-- Attendance and marks link to enrollment_id, never student_id + section_id
-- (PRD 3.3), so a mid-year transfer never corrupts historical registers.
--
-- Declarative partitioning by academic year (PRD 3.3) is deferred here, same as
-- audit_log in migration 000007 -- cheap to add now, awkward later, but not
-- required to prove the sync/conflict-resolution design correct. Tracked as
-- follow-up work.

CREATE TABLE attendance_registers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    academic_year_id UUID NOT NULL REFERENCES academic_years(id) ON DELETE RESTRICT,
    section_id UUID NOT NULL REFERENCES sections(id) ON DELETE RESTRICT,
    date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (school_id, section_id, date)
);
CREATE INDEX idx_attendance_registers_school ON attendance_registers(school_id);
CREATE INDEX idx_attendance_registers_section_date ON attendance_registers(section_id, date);

CREATE TYPE attendance_status AS ENUM ('present', 'absent');
CREATE TYPE absence_reason AS ENUM ('sick', 'permitted', 'unexcused');

-- One row per student per day (PRD 4.4.5 / 3.3: sync granularity is the individual
-- student entry, keyed by enrollment_id and date). server_revision is the
-- optimistic-concurrency counter the whole conflict resolution scheme in
-- internal/attendance/sync.go is built on: every accepted write increments it, and
-- a client's sync request declares the revision its edit was based on.
CREATE TABLE attendance_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    register_id UUID NOT NULL REFERENCES attendance_registers(id) ON DELETE RESTRICT,
    enrollment_id UUID NOT NULL REFERENCES enrollments(id) ON DELETE RESTRICT,
    date DATE NOT NULL,
    status attendance_status NOT NULL,
    reason absence_reason,
    server_revision INT NOT NULL DEFAULT 1,
    recorded_by UUID REFERENCES users(id),
    recorded_role TEXT NOT NULL,
    -- The device's own clock at the moment of the edit. Retained for display and
    -- audit only -- conflict resolution never trusts it (PRD 4.2.5 point 2:
    -- Android devices drift and battery-saver modes desync NTP).
    client_recorded_at TIMESTAMPTZ,
    -- Set once the school's configured absence-notification scan has enqueued a
    -- notification for this entry, so the scan never double-sends (PRD 4.2.3:
    -- notification only after sync, at a school-configured time, once).
    absence_notified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (school_id, enrollment_id, date)
);
CREATE INDEX idx_attendance_entries_school ON attendance_entries(school_id);
CREATE INDEX idx_attendance_entries_register ON attendance_entries(register_id);
CREATE INDEX idx_attendance_entries_enrollment ON attendance_entries(enrollment_id);
-- Backs the absence-notification scan: today's un-notified absences per school.
CREATE INDEX idx_attendance_entries_notify_scan ON attendance_entries(school_id, date, status)
  WHERE status = 'absent' AND absence_notified_at IS NULL;

-- The server holds the last local_counter seen per (device, entry) pair (PRD
-- 4.2.5 point 2), so out-of-order or duplicate delivery from the SAME device is
-- rejected without trusting wall-clock time. This is deliberately separate from
-- server_revision, which arbitrates conflicting edits from DIFFERENT writers.
CREATE TABLE attendance_entry_device_counters (
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    entry_id UUID NOT NULL REFERENCES attendance_entries(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL,
    last_counter BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entry_id, device_id)
);
CREATE INDEX idx_attendance_entry_device_counters_school ON attendance_entry_device_counters(school_id);

-- Every rejected sync edit is audit-logged with both values and the reason (PRD
-- 4.2.5 point 7); accepted routine writes are not (PRD 6.5: log changes, not
-- writes). This table holds the superseded/invalid attempts specifically, so a
-- teacher's "why was my edit rejected" question is a query, not log archaeology.
CREATE TABLE attendance_sync_rejections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    entry_id UUID NOT NULL REFERENCES attendance_entries(id) ON DELETE RESTRICT,
    attempted_by UUID REFERENCES users(id),
    attempted_role TEXT NOT NULL,
    attempted_status attendance_status NOT NULL,
    attempted_reason absence_reason,
    current_status attendance_status NOT NULL,
    current_reason absence_reason,
    rejection_reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_attendance_sync_rejections_school ON attendance_sync_rejections(school_id);
CREATE INDEX idx_attendance_sync_rejections_entry ON attendance_sync_rejections(entry_id);
