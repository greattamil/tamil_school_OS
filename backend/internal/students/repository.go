package students

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var ErrNotFound = errors.New("students: not found")
var ErrDuplicateAdmissionNumber = errors.New("students: admission number already in use")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a student scoped to the tenant on ctx, inside a transaction with
// SET LOCAL app.current_school_id (via db.WithTenantTx) so RLS enforces the same
// boundary the query itself already targets -- two independent mechanisms, per
// PRD 6.1 point 4.
func (r *Repository) Create(ctx context.Context, in CreateStudentInput) (Student, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Student{}, db.ErrNoTenant
	}

	var out Student
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO students (
				school_id, admission_number, name_english, name_tamil, date_of_birth,
				gender, mother_tongue, blood_group, address, previous_school,
				rte_quota, admission_date
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			RETURNING id, admission_number, name_english, name_tamil, date_of_birth,
				gender, mother_tongue, blood_group, address, previous_school,
				rte_quota, pen_number, apaar_id, emis_number, admission_date, created_at
		`, schoolID, in.AdmissionNumber, in.NameEnglish, in.NameTamil, in.DateOfBirth,
			in.Gender, in.MotherTongue, in.BloodGroup, in.Address, in.PreviousSchool,
			in.RTEQuota, in.AdmissionDate)
		return scanStudent(row, &out)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Student{}, ErrDuplicateAdmissionNumber
		}
		return Student{}, err
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Student, error) {
	var out Student
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, admission_number, name_english, name_tamil, date_of_birth,
				gender, mother_tongue, blood_group, address, previous_school,
				rte_quota, pen_number, apaar_id, emis_number, admission_date, created_at
			FROM students
			WHERE id = $1 AND deleted_at IS NULL
		`, id)
		return scanStudent(row, &out)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Student{}, ErrNotFound
		}
		return Student{}, err
	}
	return out, nil
}

// List returns up to limit students with admission_number greater than cursor
// (cursor pagination on all list endpoints, PRD 8.5). deleted_at IS NULL is applied
// unconditionally -- exclusion of soft-deleted rows is structural, not left to the
// caller (PRD 3.3).
func (r *Repository) List(ctx context.Context, cursor string, limit int) ([]Student, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	out := []Student{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, admission_number, name_english, name_tamil, date_of_birth,
				gender, mother_tongue, blood_group, address, previous_school,
				rte_quota, pen_number, apaar_id, emis_number, admission_date, created_at
			FROM students
			WHERE deleted_at IS NULL AND admission_number > $1
			ORDER BY admission_number
			LIMIT $2
		`, cursor, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s Student
			if err := scanStudent(rows, &s); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanStudent(row scanner, s *Student) error {
	return row.Scan(
		&s.ID, &s.AdmissionNumber, &s.NameEnglish, &s.NameTamil, &s.DateOfBirth,
		&s.Gender, &s.MotherTongue, &s.BloodGroup, &s.Address, &s.PreviousSchool,
		&s.RTEQuota, &s.PENNumber, &s.APAARID, &s.EMISNumber, &s.AdmissionDate, &s.CreatedAt,
	)
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
