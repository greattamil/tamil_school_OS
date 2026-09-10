# Build progress

Tracking against the PRD's phased release plan (section 9). Update this file at the
end of each work session rather than relying on git log to reconstruct status.

## Phase 1 — Foundation (weeks 1-4 target) — COMPLETE

Multi-tenant schema, RLS on every tenant table (catalog-driven, not a maintained
list), global identity split, auth (staff password + parent OTP with the
find-or-provision flow described below), students/staff/guardians/academic-years/
enrollments CRUD, bulk import with the PRD 4.1.3 guardian auto-linking safety
heuristics, Next.js admin panel scaffold, CORS, CI running the cross-tenant
integration suite on every push. See git history for the two Phase 1 commits;
this file now tracks Phase 2 onward in detail.

## Phase 2 — Daily use (weeks 5-9 target)

### Backend: done and verified in Docker

- **Attendance module** (`internal/attendance`): registers + entries, and the
  full per-entry conflict resolution algorithm from PRD 4.2.5 -- kept as pure,
  unit-tested functions in `conflict.go`/`conflict_test.go` separate from the
  database orchestration in `repository.go`. Verified against the PRD's own
  narrative scenario (class teacher marks offline at 08:45, office admin
  corrects at 09:00, teacher's phone syncs its stale edit at 09:15) **through
  the live HTTP API** with a real teacher account and a real correspondent
  account, not just at the unit-test level: office correction wins even when
  it declares a stale base revision (role-authority override), a same-rank
  stale retry is rejected, and a same-device replayed local_counter is
  rejected as invalid with no data change. Past-day correction is blocked for
  the teacher role and allowed for office admin/correspondent (PRD 4.2.2).
  Every rejection is written to `attendance_sync_rejections` with before/after
  values. Reporting: daily section/school summary, per-student percentage
  (aggregated across enrollments, so a mid-year transfer doesn't fragment the
  figure), and a consecutive-absence report via a gaps-and-islands SQL query.
- **Jobs** (`internal/jobs`): the PostgreSQL-backed durable queue from PRD 8.3.
  Claim-and-mark-processing is one atomic `UPDATE ... FROM (SELECT ... FOR
  UPDATE SKIP LOCKED)` statement, not two separate ones -- two statements would
  let the row lock release between them and let a second worker claim the same
  job. Transactional enqueue (`EnqueueTx`) is used everywhere a job is a direct
  consequence of another write. Retry with backoff, terminal `failed` status
  past `max_attempts`, visible via a plain SQL query (no dashboard yet).
- **Notify module** (`internal/notify`): notices (PRD 4.5.1) with whole-school/
  class/section/student/custom targeting, resolved to concrete guardian
  recipients at compose time; per-recipient delivery/read tracking; an FCM
  `Dispatcher` interface with only the actual Firebase API call stubbed
  (`LogDispatcher`) -- targeting, batching to 500 tokens (PRD 8.4), delivery
  tracking and stale-token pruning are all real. The absence-notification scan
  (PRD 4.2.3, default 11:00 IST) runs as a periodic check in `cmd/worker`,
  finds schools past their configured time with un-notified absences, and
  fans out one notification per absent student to their primary guardian.
  Verified end-to-end in Docker: composed a notice, watched the worker claim
  and "dispatch" it via the stub, confirmed it landed in the parent's own
  inbox via a real OTP-authenticated session.
- **Fixed a real gap found by testing, not by inspection**: the parent OTP
  login flow from Phase 1 looked complete but had never actually been
  exercised -- `VerifyParentOTP` looked up a `users` row by mobile that nothing
  ever created. Guardians are tenant-scoped (RLS-protected) but OTP
  verification happens before any tenant context exists, so there's no way to
  look up "which school does this mobile belong to" through the normal path.
  Fixed in `auth.Service.findOrProvisionGuardianUser`: on first successful
  OTP verification, check each school's tenant context in turn (fine at PRD's
  target scale of 15 schools; would need a dedicated global mobile index
  before it stops being fine), provision a global user for any match, link
  every matching guardian row, and create the accompanying `user_school_roles`
  (role='parent') the same way staff roles work. Verified end-to-end: OTP
  request -> verify -> token scoped to the right school with role=parent ->
  guardian row now has user_id set -> parent-facing endpoints work.
- **Fee dues read-only view** (`internal/fees`): CSV import (rupees converted
  to integer paise on the way in, per PRD 3.3), per-student and office-wide
  read endpoints. Explicitly not the fee engine (Phase 3 scope).
- **CSV robustness fix, found by testing**: Go's `encoding/csv` aborts the
  *entire* read on the first row with a different field count than the
  header, by default. Both this module and `bulkimport` had that default on --
  meaning one malformed row (a common real-world spreadsheet export quirk)
  would have silently discarded every row after it, not just that row. Fixed
  by setting `FieldsPerRecord = -1` in both parsers; the existing per-field
  required-value checks already handle the resulting missing trailing values
  correctly, now verified with a regression test CSV in each module.
- **Tamil text end-to-end**: verified a Tamil-body notice survives compose ->
  Postgres storage -> API response with correct UTF-8 bytes at every hop
  (checked via raw hex, not just visual inspection -- a terminal encoding
  artifact in this session's own shell briefly looked like data corruption
  and was confirmed to be a test-harness issue, not an application bug).
- `cmd/worker`: the separate background-job process from PRD 8.1, added to
  Docker Compose. Runs the job-claim loop and the absence-scan timer.

### Backend: not yet started

- **Mobile app** (Flutter): this is the actual Phase 2 headline deliverable --
  offline-first attendance marking with local-first writes, an outbox sync
  queue, SQLCipher-encrypted local storage, and the client side of the
  conflict-resolution protocol above. Starting next.
- Class diary is Phase 4 per the PRD, not Phase 2 -- not started, correctly.
- SMS rollover for undelivered push (PRD 4.5.3's emergency-broadcast
  requirement) is not implemented; the dispatch pipeline has the hook points
  (`push_acknowledged_at`, `sms_sent_at` columns exist) but nothing populates
  them yet.
- Quiet hours and the daily-message cap (`school_settings` columns exist) are
  not enforced anywhere yet -- notices send immediately regardless of time of
  day or how many the recipient already received today.
- Class-teacher notice targeting is restricted to "their own section" only in
  the sense of rejecting whole-school/class targets from that role; it does
  not yet verify the teacher is actually assigned to the section they're
  targeting (needs `section_teachers` to be consulted, not just role-checked).
- The academic_calendar_days entity (PRD 3.1) still doesn't exist, so
  attendance-percentage figures remain a known simplification (documented in
  `attendance/reporting.go`) -- not safe to print on a TC yet.

## Phase 3+

Not started. See PRD section 9 (full fee engine, exams/report cards,
certificates, dashboards, class diary).
