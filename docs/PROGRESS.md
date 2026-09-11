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

- **Parent children endpoint** (`internal/guardians`, `GET /api/v1/parent/children`):
  added after the mobile parent dashboard exposed a real gap -- the OTP login
  flow provisions a parent's identity and links their guardian rows, but
  nothing let the app ask "which students is this logged-in parent actually
  linked to." Verified end-to-end with a real parent OTP session.

### Mobile app (Flutter) -- the actual Phase 2 headline deliverable

`mobile/`, Android-first per the PRD (Windows desktop also enabled, dev/test
convenience only -- see below).

- **Offline-first attendance, the safety-critical piece**
  (`lib/features/attendance/`): local-first writes (PRD 4.2.1: "Writes to
  local SQLite first and confirms immediately"), an outbox queue, and a sync
  engine implementing the client side of the exact protocol
  `internal/attendance/conflict.go` enforces server-side -- `base_revision`
  captured from local state at write time (never recomputed later),
  `local_counter` persisted in the local DB itself so an app restart mid-
  session can't reissue a value the server already saw, and per-entry
  (not per-batch) reconciliation of the sync response: applied edits update
  the local revision, superseded/invalid edits overwrite local state with
  the server's truth and surface a dialog telling the teacher which students
  were overridden and to what (PRD 4.2.5 point 6). Foreground and
  connectivity-regained are the sync triggers (PRD 4.2.5), not a periodic
  background task. The marking screen defaults every student to present and
  a single "Confirm & sync" action writes the whole register in one batch
  (PRD 4.2.1), matching the actual described flow rather than a tap-per-sync
  design.
- **Local encrypted storage** (`lib/core/db/`): Drift over SQLCipher, one
  database file per school (`app_db_<school_id>.sqlite`, PRD 3.2.1) so
  revoking one role's access is exactly "delete this file, discard this key"
  with another school's roster on the same device untouched. Key generated
  from a secure random source on first launch, stored via
  `flutter_secure_storage` (Android Keystore-backed), never bound to
  biometric enrollment (PRD 6.4: binding it would silently wipe a teacher's
  local data the moment they enroll a fingerprint). `connection.dart`
  verifies `PRAGMA cipher_version` is non-empty before trusting the key
  pragma at all -- SQLCipher's key pragma fails silently against a plain
  SQLite build rather than erroring, which would otherwise mean quietly
  falling back to plaintext storage with no indication anything was wrong.
- **Auth** (`lib/core/auth/`): staff password login and parent OTP login
  against the same endpoints verified on the backend, token storage in
  platform secure storage (never SharedPreferences, PRD 6.4), a school picker
  for multi-school staff, and a `handleSessionRevoked` path that performs the
  exact wipe PRD 6.2 specifies (purge key, delete the school's local DB file,
  clear tokens) -- implemented as the *same* code path as a normal logout,
  since the PRD's point is that revocation should be indistinguishable from a
  deliberate wipe, not a special case bolted onto it.
- **Parent dashboard** (`lib/features/parent/`): child selector for parents
  with more than one child (PRD 4.8), attendance percentage and dues for the
  selected child, and the recipient's own notice inbox.
- **Verified for real, not just compiled**: `flutter analyze` clean, 6 tests
  passing (`flutter test`) -- five exercise the local-only write path
  (`markAttendance`/`confirmRegister`/`watchRoster`) against an in-memory
  Drift database, checking specifically that `base_revision` and
  `local_counter` are captured correctly, since a bug there would silently
  break the server-side conflict resolution no amount of backend testing
  could catch. The app was also launched on a real Android emulator
  (API 36, x86_64) against the live Docker backend, driven end-to-end via
  `adb input` and screenshots (no chromium-cli/device-automation tool was
  available, so this was done directly): login screen rendered correctly,
  staff login succeeded against the real API and routed to the right
  screen, an invalid-password attempt correctly surfaced the server's error
  message, the academic-year/class/section picker loaded and cascaded from
  real API data, and the parent OTP flow's session persisted across an app
  restart (auto-login from secure storage).
- **A real on-device bug this surfaced, and the fix**: opening the section
  roster came back empty every time, with the local database queries
  themselves apparently succeeding (no crash, no thrown error visible
  anywhere) -- suspicious given the same data was confirmed present via a
  direct API call. Root cause: `NativeDatabase.createInBackground` runs all
  database operations on a **separate Dart isolate**, and the
  `open.overrideFor(OperatingSystem.android, openCipherOnAndroid)` call in
  `main()` -- required so `package:sqlite3` opens the SQLCipher build instead
  of the plain SQLite library Android also ships -- only applies to the
  isolate that calls it. The background isolate silently fell back to plain
  SQLite, which fails this codebase's own `PRAGMA cipher_version` guard in
  `connection.dart` (added specifically so a silent fallback to plaintext
  storage can't happen unnoticed, per PRD 6.4) -- so every query on that
  isolate threw, and the attendance screen's `.catchError` swallowed it
  with no visible sign beyond an empty list. Confirmed by temporarily
  surfacing the caught error in the UI rather than guessing from logs, which
  is what actually revealed the `StateError` (this back-and-forth, and the
  Android emulator's storage running out mid-session while trying to close
  the loop with a fresh install, is why this fix initially landed without a
  final post-fix on-device re-run -- it was implemented and reasoned through
  against Drift's own documented isolate-scoping behavior and the
  sqlcipher_flutter_libs README's explicit warning about this exact case.
  **Since confirmed on a real device in a later session**: logged in as a
  real teacher account, opened a section's attendance screen, and the full
  real roster (five real students fetched from the server) rendered
  correctly -- the fix holds). Fixed by passing `isolateSetup:` to
  `NativeDatabase.createInBackground`, which Drift runs once inside the new
  isolate before any query -- re-applying the same override there closes the
  gap. Also turned the screen's silent `.catchError` into a real (if
  understated) error message in the empty-state UI, since this bug would
  have been invisible without one.
- **Dependency wrangling, worth recording**: the current Dart/Flutter
  ecosystem has partially migrated SQLite bundling to a new native-assets
  "hooks" build system (`sqlite3` 3.x), which `build_runner`'s script
  compiler doesn't yet support -- even a transitive, iOS/macOS-only
  dependency (`path_provider_foundation`'s `objective_c` bridge) pulling in a
  hook broke Drift's code generation entirely. Fixed by pinning `drift`/
  `drift_dev` to the 2.31 line (predates the hooks-based `sqlite3` 3.x) and
  overriding `path_provider_foundation` to a pre-hooks release neither
  Android nor Windows needs anyway. Separately, `sqlite3_flutter_libs` and
  `sqlcipher_flutter_libs` must never be dependencies at the same time --
  both bundle a plugin class under the same package name, which fails
  Android's dex-merge step with a duplicate-class error. Since PRD 6.4
  requires the local database always be encrypted, only
  `sqlcipher_flutter_libs` is needed; its SQLCipher build is ABI-compatible
  with plain sqlite3, so nothing is lost by not depending on both.
- **A second real on-device bug, found and fixed in that later session**: the
  attendance confirm-and-sync flow wrote locally (outbox correctly queued 5
  pending entries) but the network sync silently never completed -- the app
  showed "N pending" forever with no error anywhere, because
  `attendance_screen.dart`'s sync-failure handler was a bare `catch (_) {}`.
  Root cause, found by temporarily surfacing the caught error in the UI (same
  technique as the isolate bug above): Drift's SQLite round-trip loses the
  UTC flag on `DateTime` columns -- the value read back is always
  `isUtc: false` even though it was written from `.toUtc()` -- so
  `clientTimestamp.toIso8601String()` in the sync payload omitted the `Z`
  suffix, and the backend's RFC3339 `time.Parse` rejected every sync request
  with a 400. Fixed by forcing `.toUtc()` again immediately before formatting
  in `attendance_repository.dart`. Verified for real: marked a student
  absent on-device, tapped confirm, and watched the server's own
  `server_revision` actually increment for that exact student. Also: two
  release-build-only bugs found and fixed along the way that would have
  blocked any real distribution regardless of this one -- `INTERNET`
  permission was only in Flutter's debug-only manifest (missing from real
  release builds), and no network security config existed to permit the
  local dev backend's cleartext HTTP (blocked by default at this target SDK
  level in every build type, not just release).
