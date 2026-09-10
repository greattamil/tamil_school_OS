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
  the loop with a fresh install, is why this fix landed without a final
  post-fix on-device re-run -- it's implemented and reasoned through against
  Drift's own documented isolate-scoping behavior and the
  sqlcipher_flutter_libs README's explicit warning about this exact case,
  but the very last "empty roster now shows real students on a real device"
  screenshot is still owed). Fixed by passing `isolateSetup:` to
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
- **Not yet done**: period-wise/half-day attendance (PRD 4.2.1 mentions it as
  school-configurable; this pass only does whole-day), marks entry (Phase 4
  per the PRD's own module scope, not missing from Phase 2), the pending-
  register-older-than-24-hours warning (PRD 4.2.5), and the
  battery-optimization-exemption onboarding prompt (PRD 4.2.5) -- all
  reasonable follow-ups once the core sync mechanism (done) is exercised on
  a real device over multiple days. Also owed: a fresh on-device run to
  visually confirm the isolateSetup fix above (blocked mid-session by the
  shared emulator running out of storage, not by anything in the fix
  itself).
- **CI now covers mobile too**: added a `mobile` job to
  `.github/workflows/backend-ci.yml` (`flutter analyze` + `flutter test`,
  the in-memory-database suite -- no emulator needed for that, so it's a
  normal fast CI job, not something blocked by the storage issue above).
  The workflow's display name changed from `backend-ci` to `ci` to match.

### Backend: not yet started

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
