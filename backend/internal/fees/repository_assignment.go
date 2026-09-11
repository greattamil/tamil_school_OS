package fees

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var ErrAssignmentExists = errors.New("fees: assignment already exists for this student and year")

// ConcessionReviewError is returned instead of generating an assignment when
// one or more of the student's concessions just lapsed a dependency check
// (PRD 4.4.1: "a blocking warning to the office ... requiring an explicit
// decision"). The office must resolve every listed concession via
// ResolveReviewRequiredConcession before retrying.
type ConcessionReviewError struct {
	Concessions []Concession
}

func (e *ConcessionReviewError) Error() string {
	return fmt.Sprintf("fees: %d concession(s) need office review before this demand can be generated", len(e.Concessions))
}

// GenerateAssignment answers PRD 4.4.2: "On enrollment, applicable heads are
// assigned to the student, producing dated line items." Resolves the
// student's class for academicYearID, finds that class's active fee
// structure, applies every currently-active concession to each instalment,
// and writes the resulting fee_line_items. Blocks (ConcessionReviewError)
// rather than silently continuing or silently dropping a lapsed sibling/
// staff-ward concession -- see lapsedDependencyConcessions.
func (r *Repository) GenerateAssignment(ctx context.Context, studentID, academicYearID uuid.UUID, createdBy uuid.UUID) (FeeAssignment, []LineItem, error) {
	var alreadyExists bool
	if err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fee_assignments WHERE student_id = $1 AND academic_year_id = $2)`, studentID, academicYearID).Scan(&alreadyExists)
	}); err != nil {
		return FeeAssignment{}, nil, err
	}
	if alreadyExists {
		return FeeAssignment{}, nil, ErrAssignmentExists
	}

	// Run as its own committed transaction, separate from the assignment
	// generation below: lapsedDependencyConcessions's status flip to
	// review_required must survive even though (especially because)
	// generation itself is about to abort -- if this ran inside the same
	// transaction as the generation attempt, the abort would roll the flip
	// back too, leaving the office's review worklist silently empty despite
	// the office having just been told a concession needs their attention.
	var lapsed []Concession
	if err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		lapsed, err = lapsedDependencyConcessions(ctx, tx, studentID, academicYearID)
		return err
	}); err != nil {
		return FeeAssignment{}, nil, err
	}
	if len(lapsed) > 0 {
		return FeeAssignment{}, nil, &ConcessionReviewError{Concessions: lapsed}
	}

	var assignment FeeAssignment
	var lineItems []LineItem

	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fee_assignments WHERE student_id = $1 AND academic_year_id = $2)`, studentID, academicYearID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrAssignmentExists
		}

		var classID uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT class_id FROM enrollments
			WHERE student_id = $1 AND academic_year_id = $2 AND deleted_at IS NULL
			ORDER BY lower(period) DESC LIMIT 1
		`, studentID, academicYearID).Scan(&classID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("fees: student has no enrollment for this academic year")
			}
			return err
		}

		var structureID uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT id FROM fee_structures WHERE academic_year_id = $1 AND class_id = $2 AND is_active
		`, academicYearID, classID).Scan(&structureID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNoActiveStructure
			}
			return err
		}

		rows, err := tx.Query(ctx, `
			SELECT id, fee_head_id, label, amount_paise, due_date
			FROM fee_structure_instalments WHERE structure_id = $1
			ORDER BY due_date
		`, structureID)
		if err != nil {
			return err
		}
		type instalmentRow struct {
			feeHeadID uuid.UUID
			label     string
			amount    int64
			dueDate   time.Time
		}
		var instalments []instalmentRow
		for rows.Next() {
			var i instalmentRow
			var id uuid.UUID
			if err := rows.Scan(&id, &i.feeHeadID, &i.label, &i.amount, &i.dueDate); err != nil {
				rows.Close()
				return err
			}
			instalments = append(instalments, i)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		concRows, err := tx.Query(ctx, `
			SELECT id, fee_head_id, percentage, flat_amount_paise
			FROM concessions WHERE student_id = $1 AND status = 'active' AND deleted_at IS NULL
		`, studentID)
		if err != nil {
			return err
		}
		type concessionRow struct {
			id         uuid.UUID
			feeHeadID  *uuid.UUID
			percentage *float64
			flatRemain int64
			isFlat     bool
		}
		var concessions []concessionRow
		for concRows.Next() {
			var c concessionRow
			var flat *int64
			if err := concRows.Scan(&c.id, &c.feeHeadID, &c.percentage, &flat); err != nil {
				concRows.Close()
				return err
			}
			if flat != nil {
				c.isFlat = true
				c.flatRemain = *flat
			}
			concessions = append(concessions, c)
		}
		concRows.Close()
		if err := concRows.Err(); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO fee_assignments (school_id, student_id, academic_year_id, structure_id, created_by)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id, student_id, academic_year_id, structure_id, is_rte, created_at
		`, schoolID, studentID, academicYearID, structureID, createdBy).
			Scan(&assignment.ID, &assignment.StudentID, &assignment.AcademicYearID, &assignment.StructureID, &assignment.IsRTE, &assignment.CreatedAt); err != nil {
			return err
		}

		for _, ins := range instalments {
			gross := ins.amount
			remaining := gross
			var totalConcession int64
			for i := range concessions {
				c := &concessions[i]
				if c.feeHeadID != nil && *c.feeHeadID != ins.feeHeadID {
					continue
				}
				var amt int64
				if c.percentage != nil {
					amt = int64(math.Round(float64(gross) * *c.percentage / 100))
				} else {
					amt = c.flatRemain
				}
				if amt > remaining {
					amt = remaining
				}
				if amt < 0 {
					amt = 0
				}
				if c.isFlat {
					c.flatRemain -= amt
				}
				totalConcession += amt
				remaining -= amt
			}

			var li LineItem
			if err := tx.QueryRow(ctx, `
				INSERT INTO fee_line_items (school_id, assignment_id, student_id, fee_head_id, label,
					due_date, gross_amount_paise, concession_amount_paise, net_amount_paise)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
				RETURNING id, assignment_id, student_id, fee_head_id, label, due_date,
					gross_amount_paise, concession_amount_paise, net_amount_paise, paid_amount_paise, status
			`, schoolID, assignment.ID, studentID, ins.feeHeadID, ins.label, ins.dueDate,
				gross, totalConcession, remaining).
				Scan(&li.ID, &li.AssignmentID, &li.StudentID, &li.FeeHeadID, &li.Label, &li.DueDate,
					&li.GrossAmountPaise, &li.ConcessionAmountPaise, &li.NetAmountPaise, &li.PaidAmountPaise, &li.Status); err != nil {
				return err
			}
			lineItems = append(lineItems, li)
		}

		return nil
	})
	if err != nil {
		return FeeAssignment{}, nil, err
	}
	return assignment, lineItems, nil
}
