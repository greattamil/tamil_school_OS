package guardians

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var ErrNotFound = errors.New("guardians: not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, name, mobile string, email, occupation *string) (Guardian, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Guardian{}, db.ErrNoTenant
	}

	var out Guardian
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO guardians (school_id, name, mobile, email, occupation)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id, name, mobile, email, occupation, created_at
		`, schoolID, name, mobile, email, occupation).Scan(&out.ID, &out.Name, &out.Mobile, &out.Email, &out.Occupation, &out.CreatedAt)
	})
	return out, err
}

// FindByMobile returns every guardian at this school already registered under
// mobile, so the caller can offer to link rather than create a duplicate
// (PRD 4.1.2: "Detect existing guardian by mobile number and offer to link rather
// than duplicate -- this is how sibling grouping happens").
func (r *Repository) FindByMobile(ctx context.Context, mobile string) ([]Guardian, error) {
	out := []Guardian{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, name, mobile, email, occupation, created_at
			FROM guardians WHERE mobile = $1 AND deleted_at IS NULL
		`, mobile)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var g Guardian
			if err := rows.Scan(&g.ID, &g.Name, &g.Mobile, &g.Email, &g.Occupation, &g.CreatedAt); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}

// LinkToStudent attaches an existing guardian to a student. Uniqueness of
// (student_id, guardian_id) is enforced by the schema; callers that need "one
// primary contact" / "one fee-responsible" semantics must clear existing flags
// themselves in the same request if that invariant matters to them.
func (r *Repository) LinkToStudent(ctx context.Context, link StudentGuardianLink) error {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return db.ErrNoTenant
	}

	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO student_guardians (school_id, student_id, guardian_id, relationship, is_primary_contact, is_fee_responsible)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (student_id, guardian_id) DO UPDATE SET
				relationship = EXCLUDED.relationship,
				is_primary_contact = EXCLUDED.is_primary_contact,
				is_fee_responsible = EXCLUDED.is_fee_responsible
		`, schoolID, link.StudentID, link.GuardianID, link.Relationship, link.IsPrimaryContact, link.IsFeeResponsible)
		return err
	})
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Guardian, error) {
	var out Guardian
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, name, mobile, email, occupation, created_at
			FROM guardians WHERE id = $1 AND deleted_at IS NULL
		`, id).Scan(&out.ID, &out.Name, &out.Mobile, &out.Email, &out.Occupation, &out.CreatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Guardian{}, ErrNotFound
		}
		return Guardian{}, err
	}
	return out, nil
}

// Child is a student as seen from a parent's own dashboard (PRD 4.8): enough
// to identify and switch between children, not the full student record.
type Child struct {
	StudentID       uuid.UUID    `json:"student_id"`
	NameEnglish     string       `json:"name_english"`
	AdmissionNumber string       `json:"admission_number"`
	Relationship    Relationship `json:"relationship"`
}

// ListChildrenForUser answers "which students is this logged-in parent a
// guardian of" -- the query the parent app's child selector runs right after
// login (PRD 4.8: "A parent with two or more children in the school switches
// between them with a child selector"). There is deliberately no equivalent
// "list my schools" cross-tenant version of this: which schools a user holds
// a role at comes from the global user_school_roles table (PRD 3.2.1); this
// answers the tenant-scoped question of which students they're linked to
// *within* the school the caller's session is already scoped to.
func (r *Repository) ListChildrenForUser(ctx context.Context, userID uuid.UUID) ([]Child, error) {
	out := []Child{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.name_english, s.admission_number, sg.relationship
			FROM student_guardians sg
			JOIN guardians g ON g.id = sg.guardian_id
			JOIN students s ON s.id = sg.student_id
			WHERE g.user_id = $1 AND sg.deleted_at IS NULL AND g.deleted_at IS NULL AND s.deleted_at IS NULL
			ORDER BY s.name_english
		`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Child
			if err := rows.Scan(&c.StudentID, &c.NameEnglish, &c.AdmissionNumber, &c.Relationship); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

type GuardianWithLink struct {
	Guardian
	Relationship     Relationship `json:"relationship"`
	IsPrimaryContact bool         `json:"is_primary_contact"`
	IsFeeResponsible bool         `json:"is_fee_responsible"`
}

func (r *Repository) ListForStudent(ctx context.Context, studentID uuid.UUID) ([]GuardianWithLink, error) {
	out := []GuardianWithLink{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT g.id, g.name, g.mobile, g.email, g.occupation, g.created_at,
			       sg.relationship, sg.is_primary_contact, sg.is_fee_responsible
			FROM student_guardians sg
			JOIN guardians g ON g.id = sg.guardian_id
			WHERE sg.student_id = $1 AND sg.deleted_at IS NULL AND g.deleted_at IS NULL
		`, studentID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var g GuardianWithLink
			if err := rows.Scan(&g.ID, &g.Name, &g.Mobile, &g.Email, &g.Occupation, &g.CreatedAt,
				&g.Relationship, &g.IsPrimaryContact, &g.IsFeeResponsible); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}
