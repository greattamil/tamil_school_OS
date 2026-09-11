package fees

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/audit"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var ErrConcessionNotFound = errors.New("fees: concession not found")

type CreateConcessionInput struct {
	StudentID        uuid.UUID
	ConcessionType   ConcessionType
	FeeHeadID        *uuid.UUID
	Percentage       *float64
	FlatAmountPaise  *int64
	ElderStudentID   *uuid.UUID
	DependentStaffID *uuid.UUID
	Reason           string
}

// CreateConcession answers PRD 4.4.1: "Each concession recorded with an
// approver and a reason." Exactly one of Percentage/FlatAmountPaise must be
// set (the DB constraint enforces this too, but validating here gives a
// clean error instead of a raw constraint-violation message).
func (r *Repository) CreateConcession(ctx context.Context, in CreateConcessionInput, approverID uuid.UUID) (Concession, error) {
	if (in.Percentage == nil) == (in.FlatAmountPaise == nil) {
		return Concession{}, errors.New("fees: exactly one of percentage or flat_amount_paise is required")
	}
	if in.Reason == "" {
		return Concession{}, errors.New("fees: reason is required")
	}
	if in.ConcessionType == ConcessionSibling && in.ElderStudentID == nil {
		return Concession{}, errors.New("fees: sibling concession requires elder_student_id")
	}
	if in.ConcessionType == ConcessionStaffWard && in.DependentStaffID == nil {
		return Concession{}, errors.New("fees: staff_ward concession requires dependent_staff_id")
	}

	var out Concession
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)
		if err := tx.QueryRow(ctx, `
			INSERT INTO concessions (school_id, student_id, concession_type, fee_head_id, percentage,
				flat_amount_paise, elder_student_id, dependent_staff_id, approver_id, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			RETURNING id, student_id, concession_type, fee_head_id, percentage, flat_amount_paise,
				elder_student_id, dependent_staff_id, status, approver_id, reason, created_at
		`, schoolID, in.StudentID, in.ConcessionType, in.FeeHeadID, in.Percentage, in.FlatAmountPaise,
			in.ElderStudentID, in.DependentStaffID, approverID, in.Reason).
			Scan(&out.ID, &out.StudentID, &out.ConcessionType, &out.FeeHeadID, &out.Percentage, &out.FlatAmountPaise,
				&out.ElderStudentID, &out.DependentStaffID, &out.Status, &out.ApproverID, &out.Reason, &out.CreatedAt); err != nil {
			return err
		}
		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: approverID, Action: "concession.create", EntityType: "concession", EntityID: out.ID,
			After:  map[string]any{"concession_type": in.ConcessionType, "student_id": in.StudentID},
			Reason: in.Reason,
		})
	})
	if err != nil {
		return Concession{}, err
	}
	return out, nil
}

