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
- Go backend (modular monolith, `internal/<module>` layout):
  - `tenancy`, `db` (transaction-scoped `SET LOCAL app.current_school_id`)
  - `auth`: argon2id password hashing, JWT access tokens, opaque hashed refresh
    tokens, OTP request/verify, school selection/token exchange,
    `tokens_valid_after` revocation
  - `academic`: academic years (with the state machine transition, correspondent-
    only, audited), classes, sections
  - `students`, `staff`: CRUD with cursor pagination
  - `guardians`: create, find-by-mobile (dedup-on-lookup), link/list for a student
  - `enrollments`: create, list for student, **mid-year section transfer**
    (closes the old enrollment's period, opens a new one, same transaction), and
    **roster reconstruction by date** (`GET /sections/{id}/roster?date=`) —
    verified against real transfer scenarios in Docker
  - `bulkimport`: CSV preview/commit for student admission with the **guardian
    auto-linking safety heuristics** from PRD 4.1.3 as a standalone, unit-tested
    package (`bulkimport/guardianlink`): placeholder/sequential-digit rejection,
    Indian mobile format validation, >4-students-per-number blocks auto-merge and
    routes to manual review, with per-row error reporting (row/column/reason)
  - `audit`: delta-based append-only writer
  - `httpmw`: auth middleware with revocation check
- Cross-tenant isolation integration tests: direct-ID access across tenants,
  concurrent single-connection pool leak test, and a catalog query asserting every
  `school_id`-bearing table has RLS + a policy.
- **CI**: `.github/workflows/backend-ci.yml` runs unit tests on every push/PR, and
  replicates the local Docker Compose flow (create `app_user`, run migrations,
  run the cross-tenant integration suite) against a Postgres service container —
  verified locally by replaying the exact same steps against a throwaway
  container before trusting the workflow file.
- Docker Compose local stack: Postgres 16, Redis, one-shot migrate service, API —
  full flow (seed two schools → login → bulk import 15-row CSV with a 5-student
  guardian cluster requiring manual review and a placeholder cluster requiring
  rejection → mid-year section transfer → roster-by-date reconstruction)
  exercised end-to-end over real HTTP against live containers, not just unit
  tests.

- **Admin panel scaffolding (Next.js 16, App Router, TypeScript, Tailwind)**:
  login (staff email/mobile + password), school picker for staff with roles at
  more than one school, students list + create form, and the bulk-import UI
  (CSV upload -> preview with per-row errors and the guardian-cluster review
  list -> commit). Verified end-to-end with a real headless-browser session
  (Playwright) against the live Docker stack: login, student creation, and a
  full CSV import including picking "link" on a manual-review cluster -- zero
  console errors, screenshots inspected. **Required adding CORS middleware to
  the Go backend** (`internal/httpmw/cors.go`, `ALLOWED_ORIGINS` env var):
  curl-based testing never exercises the browser's CORS preflight, so this
  gap was invisible until the admin panel was actually driven in a browser --
  worth remembering for the mobile app's HTTP layer too, though native HTTP
  clients aren't subject to CORS the way browsers are.
  Token storage is localStorage, called out in `api.ts` as scaffold-quality:
  hardening (httpOnly cookies via a route-handler proxy, or a BFF pattern) is
  follow-up work, not a Phase 1 blocker.

Not yet done (explicitly deferred, not forgotten):
- SMS/OTP dispatch is stubbed (returns the code directly in the dev response body);
  real DLT-registered gateway integration is Phase 5 per the PRD, but the `OTP_LOGIN`
  template itself should be filed in week 1 per PRD 9 external-process note.
- Redis is provisioned in Docker Compose but not yet wired into the app (session
  revocation cache, rate limiting) — currently every revocation check hits Postgres
  directly, which is correct but not yet the cached-for-speed design in PRD 6.2.
- Excel (.xlsx) import isn't implemented, only CSV — PRD 4.1.3 says "Excel/CSV";
  CSV covers the mechanism end-to-end (parsing, validation, guardian heuristics),
  adding an .xlsx reader on top is comparatively small follow-up work.
- Bulk import for staff (only students are implemented).
- No role/permission enforcement yet beyond the one correspondent-only check on
  academic year transitions — the full role matrix (PRD 2.2) needs a proper
  authorization layer before Phase 2, not per-handler ad hoc checks.

## Phase 2+

Not started. See PRD section 9 for scope (attendance offline sync, parent app, fee
dues read-only view, then the full fee module, exams/report cards, certificates,
communication, dashboards, class diary).
