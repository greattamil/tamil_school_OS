package staff

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var ErrNotFound = errors.New("staff: not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, in CreateStaffInput) (Staff, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Staff{}, db.ErrNoTenant
	}

	var out Staff
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO staff (school_id, name, designation, qualification, date_of_joining, is_teaching)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, name, designation, qualification, date_of_joining, emis_staff_id, is_teaching, created_at
		`, schoolID, in.Name, in.Designation, in.Qualification, in.DateOfJoining, in.IsTeaching)
		return scanStaff(row, &out)
	})
	return out, err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Staff, error) {
	var out Staff
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, name, designation, qualification, date_of_joining, emis_staff_id, is_teaching, created_at
			FROM staff WHERE id = $1 AND deleted_at IS NULL
		`, id)
		return scanStaff(row, &out)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Staff{}, ErrNotFound
		}
		return Staff{}, err
	}
	return out, nil
}

func (r *Repository) List(ctx context.Context) ([]Staff, error) {
	out := []Staff{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, name, designation, qualification, date_of_joining, emis_staff_id, is_teaching, created_at
			FROM staff WHERE deleted_at IS NULL ORDER BY name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s Staff
			if err := scanStaff(rows, &s); err != nil {
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

func scanStaff(row scanner, s *Staff) error {
	return row.Scan(&s.ID, &s.Name, &s.Designation, &s.Qualification, &s.DateOfJoining, &s.EMISStaffID, &s.IsTeaching, &s.CreatedAt)
}