- **Not yet done**: period-wise/half-day attendance (PRD 4.2.1 mentions it as
  school-configurable; this pass only does whole-day), and marks entry
  (Phase 4 per the PRD's own module scope, not missing from Phase 2).
  Battery-optimization-exemption prompt and the pending-register-older-than-
  24-hours warning (both PRD 4.2.5) are being closed out in this same
  session -- see below rather than listed as outstanding twice.
- **CI now covers mobile too**: added a `mobile` job to
  `.github/workflows/backend-ci.yml` (`flutter analyze` + `flutter test`,
  the in-memory-database suite -- no emulator needed for that, so it's a
  normal fast CI job, not something blocked by the storage issue above).
  The workflow's display name changed from `backend-ci` to `ci` to match.

### Phase 2 gaps closed in this session -- all verified live in Docker, not just compiled

- **Quiet hours enforced** (PRD 4.5.4): `dispatchNotificationHandler` now checks
  `school_settings.quiet_hours_start/end` for every automated (non-`notice`,
  non-emergency) notification before sending, and defers rather than drops --
  it re-enqueues the same dispatch job at the moment quiet hours end (a new
  `EnqueueTxAt` job-package helper using the `jobs.run_after` column that
  already existed for exactly this). Verified by temporarily narrowing a real
  school's quiet-hours window to include the current time, composing an
  automated notification, and confirming in Postgres that a new job appeared
  with `run_after` set to the window's end and `push_sent_at` stayed null.
- **Daily automated-message cap enforced** (PRD 4.5.4, default 3/guardian/day):
  checked per-recipient at send time against today's already-sent count for
  non-notice, non-emergency notifications; the in-app notification row is
  still created regardless (delivery/read tracking is per-recipient per PRD
  4.5.1 -- the cap governs push/SMS noise, not in-app visibility). Verified by
  sending four automated notices to the same guardian with the cap set to 3:
  the first three set `push_sent_at`, the fourth did not.
- **Class-teacher section-targeting actually verified, not role-proxied**: the
  previous "reject whole-school/class from teacher role" placeholder is
  replaced with a real `section_teachers` lookup (`Repository.TeacherOwnsSection`/
  `TeacherOwnsStudent`, scoped to the active academic year). Verified with a
  real teacher account assigned to one section: composing to their own section
  succeeds, composing to a second section they're not assigned to returns 403
  `not assigned to this section`, and whole-school/class targeting from that
  role is still rejected outright.
- **SMS rollover for emergency broadcasts** (PRD 4.5.3): a new `SMSSender`
  interface (real telecom-API call stubbed as `LogSMSSender`, same pattern as
  `Dispatcher`/`LogDispatcher`) plus `ScanSMSRollover`, ticked every 30s from
  `cmd/worker`, finds emergency-broadcast recipients whose push was sent but
  not acknowledged within 3 minutes and sends the `EMERGENCY_HOLIDAY`-template
  fallback. A new `POST /api/v1/notices/{id}/ack` endpoint records the
  acknowledgment the mobile client's FCM handler is expected to call (mobile
  side not wired yet -- see below). Verified by composing an emergency
  broadcast, backdating `push_sent_at` past the window, and confirming
  `sms_sent_at` was set by the real worker process.
