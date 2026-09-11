package fees

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/audit"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

// AnnexureRow is one statutory-category line of PRD 4.4.7's committee
// filing: "Fee heads and amounts exportable in a format suitable for the
// state fee determination committee." Every fee head was categorized at
// definition time (PRD 4.4.1) specifically so this is a live query, not a
// manual reclassification exercise every filing cycle.
type AnnexureRow struct {
	StatutoryCategory StatutoryCategory `json:"statutory_category"`
	GrossPaise        int64             `json:"gross_paise"`
	ConcessionPaise   int64             `json:"concession_paise"`
	NetPaise          int64             `json:"net_paise"`
	CollectedPaise    int64             `json:"collected_paise"`
}

func (r *Repository) AnnexureExport(ctx context.Context, academicYearID uuid.UUID) ([]AnnexureRow, error) {
	out := []AnnexureRow{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT fh.statutory_category,
			       COALESCE(SUM(li.gross_amount_paise), 0),
			       COALESCE(SUM(li.concession_amount_paise), 0),
			       COALESCE(SUM(li.net_amount_paise), 0),
			       COALESCE(SUM(li.paid_amount_paise), 0)
			FROM fee_line_items li
			JOIN fee_heads fh ON fh.id = li.fee_head_id
			JOIN fee_assignments fa ON fa.id = li.assignment_id
			WHERE fa.academic_year_id = $1
			GROUP BY fh.statutory_category
			ORDER BY fh.statutory_category
		`, academicYearID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row AnnexureRow
			if err := rows.Scan(&row.StatutoryCategory, &row.GrossPaise, &row.ConcessionPaise, &row.NetPaise, &row.CollectedPaise); err != nil {
				return err
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}

// SetRTEStatus answers PRD 4.4: "RTE students tracked as a distinct category
// with reimbursement status." Applied to an existing assignment (not at
// creation) since RTE quota confirmation routinely lands after the year's
// demand has already been generated from the standard structure.
func (r *Repository) SetRTEStatus(ctx context.Context, assignmentID uuid.UUID, isRTE bool, reimbursementStatus string, actorID uuid.UUID) error {
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)
		tag, err := tx.Exec(ctx, `
			UPDATE fee_assignments SET is_rte = $1, rte_reimbursement_status = $2 WHERE id = $3
		`, isRTE, nullIfEmptyStr(reimbursementStatus), assignmentID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: actorID, Action: "fee_assignment.set_rte", EntityType: "fee_assignment", EntityID: assignmentID,
			After: map[string]any{"is_rte": isRTE, "rte_reimbursement_status": reimbursementStatus},
		})
	})
}

type RTEAssignmentRow struct {
	AssignmentID           uuid.UUID `json:"assignment_id"`
	StudentID              uuid.UUID `json:"student_id"`
	StudentName            string    `json:"student_name"`
	RTEReimbursementStatus string    `json:"rte_reimbursement_status,omitempty"`
}

func (r *Repository) ListRTEAssignments(ctx context.Context, academicYearID uuid.UUID) ([]RTEAssignmentRow, error) {
	out := []RTEAssignmentRow{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT fa.id, fa.student_id, s.name_english, COALESCE(fa.rte_reimbursement_status, '')
			FROM fee_assignments fa
			JOIN students s ON s.id = fa.student_id
			WHERE fa.academic_year_id = $1 AND fa.is_rte = true
			ORDER BY s.name_english
		`, academicYearID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row RTEAssignmentRow
			if err := rows.Scan(&row.AssignmentID, &row.StudentID, &row.StudentName, &row.RTEReimbursementStatus); err != nil {
				return err
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}
