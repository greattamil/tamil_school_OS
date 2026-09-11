ALTER TABLE fee_line_items DROP COLUMN IF EXISTS overdue_reminder_sent_at;
ALTER TABLE fee_line_items DROP COLUMN IF EXISTS due_reminder_sent_at;