func (r *Repository) ListConcessionsForStudent(ctx context.Context, studentID uuid.UUID) ([]Concession, error) {
	out := []Concession{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, student_id, concession_type, fee_head_id, percentage, flat_amount_paise,
				elder_student_id, dependent_staff_id, status, approver_id, reason, created_at
			FROM concessions WHERE student_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC
		`, studentID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Concession
			if err := rows.Scan(&c.ID, &c.StudentID, &c.ConcessionType, &c.FeeHeadID, &c.Percentage, &c.FlatAmountPaise,
				&c.ElderStudentID, &c.DependentStaffID, &c.Status, &c.ApproverID, &c.Reason, &c.CreatedAt); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

// ListReviewRequiredConcessions answers the office-facing worklist: every
// concession whose dependency has lapsed and needs an explicit decision
// (PRD 4.4.1: "a blocking warning to the office ... requiring an explicit
// decision to continue, cancel, or convert it").
func (r *Repository) ListReviewRequiredConcessions(ctx context.Context) ([]Concession, error) {
	out := []Concession{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, student_id, concession_type, fee_head_id, percentage, flat_amount_paise,
				elder_student_id, dependent_staff_id, status, approver_id, reason, created_at
			FROM concessions WHERE status = 'review_required' AND deleted_at IS NULL
			ORDER BY created_at
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Concession
			if err := rows.Scan(&c.ID, &c.StudentID, &c.ConcessionType, &c.FeeHeadID, &c.Percentage, &c.FlatAmountPaise,
				&c.ElderStudentID, &c.DependentStaffID, &c.Status, &c.ApproverID, &c.Reason, &c.CreatedAt); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

// ResolveReviewRequiredConcession is the office's explicit decision on a
// lapsed dependency (PRD 4.4.1: "continue, cancel, or convert"). "continue"
// moves it back to active as-is; "cancel" ends it; "convert" moves it to
// converted and the caller is expected to separately create a fresh
// concession of a different type/reason if that's what "convert" means for
// this family (e.g. a lapsed sibling discount becomes a merit concession) --
// this method only records the decision on the old one, it doesn't guess
// what to create in its place.
func (r *Repository) ResolveReviewRequiredConcession(ctx context.Context, concessionID uuid.UUID, decision ConcessionStatus, note string, resolvedBy uuid.UUID) error {
	if decision != ConcessionActive && decision != ConcessionCancelled && decision != ConcessionConverted {
		return errors.New("fees: decision must be active (continue), cancelled, or converted")
	}
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)
		tag, err := tx.Exec(ctx, `
			UPDATE concessions SET status = $1, resolution_note = $2, resolved_by = $3, resolved_at = now()
			WHERE id = $4 AND status = 'review_required'
		`, decision, note, resolvedBy, concessionID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrConcessionNotFound
		}
		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: resolvedBy, Action: "concession.resolve_review", EntityType: "concession", EntityID: concessionID,
			After:  map[string]any{"decision": decision},
			Reason: note,
		})
	})
}

// lapsedDependencyConcessions finds this student's active sibling/staff_ward
// concessions whose dependency has ended for academicYearID, and flips them
// to review_required in the same transaction as whatever triggered the
// check (PRD 4.4.1: "the next fee demand generation ... raises a blocking
// warning"). Returns the concessions that were just flipped, so the caller
// can report exactly which ones and why. Keyed off student_id, not
// enrollment_id, per the PRD's own explanation of why that distinction
// matters (a promoted elder sibling must not falsely trigger this).
func lapsedDependencyConcessions(ctx context.Context, tx pgx.Tx, studentID, academicYearID uuid.UUID) ([]Concession, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.id, c.student_id, c.concession_type, c.fee_head_id, c.percentage, c.flat_amount_paise,
			c.elder_student_id, c.dependent_staff_id, c.status, c.approver_id, c.reason, c.created_at
		FROM concessions c
		WHERE c.student_id = $1 AND c.status = 'active' AND c.deleted_at IS NULL
		  AND (
		    (c.concession_type = 'sibling' AND c.elder_student_id IS NOT NULL AND NOT EXISTS (
		        SELECT 1 FROM enrollments e
		        WHERE e.student_id = c.elder_student_id AND e.academic_year_id = $2
		          AND e.status = 'active' AND e.deleted_at IS NULL
		    ))
		    OR
		    (c.concession_type = 'staff_ward' AND c.dependent_staff_id IS NOT NULL AND NOT EXISTS (
		        SELECT 1 FROM staff st WHERE st.id = c.dependent_staff_id AND st.left_at IS NULL AND st.deleted_at IS NULL
		    ))
		  )
	`, studentID, academicYearID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lapsed []Concession
	for rows.Next() {
		var c Concession
		if err := rows.Scan(&c.ID, &c.StudentID, &c.ConcessionType, &c.FeeHeadID, &c.Percentage, &c.FlatAmountPaise,
			&c.ElderStudentID, &c.DependentStaffID, &c.Status, &c.ApproverID, &c.Reason, &c.CreatedAt); err != nil {
			return nil, err
		}
		lapsed = append(lapsed, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, c := range lapsed {
		if _, err := tx.Exec(ctx, `UPDATE concessions SET status = 'review_required' WHERE id = $1`, c.ID); err != nil {
			return nil, err
		}
	}
	return lapsed, nil
}
