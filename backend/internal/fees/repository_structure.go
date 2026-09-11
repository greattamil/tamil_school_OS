package fees

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrDuplicateHeadName = errors.New("fees: fee head name already exists for this year")
	ErrStructureNotFound = errors.New("fees: fee structure not found")
	ErrNoActiveStructure = errors.New("fees: no active fee structure for this class and year")
)

// CreateFeeHead answers PRD 4.4.1: "Fee heads defined per academic year,
// assignable per class" with "Every fee head carries a statutory category".
func (r *Repository) CreateFeeHead(ctx context.Context, academicYearID uuid.UUID, name string, category StatutoryCategory) (FeeHead, error) {
	var out FeeHead
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)
		return tx.QueryRow(ctx, `
			INSERT INTO fee_heads (school_id, academic_year_id, name, statutory_category)
			VALUES ($1, $2, $3, $4)
			RETURNING id, academic_year_id, name, statutory_category, created_at
		`, schoolID, academicYearID, name, category).
			Scan(&out.ID, &out.AcademicYearID, &out.Name, &out.StatutoryCategory, &out.CreatedAt)
	})
	if err != nil {
		if isUniqueViolationFees(err) {
			return FeeHead{}, ErrDuplicateHeadName
		}
		return FeeHead{}, err
	}
	return out, nil
}

func (r *Repository) ListFeeHeads(ctx context.Context, academicYearID uuid.UUID) ([]FeeHead, error) {
	out := []FeeHead{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, academic_year_id, name, statutory_category, created_at
			FROM fee_heads WHERE academic_year_id = $1 AND deleted_at IS NULL
			ORDER BY name
		`, academicYearID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var h FeeHead
			if err := rows.Scan(&h.ID, &h.AcademicYearID, &h.Name, &h.StatutoryCategory, &h.CreatedAt); err != nil {
				return err
			}
			out = append(out, h)
		}
		return rows.Err()
	})
	return out, err
}

// CreateFeeStructure creates the next version for (class, academic year) and
// its instalment schedule, activating it and deactivating whatever was
// active before -- PRD 4.4.1: "Fee structures are versioned; changing a
// structure mid-year does not retroactively alter issued demands." The
// previous version's row (and every fee_assignment that still points at it)
// is untouched; only new assignments from this point resolve to the new one.
func (r *Repository) CreateFeeStructure(ctx context.Context, academicYearID, classID uuid.UUID, createdBy uuid.UUID, instalments []InstalmentInput) (FeeStructure, error) {
	if len(instalments) == 0 {
		return FeeStructure{}, errors.New("fees: a structure needs at least one instalment")
	}
	var out FeeStructure
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var nextVersion int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(version), 0) + 1 FROM fee_structures
			WHERE academic_year_id = $1 AND class_id = $2
		`, academicYearID, classID).Scan(&nextVersion); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE fee_structures SET is_active = false, superseded_at = now()
			WHERE academic_year_id = $1 AND class_id = $2 AND is_active
		`, academicYearID, classID); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO fee_structures (school_id, academic_year_id, class_id, version, is_active, created_by)
			VALUES ($1, $2, $3, $4, true, $5)
			RETURNING id, academic_year_id, class_id, version, is_active, created_at
		`, schoolID, academicYearID, classID, nextVersion, createdBy).
			Scan(&out.ID, &out.AcademicYearID, &out.ClassID, &out.Version, &out.IsActive, &out.CreatedAt); err != nil {
			return err
		}

		batch := &pgx.Batch{}
		for _, ins := range instalments {
			dueDate, err := time.Parse("2006-01-02", ins.DueDate)
			if err != nil {
				return fmt.Errorf("instalment due_date must be YYYY-MM-DD: %w", err)
			}
			if ins.AmountPaise < 0 {
				return errors.New("instalment amount_paise must not be negative")
			}
			batch.Queue(`
				INSERT INTO fee_structure_instalments (school_id, structure_id, fee_head_id, label, amount_paise, due_date)
				VALUES ($1,$2,$3,$4,$5,$6)
			`, schoolID, out.ID, ins.FeeHeadID, ins.Label, ins.AmountPaise, dueDate)
		}
		br := tx.SendBatch(ctx, batch)
		defer br.Close()
		for range instalments {
			if _, err := br.Exec(); err != nil {
				return fmt.Errorf("insert instalment: %w", err)
			}
		}
		return br.Close()
	})
	if err != nil {
		return FeeStructure{}, err
	}
	return out, nil
}

func (r *Repository) ListInstalments(ctx context.Context, structureID uuid.UUID) ([]Instalment, error) {
	out := []Instalment{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, structure_id, fee_head_id, label, amount_paise, due_date
			FROM fee_structure_instalments WHERE structure_id = $1
			ORDER BY due_date
		`, structureID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var i Instalment
			if err := rows.Scan(&i.ID, &i.StructureID, &i.FeeHeadID, &i.Label, &i.AmountPaise, &i.DueDate); err != nil {
				return err
			}
			out = append(out, i)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) ListFeeStructures(ctx context.Context, academicYearID, classID uuid.UUID) ([]FeeStructure, error) {
	out := []FeeStructure{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, academic_year_id, class_id, version, is_active, created_at
			FROM fee_structures WHERE academic_year_id = $1 AND class_id = $2
			ORDER BY version DESC
		`, academicYearID, classID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s FeeStructure
			if err := rows.Scan(&s.ID, &s.AcademicYearID, &s.ClassID, &s.Version, &s.IsActive, &s.CreatedAt); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

func isUniqueViolationFees(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