- **The `academic_calendar_days` entity now exists** (PRD 3.1: migration
  000015, `day_type` enum `regular_working/holiday/compensatory_working/
  half_day/exam_day`, RLS-protected, `PUT`/`GET /api/v1/academic-calendar`
  for the office to set and amend it). `AttendancePercentage` and
  `AttendancePercentageForStudent` now compute their denominator from working
  calendar days in range rather than counting recorded entries, so a
  retroactively-declared holiday correctly stops counting against a student
  even though its attendance entries are preserved (PRD 3.1's own example) --
  and recomputation is automatic since the calendar table is read live on
  every call, no cached total to invalidate. Falls back to the previous
  entry-count method only when a school has configured zero calendar days in
  the requested range, so a school that hasn't set up its calendar yet still
  gets *a* percentage rather than a false 0/0. Verified against real data: a
  student present on the one day with a recorded register, across a 5-working-
  day range with one of those days retroactively marked a holiday, correctly
  showed 1/4 (25%) -- not the 100% the old entry-count method would have shown,
  and not counting the holiday.
- **A second, more serious bug found while building the above, unrelated to
  any of it**: `ScanAbsenceNotifications` -- the absence-notification scan
  previously marked "Verified end-to-end in Docker" in this same file -- runs
  with no tenant context by design (it has to look across every school at
  once), and queried `attendance_entries`/`school_settings` directly with a
  plain `pool.Query`. Both are RLS-protected tenant tables, and this worker's
  pool connects as `app_user`, which per PRD 6.1 point 6 never gets
  `BYPASSRLS`. With no `app.current_school_id` set, Postgres's RLS policy
  (`school_id = current_school_id()`) evaluates `current_school_id()` as NULL
  and silently returns **zero rows on every school, with no error** -- meaning
  this scan could never have found a single school in real operation, ever.
  Confirmed directly: `psql -U app_user` with no tenant context set returned
  `count(*) = 0` against tables holding real rows. (The earlier "verified"
  note in this file was evidently exercising the fan-out/dispatch side with a
  job enqueued directly, not the scan query itself finding the school on its
  own -- worth remembering as a lesson: verify the *trigger*, not just what
  happens once triggered.) Fixed the same way `ScanSMSRollover` had to be
  built from the start: loop every school (from the global, non-RLS `schools`
  table) and check each under its own real tenant context via
  `db.WithTenantTx`, exactly like `dispatchAbsenceAlertsHandler` and
  `dispatchNotificationHandler` already did correctly for their own per-school
  work. Verified for real this time: marked a student absent, waited for the
  worker's real 1-minute scan tick (not a manually-enqueued job), and watched
  `absence_notified_at` actually get set.
- The backend's own `go test ./...` and the cross-tenant isolation suite
  (`go test -tags=integration ./test/...` -- `TestDirectIDAccessAcrossTenants
  ReturnsNothing`, `TestConcurrentPoolDoesNotLeakTenantContext`,
  `TestEveryTenantTableHasRLSPolicy`) both pass after these changes, the
  latter confirming `academic_calendar_days` carries the RLS policy the new
  migration adds.

### Backend: still not started

- Class diary is Phase 4 per the PRD, not Phase 2 -- not started, correctly.
- The mobile app's FCM background-message handler doesn't yet call the new
  `POST /api/v1/notices/{id}/ack` endpoint -- the backend half of SMS
  rollover (scan, send, mark) is real and verified above, but nothing on the
  device reports a push arriving yet, so in real operation every push would
  currently look "unacknowledged" and roll over to SMS after 3 minutes even
  when the app received it fine. Needs the mobile FCM integration itself
  first (this codebase's push dispatch is still `LogDispatcher`, no real
  Firebase project wired), so the ack call has something to attach to.

## Phase 3 — Fees (weeks 10-14 target)

PRD 4.4 opens with "This module carries the highest trust risk in the
product. Every requirement here is a correctness requirement" -- everything
below was built and then verified against the live Docker stack with real
data and real curl calls, not just compiled, specifically because of that
sentence. Backend only in this pass; the admin-panel UI for fee
configuration/collection is not built yet (see "Not built" below).

### Backend: done and verified in Docker

- **Fee structure configuration** (`internal/fees`, migration 000016): fee
  heads per academic year with the fixed statutory-category enumeration
  (PRD 4.4.1), versioned fee structures per class+year (`is_active`
  enforced unique per class+year by a partial unique index, not just
  application discipline) with an instalment schedule (head, label, amount,
  due date) per version. Changing a structure creates a new version and
  deactivates the old one; existing `fee_assignments` keep pointing at
  whichever version they were generated from, so a later edit never
  retroactively alters an issued demand -- verified by generating a
  demand, then confirming its line items still reference the original
  structure version.
- **Concessions with the PRD's sibling/staff-ward dependency tracking**,
  which the PRD spends real space on because both "continue silently" and
  "cancel automatically" are explicitly wrong: a concession keyed to an
  elder sibling's `student_id` (never `enrollment_id`, for the exact reason
  the PRD gives -- enrollments close every June at promotion). The
  dependency is checked at the moment a fee demand is next generated for the
  younger child (PRD's own phrasing), flips the concession to
  `review_required`, and blocks generation with the specific concession(s)
  in the error until the office makes an explicit continue/cancel/convert
  decision. **A real bug found and fixed here**: the status-flip and the
  blocked generation attempt were originally one transaction, so the abort
  that blocked generation also rolled back the flip -- meaning the office's
  review worklist stayed silently empty despite the office having just been
  told a concession needed attention. Fixed by committing the flip in its
  own transaction before the generation attempt runs at all. Verified
  end-to-end: created a sibling concession, ended the elder's enrollment,
  confirmed generation blocked *and* the concession showed up in the review
  worklist, resolved it as cancelled, confirmed generation then succeeded at
  full (un-discounted) amount.
- **Fee assignment generation** (PRD 4.4.2): resolves the student's
  enrollment for the year, finds the class's active structure, applies every
  currently-active concession per instalment (percentage concessions
  computed against gross and stacked additively, flat concessions drawn down
  from a running per-concession pool across instalments of the same head,
  both capped so concessions can never take net below zero), writes the
  dated `fee_line_items`. Verified with a real 10%-off-tuition-only merit
  concession: transport instalments came out at full gross, tuition
  instalments at exactly 90%.
- **Payment collection** (PRD 4.4.3): cash and cheque entry, receipt numbers
  assigned via an atomic `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`
  counter (never two collectors racing to the same number), allocation
  across specific line items with partial-payment support, and the
  overpayment/advance split (PRD 4.4.3.3) landing in a
  `student_credit_balances` running balance plus an append-only
  `credit_balance_transactions` ledger. Verified: a ₹20,000 cash payment
  against an ₹18,000 tuition instalment correctly split ₹18,000 allocated /
  ₹2,000 credited, with the line item flipping to `paid` and the credit
  balance showing separately (never netted into the outstanding figure, per
  PRD 4.4.3.3's explicit requirement) in the dues report.
- **A real Postgres semantics bug found and fixed in the credit-balance
  upsert**: `INSERT ... ON CONFLICT DO UPDATE` validates CHECK constraints
  against the `VALUES`-list candidate row even when the conflict path (the
  `UPDATE`) is what actually runs -- confirmed directly: a negative delta
  against an existing row with ample balance aborted with a check-constraint
  violation even though the *update* itself would have produced a valid
  balance. Fixed by wrapping the `VALUES` clause's candidate value in
  `GREATEST(0, $delta)` (the `UPDATE` branch's arithmetic is unaffected and
  still correct); documented in the code as a real semantics gotcha, not
  just a workaround, since a fresh row starting from a negative delta with
  nothing to debit yet would be a bug regardless.
- **Void and refunds** (PRD 4.4.4): void requires a mandatory reason,
  reverses the payment's line-item and credit effects, never deletes the
  payment or reuses its receipt number (the row stays, `is_void=true`).
  Refunds draw from the student's credit balance (PRD 4.4.3.3: "Credit is
  refundable, subject to the same authority and audit rules as any refund"),
  rejected cleanly if requested beyond the available balance. Voiding a
  cash payment whose business day has been closed is blocked (see cash
  drawer below). **A real bug found and fixed**: voiding an already-bounced
  cheque payment tried to reverse the line items a second time (the bounce
  had already reversed them once), driving `paid_amount_paise` negative and
  hitting its own CHECK constraint. Fixed by having void recognize a
  bounced-cheque payment's effects are already reversed and skip re-reversing
  them, only recording the void itself.
- **Cheque lifecycle** (PRD 4.4.3.3): received -> deposited -> cleared, or
  -> bounced from either prior state, enforced as a real transition table,
  not just accepting any value. A bounced cheque reinstates the dues it had
  covered and optionally (per-instance, per the PRD) adds a configurable
  cheque-return-charge line item, defaulting from `fee_settings` and
  auditable including when waived. Verified: a bounced cheque correctly put
  its transport instalment back to `pending` and added a real ₹250
  "Cheque Return Charge" line item to the student's dues.
- **Day-end cash drawer closing, PRD 4.4.6.1 in full**: blind count (the
  clerk's denomination breakdown is recorded *before* the expected total,
  computed from that collector's actual cash receipts for the day, is
  revealed), up to two recounts, a third attempt with a variance beyond
  tolerance rejected outright (without consuming the attempt) unless
  submitted together with a written explanation, and closing freezes every
  matching cash payment against further void/back-dated correction. Verified
  every rule directly: an exact-match count closed cleanly; a mismatched
  count exercised attempt 1 -> attempt 2 (recount) -> attempt 3 rejected
  without an explanation -> attempt 3 accepted with one -> a 4th attempt
  hard-blocked; closing a drawer and then attempting to void one of its
  payments was correctly rejected with "this payment's business day is
  closed."
- **Real-time dues and ageing** (PRD 4.4.5): computed live from
  `fee_line_items`/`student_credit_balances` on every call, never a cached
  total -- a void or a bounced cheque is reflected the instant it happens.
  Credit balance shown as its own column, never netted into the outstanding
  figure. The parent-facing `GET /api/v1/students/{id}/dues` endpoint (used
  by the mobile app since Phase 2) now prefers this real ledger the moment a
  student has any real `fee_assignment`, falling back to the Phase 2
  `fee_dues_snapshot` import only when they don't -- verified both paths on
  real students, including the pure-fallback case for a student with no
  assignment at all.
- **Regulatory export** (PRD 4.4.7): `GET /api/v1/fees/annexure` aggregates
  gross/concession/net/collected by statutory category for the state fee
  determination committee filing, a live query rather than a manual
  reclassification exercise, exactly per the PRD's own framing of why
  categorization happens at fee-head definition time. RTE tracked as a
  distinct category on `fee_assignments` (`is_rte` +
  `rte_reimbursement_status`) with a dedicated listing endpoint.
- **Webhook ingestion pipeline** (PRD 4.4.3.2): `webhook_events`, idempotency
  enforced by a real `UNIQUE(school_id, provider, idempotency_key)`
  constraint (not an application check), verified directly -- the identical
  payload sent twice produced exactly one row and a 200 both times, so a
  retrying provider stops retrying without ever being processed twice. **This
  table is correctly RLS-protected** (unlike `jobs`): a payment gateway is
  registered per school (PRD 4.4.3: "in the school's name"), so its webhook
  URL carries the school as a path segment and real tenant context is set
  before the row is ever written -- there's no cross-school ambiguity to
  justify the `jobs`-style exception here. The cross-tenant integration
  suite's `TestEveryTenantTableHasRLSPolicy` caught the first draft of this
  table missing that (it was written with the `jobs` reasoning applied
  somewhere it didn't actually fit) before it ever reached Docker.
- **UPI intent link generation** (PRD 4.4.3.1): the `upi://pay?...` deep
  link format with a unique per-demand reference. This is the whole of what
  PRD 4.4.3.1 itself says is buildable without external confirmation --
  see "Not built" below for why automatic reconciliation isn't.
- **A second RLS bug of the exact same shape, found proactively this time**:
  while building the webhook endpoint, the instinct was to reuse the `jobs`
  table's "no tenant context, no RLS" pattern, since a webhook also arrives
  outside any request-scoped context. Recognizing that a gateway is
  per-school (unlike the genuinely cross-school absence-notification scan
  fixed earlier this same session) avoided writing that bug into new code
  instead of just fixing the one already found -- worth recording as the
  actual lesson from that earlier bug, not just its fix.
- `go test ./...` and the cross-tenant isolation suite
  (`go test -tags=integration ./test/...`) both pass after all of the above,
  the latter confirming every new table is RLS-covered.

### Admin panel UI -- built in a later session than the backend above

The gap flagged below as "the single largest remaining gap" is closed:
`admin-panel/src/app/fees/` now has a working screen for every backend
capability above -- `/fees` (dues/ageing + student search), `/fees/students/
[id]` (the actual daily-use counter screen: generate a demand, view line
items, collect a cash/cheque payment with allocation across specific dues
plus the advance/credit split, void, refund from credit, cheque status
transitions including the bounce return-charge prompt, and concessions
including the sibling-dependency flow), `/fees/config` (heads and versioned
structures, correspondent-only), `/fees/concessions` (the review-required
worklist with continue/cancel/convert), `/fees/cash-drawer` (the full
blind-count/recount/close flow), `/fees/annexure` (the regulatory export
with CSV download). Role-gated throughout per the PRD 2.2 matrix.

`npm run build` and `npm run lint` both pass, and every route was confirmed
reachable (200) with the dev server running. **What this does not claim**:
no browser-automation tool was available in this environment, so no page was
actually click-tested end-to-end in a real browser -- no form was submitted,
no real API round-trip was watched happen from the UI. The backend behind
every one of these screens was independently verified for real (see above);
the screens themselves are unverified beyond compiling, linting, and
rendering their initial HTML shell. Treat this UI as needing a real
first-use pass by an actual person before relying on it at a real school.

**That warning was immediately justified**: the user's very first real
click -- voiding a payment on `/fees/students/[id]` -- hit a runtime error,
`window.prompt() is not supported`, because this deployment environment
doesn't support native browser dialogs at all. Both call sites (`prompt`
for the void reason, `confirm`+`prompt` for the bounced-cheque return-charge
decision) used them. Fixed by replacing both with inline panel UI using
component state instead of native dialogs; re-running lint after the fix
also caught two real ESLint errors (an unescaped apostrophe, a
set-state-in-effect case) the prior lint pass hadn't been re-run against.
Lesson worth keeping: "builds and lints clean" is not the same claim as
"was clicked," and this UI's very first real interaction proved the gap
between those two claims was not hypothetical.

### Fee event notifications -- built in the same later session

`payment_receipt`, `fee_due_reminder` and `fee_overdue_reminder` (PRD 4.5.3)
are now wired, not just valid enum values waiting for a caller:
- Payment receipts fire synchronously from `CollectPayment` via the existing
  `notify.Compose`, targeting every guardian of the student.
- Due/overdue reminders run from a new `fees.ScanFeeReminders`, ticked from
  `cmd/worker` alongside the existing scans, looped per-school under real
  tenant context from the start (applying the RLS lesson from earlier in
  this phase proactively rather than rediscovering it a third time). New
  `due_reminder_sent_at`/`overdue_reminder_sent_at` columns (migration
  000017) mean a due-soon reminder fires once and an overdue one repeats on
  a 7-day cooldown, not every tick.
- Both inherit the quiet-hours/daily-cap discipline automatically, with zero
  new enforcement code -- verified directly: a receipt notification to a
  guardian who'd already hit today's cap from earlier testing correctly
  deferred, then dispatched once the cap was raised.
- Verified against real data: a real payment produced a real notification to
  the correct guardian; one real worker tick produced exactly the two
  genuinely-qualifying reminders present in the data (one overdue, one
  due-soon), correctly worded and targeted, with every scanned line item
  marked so re-scanning won't duplicate them.

### A real Phase 2 gap found and closed in a later session, before starting Phase 4

Audited this file against the PRD rather than assuming its own "done" claims
were complete, per an explicit request to close real gaps before moving on.
Found one: **PRD 4.5.2 requires "per-guardian channel preference and
per-category opt-out (fee reminders, attendance, general notices), which is
also a DPDP consent requirement"** -- never built. The message-discipline
work earlier this phase (quiet hours, the daily cap) governs volume, not
consent; those are different requirements and neither one satisfies the
other.

- Migration 000018: four columns on `guardians` (`opt_out_fee_reminders`,
  `opt_out_attendance_alerts`, `opt_out_general_notices`, `sms_opt_out`).
- Enforced at the three actual notification-creation points: general
  notices in `notify.resolveRecipients` (scoped specifically to
  `kind == "notice"`, so it doesn't wrongly filter `payment_receipt`, which
  also targets a single student via the same code path), the absence-alert
  primary-guardian lookup, and the fee-reminder scan's fee-responsible-
  guardian lookup. Emergency broadcasts remain exempt from all of it, per
  PRD 4.5.3's own explicit exemption.
- New `PUT /api/v1/guardians/{id}/notification-preferences`, authorized for
  office roles or the guardian's own linked account (self-service),
  verified against the actual row rather than trusted from the role claim.
- Verified on real data across all three categories: opting a guardian out
  of fee reminders zeroed that exact lookup query; opting out of general
  notices made a real `Compose` call resolve to zero recipients and get
  rejected, while a `payment_receipt` to the same guardian was confirmed
  *not* wrongly filtered by that opt-out; opting out of attendance alerts
  zeroed the primary-guardian lookup. Preferences restored to default
  (opted-in) afterward.

### Not built, and why -- read before treating Phase 3 as fully done

- **No real payment gateway integration**, and this is not a code gap:
  PRD 9 (Phase 1, week 1) calls out gateway onboarding as an external
  business process requiring the school's own documentation, which nothing
  in this codebase can perform. `LogGatewayVerifier` accepts any webhook
  payload's signature unconditionally and is explicitly a stand-in, the
  same pattern as `notify.LogDispatcher`/`LogSMSSender`.
- **No automatic UPI reconciliation**, for the reason PRD 4.4.3.1 states
  itself: "a bare UPI deep link to a plain VPA gives the school no
  server-side notification... Confirm what the school's bank actually
  offers before designing around this." No school's actual bank capability
  is known yet -- this specific piece stays not-built until one is.

  **The honest fallback PRD 4.4.3.1 names in the same breath -- "a link plus
  a parent-submitted UTR that the office verifies against the bank
  statement" -- is now built** (a later session; PRD explicitly frames this
  as buildable without bank/gateway confirmation, unlike the reconciliation
  path above): `upi_payment_requests` (migration 000019), a real
  `upi://pay` intent link with declared allocations up front, a
  `pending -> utr_submitted -> verified/rejected` lifecycle, and verification
  replaying the declared allocations into a real `CollectPayment` call the
  moment an office human confirms the UTR against the bank statement.
  `fee_settings.upi_vpa` unset means request creation refuses outright
  rather than emitting a link to nowhere -- no school has a real VPA
  configured in this codebase yet, so this is the expected state until one
  does. Verified end-to-end with a real VPA configured: refused cleanly
  before configuration, then a full pending -> submitted -> verified cycle
  produced a real receipt and correctly paid off the covered line item;
  verifying before a UTR was submitted was correctly rejected; rejecting a
  request correctly left no payment behind.
- **No thermal ESC/POS receipt printing, and no PDF receipt generation.**
  PRD 4.4.3.3 is explicit this is "a specific engineering task, not a CSS
  afterthought" requiring testing against the school's actual physical
  printer -- genuinely impossible from here. Rather than write ESC/POS
  byte-sequence code that has never been run against real hardware and
  call that "done," nothing was written for either format. A payment
  collected through the API today has no printable receipt output at all --
  only the JSON record. This needs real printer/format access before it's
  worth building.
- **Gateway settlement reconciliation** (PRD 4.4.6: "match gateway payouts
  against recorded payments and flag discrepancies") depends on having a
  real gateway account to reconcile against -- not buildable until the
  gateway onboarding above happens.
- **Day-End Closing Voucher is structured data, not a generated document.**
  `CloseDrawer`'s response has everything the PRD's voucher needs (counted/
  expected/variance, denomination breakdown, the collector, the date) but
  nothing renders it as a signable PDF/printout yet -- same underlying gap
  as the receipt formats above (no PDF pipeline in this codebase at all
  yet).

## Phase 4+

Not started. See PRD section 9 (exams/report cards, certificates,
dashboards, class diary, timetable).
