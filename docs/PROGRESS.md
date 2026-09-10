# Build progress

Tracking against the PRD's phased release plan (section 9). Update this file at the
end of each work session rather than relying on git log to reconstruct status.

## Phase 1 — Foundation (target: weeks 1-4)

Done:
- Multi-tenant schema: schools, academic_years (with the active/soft_closing/locked
  state machine columns), classes, sections, students, guardians,
  student_guardians, enrollments (daterange + GiST exclusion constraint against
  overlapping enrollments), staff, section_teachers, audit_log.
- Global identity tables outside the tenant boundary: users, credentials, sessions,
  otp_codes, user_school_roles.
- RLS enabled + `tenant_isolation` policy on every tenant table, driven by a single
  `current_school_id()` helper, applied via a catalog-driven loop (not a maintained
  table list).
- `app_user` Postgres role created separately from the migration-owning role, with
  no BYPASSRLS and no table ownership.
- Go backend skeleton (modular monolith, `internal/<module>` layout): `tenancy`,
  `db` (transaction-scoped `SET LOCAL app.current_school_id`), `auth` (argon2id
  password hashing, JWT access tokens, opaque hashed refresh tokens, OTP request/
  verify, school selection/token exchange), `students` (create/get/list, cursor
  pagination), `audit` (delta-based append-only writer), `httpmw` (auth middleware
  with `tokens_valid_after` revocation check).
- Cross-tenant isolation integration tests: direct-ID access across tenants,
  concurrent single-connection pool leak test, and a catalog query asserting every
  `school_id`-bearing table has RLS + a policy.
- Docker Compose local stack: Postgres 16, Redis, one-shot migrate service, API.

Not yet done (explicitly deferred, not forgotten):
- Bulk import (Excel/CSV) for students and staff, with the guardian auto-linking
  heuristics from PRD 4.1.3. This is substantial and adoption-critical; next up.
- Admin panel scaffolding (Next.js) — not yet started.
- SMS/OTP dispatch is stubbed (returns the code directly in the dev response body);
  real DLT-registered gateway integration is Phase 5 per the PRD, but the `OTP_LOGIN`
  template itself should be filed in week 1 per PRD 9 external-process note.
- Redis is provisioned in Docker Compose but not yet wired into the app (session
  revocation cache, rate limiting) — currently every revocation check hits Postgres
  directly, which is correct but not yet the cached-for-speed design in PRD 6.2.
- CI pipeline (GitHub Actions) to actually run the integration tests on every
  commit, per PRD 6.1's "must run on every commit" requirement — tests exist and
  pass locally against Docker; wiring them into CI is next.
- Staff/admin CRUD beyond students; TC-required and EMIS structured fields exist as
  columns on `students` but there's no bulk import or admin UI to populate them yet.

## Phase 2+

Not started. See PRD section 9 for scope (attendance offline sync, parent app, fee
dues read-only view, then the full fee module, exams/report cards, certificates,
communication, dashboards, class diary).
