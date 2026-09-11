-- PRD 4.4.5: "Automated reminders to fee-responsible guardians, schedulable,
-- with a per-school opt-out and a hard cap on frequency." The opt-out and cap
-- are already enforced generically by the notify module (quiet hours, the
-- daily automated-message cap) -- these two columns are what the fee-specific
-- scan needs to avoid re-sending the same reminder every time its ticker
-- fires: a due-soon reminder is sent once, an overdue reminder repeats on a
-- cooldown rather than a one-time flag (a line item can stay overdue for
-- months, and "notified once, ever" would go silent long before it's paid).
ALTER TABLE fee_line_items ADD COLUMN due_reminder_sent_at TIMESTAMPTZ;
ALTER TABLE fee_line_items ADD COLUMN overdue_reminder_sent_at TIMESTAMPTZ;
