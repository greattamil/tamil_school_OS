-- Per-school notification configuration (PRD 4.5.4: quiet hours, daily message
-- cap; PRD 4.2.3: school-configured absence-notification time, default 11:00).
-- Tenant-scoped and RLS-protected, unlike the `schools` identity row itself.
CREATE TABLE school_settings (
    school_id UUID PRIMARY KEY REFERENCES schools(id) ON DELETE RESTRICT,
    absence_notification_time TIME NOT NULL DEFAULT '11:00',
    quiet_hours_start TIME NOT NULL DEFAULT '21:00',
    quiet_hours_end TIME NOT NULL DEFAULT '07:00',
    max_daily_automated_messages INT NOT NULL DEFAULT 3,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE notification_kind AS ENUM (
    'notice', 'absence_alert', 'fee_due_reminder', 'fee_overdue_reminder',
    'payment_receipt', 'report_card_published', 'exam_schedule_published',
    'emergency_broadcast'
);

-- A composed notice (PRD 4.5.1) or an automated notification (PRD 4.5.3). One row
-- per message; fan-out to individual recipients is notification_recipients below,
-- which is what per-recipient delivery/read tracking hangs off.
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    kind notification_kind NOT NULL,
    title TEXT NOT NULL,
    body_en TEXT NOT NULL,
    body_ta TEXT,
    created_by UUID REFERENCES users(id),
    -- Human-readable description of who this targeted ("Class 6 Section A",
    -- "Whole school"), for display and audit -- not used to resolve recipients,
    -- which are materialized into notification_recipients at send time.
    target_description TEXT,
    attachment_url TEXT,
    scheduled_at TIMESTAMPTZ,
    dispatched_at TIMESTAMPTZ,
    -- Emergency broadcasts bypass quiet hours and frequency caps (PRD 4.5.3) --
    -- the only message class permitted to.
    is_emergency BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_school ON notifications(school_id);

CREATE TABLE notification_recipients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id),
    -- Denormalized so a rendered notice remains meaningful even if the student
    -- record is later soft-deleted or the guardian relationship changes.
    student_id UUID REFERENCES students(id),
    push_sent_at TIMESTAMPTZ,
    push_acknowledged_at TIMESTAMPTZ,
    sms_sent_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (notification_id, user_id)
);
CREATE INDEX idx_notification_recipients_school ON notification_recipients(school_id);
CREATE INDEX idx_notification_recipients_notification ON notification_recipients(notification_id);
CREATE INDEX idx_notification_recipients_user ON notification_recipients(user_id);

-- FCM registration tokens. Tenant-scoped rather than global: a token is registered
-- by the app while holding a school-scoped session, and every send operation
-- already runs inside a school's tenant context (PRD 6.1) -- keeping it here
-- avoids growing the deliberately small global-identity table set (PRD 3.2.1) for
-- what is fundamentally a per-session delivery detail, at the cost of a token
-- being re-registered per school after a role switch.
CREATE TABLE device_push_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id),
    fcm_token TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT 'android',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (school_id, user_id, fcm_token)
);
CREATE INDEX idx_device_push_tokens_school ON device_push_tokens(school_id);
CREATE INDEX idx_device_push_tokens_user ON device_push_tokens(user_id);
