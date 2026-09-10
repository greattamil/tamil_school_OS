# Tamil School OS

Multi-tenant school management system for Tamil Nadu private matriculation schools.
Full requirements: [docs/PRD.md](docs/PRD.md).

Built in phases per the PRD's release plan (section 9). See
[docs/PROGRESS.md](docs/PROGRESS.md) for what's implemented so far.

## Stack

| Layer | Technology |
|---|---|
| Backend | Go, modular monolith |
| Database | PostgreSQL 16 (row-level security) |
| Cache | Redis |
| Admin panel | Next.js (added Phase 1, deepened later) |
| Mobile | Flutter (added Phase 2) |
| Local dev / deploy | Docker Compose |

## Local development

1. Copy `.env.example` to `.env` and fill in real values (`openssl rand -base64 48` for `JWT_SECRET`).
2. Bring up the stack:

   ```
   docker compose up -d
   ```

   This starts Postgres and Redis, runs migrations to completion, then starts the API
   and the background worker (`cmd/worker`: job queue processing, the
   absence-notification scan).
3. Check it's up:

   ```
   curl http://localhost:8080/healthz
   ```

### Re-running migrations after changing them

```
docker compose run --rm migrate
```

### Admin panel

The API must already be running (via Docker Compose, above) since the admin panel
talks to it over HTTP.

```
cd admin-panel
cp .env.local.example .env.local
npm install
npm run dev
```

Open http://localhost:3000. There's no self-serve sign-up yet -- create a school and
its first correspondent login with the seed tool:

```
cd backend
DATABASE_URL="postgres://app_user:$APP_DB_PASSWORD@localhost:5432/school_erp?sslmode=disable" \
  go run ./cmd/seed -school "My School" -short-code myschool \
  -email admin@example.com -name "Admin Name" -password "ChangeMe123!"
```

### Running tests

Unit tests (no Docker required):

```
cd backend
go test ./...
```

Cross-tenant isolation integration tests (require Postgres up and migrated — this is
the suite PRD section 6.1 requires to pass in CI on every commit):

```
docker compose up -d postgres
docker compose run --rm migrate
cd backend
TEST_DATABASE_URL="postgres://app_user:$APP_DB_PASSWORD@localhost:5432/school_erp?sslmode=disable" \
  go test -tags=integration ./test/...
```

## Repository layout

```
backend/        Go modular monolith (internal/<module> per PRD 8.2)
  migrations/   Numbered SQL migrations (golang-migrate format)
  test/         Cross-tenant / RLS integration tests
admin-panel/    Next.js admin panel (scaffolded in Phase 1)
mobile/         Flutter app (added Phase 2)
deploy/         Postgres init scripts, ops assets
docs/           PRD and build progress notes
```
