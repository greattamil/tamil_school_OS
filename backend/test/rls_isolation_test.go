//go:build integration

// Package test holds the cross-tenant isolation tests required by PRD 6.1 to pass
// in CI on every commit:
//   - direct-ID access into another school's data returns not-found, not the row
//   - a concurrent pool of size 1 never lets one school's tenant context leak onto
//     a request belonging to another (the pooling-leak test -- a sequential test
//     would not catch this)
//   - every table carrying a school_id column has RLS enabled and a policy,
//     discovered from the catalog rather than a maintained list
//
// Run against a disposable database with migrations already applied:
//
//	docker compose up -d postgres
//	docker compose run --rm migrate
//	TEST_DATABASE_URL="postgres://app_user:$APP_DB_PASSWORD@localhost:5432/school_erp?sslmode=disable" \
//	  go test -tags=integration ./test/...
package test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T, maxConns int32) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func createSchool(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO schools (name, short_code) VALUES ($1, $2) RETURNING id`,
		name, name+"-"+uuid.NewString()[:8],
	).Scan(&id)
	if err != nil {
		t.Fatalf("create school: %v", err)
	}
	return id
}

// insertStudent sets the tenant context to schoolID for the duration of the insert,
// exactly as db.WithTenantTx does in application code.
func insertStudent(t *testing.T, pool *pgxpool.Pool, schoolID uuid.UUID, admissionNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_school_id', $1, true)`, schoolID.String()); err != nil {
		t.Fatalf("set tenant context: %v", err)
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO students (school_id, admission_number, name_english, date_of_birth, gender, admission_date)
		VALUES ($1, $2, 'Test Student', '2015-01-01', 'F', '2024-06-01')
		RETURNING id
	`, schoolID, admissionNumber).Scan(&id)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return id
}

// studentIDsAsSchool opens a fresh transaction, scopes it to schoolID, and returns
// every student ID visible under that scope.
func studentIDsAsSchool(ctx context.Context, pool *pgxpool.Pool, schoolID uuid.UUID) ([]uuid.UUID, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_school_id', $1, true)`, schoolID.String()); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `SELECT id FROM students`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func TestDirectIDAccessAcrossTenantsReturnsNothing(t *testing.T) {
	pool := testPool(t, 0)
	ctx := context.Background()

	schoolA := createSchool(t, pool, "School A")
	schoolB := createSchool(t, pool, "School B")
	studentInB := insertStudent(t, pool, schoolB, "B-001")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_school_id', $1, true)`, schoolA.String()); err != nil {
		t.Fatalf("set tenant context: %v", err)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM students WHERE id = $1`, studentInB).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Fatalf("school A directly queried school B's student by ID and got %d rows, want 0", count)
	}
}

// TestConcurrentPoolDoesNotLeakTenantContext is the test that catches pooling
// leaks: a pool capped at one connection forces every transaction to reuse the same
// physical connection sequentially, so if SET LOCAL's transaction scope were
// broken, one school's context would bleed into the next school's query.
func TestConcurrentPoolDoesNotLeakTenantContext(t *testing.T) {
	setupPool := testPool(t, 0)
	schoolA := createSchool(t, setupPool, "Concurrent School A")
	schoolB := createSchool(t, setupPool, "Concurrent School B")
	studentA := insertStudent(t, setupPool, schoolA, "CA-001")
	studentB := insertStudent(t, setupPool, schoolB, "CB-001")

	pool := testPool(t, 1) // the pool under test: exactly one physical connection
	ctx := context.Background()

	const iterations = 25
	var wg sync.WaitGroup
	errCh := make(chan error, iterations*2)

	check := func(schoolID, expectStudent, forbiddenStudent uuid.UUID) {
		defer wg.Done()
		ids, err := studentIDsAsSchool(ctx, pool, schoolID)
		if err != nil {
			errCh <- err
			return
		}
		found := false
		for _, id := range ids {
			if id == forbiddenStudent {
				errCh <- errCondition(schoolID, forbiddenStudent)
				return
			}
			if id == expectStudent {
				found = true
			}
		}
		if !found {
			errCh <- errMissing(schoolID, expectStudent)
		}
	}

	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go check(schoolA, studentA, studentB)
		go check(schoolB, studentB, studentA)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

func errCondition(schoolID, forbidden uuid.UUID) error {
	return &tenantLeakError{schoolID: schoolID, forbiddenStudent: forbidden}
}

func errMissing(schoolID, expected uuid.UUID) error {
	return &tenantMissingError{schoolID: schoolID, expectedStudent: expected}
}

type tenantLeakError struct {
	schoolID         uuid.UUID
	forbiddenStudent uuid.UUID
}

func (e *tenantLeakError) Error() string {
	return "tenant leak: school " + e.schoolID.String() + " saw student " + e.forbiddenStudent.String() + " which belongs to another school"
}

type tenantMissingError struct {
	schoolID        uuid.UUID
	expectedStudent uuid.UUID
}

func (e *tenantMissingError) Error() string {
	return "school " + e.schoolID.String() + " did not see its own student " + e.expectedStudent.String()
}

// globalTablesWithSchoolIDColumn are the identity tables that sit outside the
// tenant boundary by design (PRD 3.2.1): sessions.school_id records which school a
// token was scoped to, and user_school_roles.school_id is the join column between a
// global user and a school. Neither is tenant-owned data, so neither carries RLS.
// This set is exactly the "four tables, one package, no business logic" identity
// layer the PRD calls out as deliberately small and reviewed with care -- distinct
// from the open-ended set of business tables this test exists to catch, which is
// why it is safe to name these explicitly rather than infer them from the catalog.
var globalTablesWithSchoolIDColumn = map[string]bool{
	"sessions":          true,
	"user_school_roles": true,
}

// TestEveryTenantTableHasRLSPolicy queries the catalog for every base table in the
// public schema carrying a school_id column, and fails the build if any tenant
// table lacks row-level security or a policy -- so a new tenant table added without
// RLS fails CI rather than relying on someone remembering to update a list
// (PRD 6.1).
func TestEveryTenantTableHasRLSPolicy(t *testing.T) {
	pool := testPool(t, 0)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT c.relname, c.relrowsecurity,
		       EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname = 'public' AND p.tablename = c.relname) AS has_policy
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind = 'r'
		  AND EXISTS (
		    SELECT 1 FROM information_schema.columns col
		    WHERE col.table_schema = 'public' AND col.table_name = c.relname AND col.column_name = 'school_id'
		  )
	`)
	if err != nil {
		t.Fatalf("query catalog: %v", err)
	}
	defer rows.Close()

	var checked int
	for rows.Next() {
		var name string
		var rls, hasPolicy bool
		if err := rows.Scan(&name, &rls, &hasPolicy); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if globalTablesWithSchoolIDColumn[name] {
			continue
		}
		checked++
		if !rls {
			t.Errorf("table %q has a school_id column but row-level security is not enabled", name)
		}
		if !hasPolicy {
			t.Errorf("table %q has a school_id column but no RLS policy", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if checked == 0 {
		t.Fatal("catalog query found no tenant tables -- migrations likely not applied against TEST_DATABASE_URL")
	}
}
