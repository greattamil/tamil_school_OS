package enrollments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrNotFound              = errors.New("enrollments: not found")
	ErrOverlappingEnrollment = errors.New("enrollments: overlaps an existing enrollment for this student")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const selectColumns = `id, student_id, academic_year_id, class_id, section_id, roll_number, medium, group_code, lower(period), upper(period), status`

// Create opens a new enrollment with an unbounded upper period end (the student is
// enrolled indefinitely until a transfer, TC or promotion closes it). The
// no_overlapping_enrollment GiST exclusion constraint (PRD 3.3) is the actual
// integrity guarantee; this just surfaces its violation as a typed error instead of
// a raw Postgres error code.
func (r *Repository) Create(ctx context.Context, in CreateInput) (Enrollment, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Enrollment{}, db.ErrNoTenant
	}

	medium := in.Medium
	if medium == "" {
		medium = "tamil"
	}
	group := in.GroupCode
	if group == "" {
		group = "default"
	}

	var out Enrollment
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO enrollments (school_id, student_id, academic_year_id, class_id, section_id,
				roll_number, medium, group_code, period)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8, daterange($9::date, NULL))
			RETURNING `+selectColumns, schoolID, in.StudentID, in.AcademicYearID, in.ClassID, in.SectionID,
			in.RollNumber, medium, group, in.StartDate)
		return scanEnrollment(row, &out)
	})
	if err != nil {
		if isExclusionViolation(err) {
			return Enrollment{}, ErrOverlappingEnrollment
		}
		return Enrollment{}, err
	}
	return out, nil
}

func (r *Repository) ListForStudent(ctx context.Context, studentID uuid.UUID) ([]Enrollment, error) {
	out := []Enrollment{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+selectColumns+`
			FROM enrollments
			WHERE student_id = $1 AND deleted_at IS NULL
			ORDER BY lower(period)
		`, studentID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e Enrollment
			if err := scanEnrollment(rows, &e); err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

// RosterAsOf reconstructs a section's register for a specific date: whoever's
// enrollment period covered that date, not whoever is in the section today
// (PRD 3.3: "A section's October register must show who was in that section in
// October, not who is in it today").
func (r *Repository) RosterAsOf(ctx context.Context, sectionID uuid.UUID, asOf string) ([]Enrollment, error) {
	out := []Enrollment{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+selectColumns+`
			FROM enrollments
			WHERE section_id = $1 AND deleted_at IS NULL AND period @> $2::date
			ORDER BY roll_number
		`, sectionID, asOf)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e Enrollment
			if err := scanEnrollment(rows, &e); err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

// TransferSection closes the student's currently open enrollment at
// transferDate and opens a new one in the destination section from the same date,
// inside one transaction. Attendance and marks recorded against the old enrollment
// stay attached to it -- nothing is rewritten (PRD 3.3).
func (r *Repository) TransferSection(ctx context.Context, studentID, newClassID, newSectionID uuid.UUID, transferDate string, rollNumber *string) (Enrollment, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Enrollment{}, db.ErrNoTenant
	}

	var out Enrollment
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var currentID uuid.UUID
		var academicYearID uuid.UUID
		var medium, group string
		err := tx.QueryRow(ctx, `
			SELECT id, academic_year_id, medium, group_code
			FROM enrollments
			WHERE student_id = $1 AND status = 'active' AND deleted_at IS NULL AND upper_inf(period)
			FOR UPDATE
		`, studentID).Scan(&currentID, &academicYearID, &medium, &group)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE enrollments
			SET period = daterange(lower(period), $1::date)
			WHERE id = $2
		`, transferDate, currentID); err != nil {
			return err
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO enrollments (school_id, student_id, academic_year_id, class_id, section_id,
				roll_number, medium, group_code, period)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8, daterange($9::date, NULL))
			RETURNING `+selectColumns, schoolID, studentID, academicYearID, newClassID, newSectionID,
			rollNumber, medium, group, transferDate)
		return scanEnrollment(row, &out)
	})
	if err != nil {
		if isExclusionViolation(err) {
			return Enrollment{}, ErrOverlappingEnrollment
		}
		if errors.Is(err, ErrNotFound) {
			return Enrollment{}, ErrNotFound
		}
		return Enrollment{}, err
	}
	return out, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEnrollment(row scanner, e *Enrollment) error {
	return row.Scan(&e.ID, &e.StudentID, &e.AcademicYearID, &e.ClassID, &e.SectionID,
		&e.RollNumber, &e.Medium, &e.GroupCode, &e.StartDate, &e.EndDate, &e.Status)
}

func isExclusionViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23P01"
	}
	return false
}
