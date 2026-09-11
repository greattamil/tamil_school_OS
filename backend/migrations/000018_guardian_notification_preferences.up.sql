-- PRD 4.5.2: "Per-guardian channel preference and per-category opt-out (fee
-- reminders, attendance, general notices), which is also a DPDP consent
-- requirement." Never actually built until now -- notify.Compose has grown
-- real message discipline (quiet hours, the daily cap) without this, and
-- those two are about *volume*, not consent; a guardian who has explicitly
-- opted out needs to never receive that category regardless of how well
-- within the cap it is.
--
-- Columns on guardians directly, not a separate preferences table: the PRD's
-- own granularity is "per-guardian", guardians is already the natural
-- tenant-scoped row for one, and there is exactly one active preference set
-- per guardian (no history requirement stated) -- a join table would add
-- nothing here.
ALTER TABLE guardians ADD COLUMN opt_out_fee_reminders BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE guardians ADD COLUMN opt_out_attendance_alerts BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE guardians ADD COLUMN opt_out_general_notices BOOLEAN NOT NULL DEFAULT false;
-- Channel preference: SMS fallback is a real cost the school bears (PRD
-- 4.5.2: "sent through the school's own DLT-registered gateway so the cost
-- sits with the school"), so a guardian may reasonably want push-only even
-- while wanting the underlying category. Emergency broadcasts (PRD 4.5.3)
-- are explicitly exempt from all discipline including this one -- a cyclone
-- holiday alert is not something "SMS opt-out" should be able to silence.
ALTER TABLE guardians ADD COLUMN sms_opt_out BOOLEAN NOT NULL DEFAULT false;
