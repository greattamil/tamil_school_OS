package fees

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/db"
)

// Dues answers PRD 4.4.5: "Real-time outstanding dues per student" and the
// ageing report ("dues by days overdue"), computed live from fee_line_items
// and student_credit_balances -- never a cached total, since a void or a
// bounced cheque must be reflected the instant it happens, not on the next
// batch run.
func (r *Repository) Dues(ctx context.Context) ([]DuesRow, error) {
	out := []DuesRow{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.name_english,
			       COALESCE(SUM(li.net_amount_paise - li.paid_amount_paise), 0) AS outstanding,
			       COALESCE(scb.balance_paise, 0) AS credit,
			       MIN(li.due_date) FILTER (WHERE li.net_amount_paise > li.paid_amount_paise) AS oldest_due
			FROM students s
			JOIN fee_line_items li ON li.student_id = s.id AND li.status <> 'waived'
			LEFT JOIN student_credit_balances scb ON scb.student_id = s.id
			WHERE s.deleted_at IS NULL
			GROUP BY s.id, s.name_english, scb.balance_paise
			HAVING COALESCE(SUM(li.net_amount_paise - li.paid_amount_paise), 0) > 0
			ORDER BY outstanding DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		today := time.Now()
		for rows.Next() {
			var d DuesRow
			if err := rows.Scan(&d.StudentID, &d.StudentName, &d.OutstandingPaise, &d.CreditBalancePaise, &d.OldestDueDate); err != nil {
				return err
			}
			if d.OldestDueDate != nil && d.OldestDueDate.Before(today) {
				d.DaysOverdue = int(today.Sub(*d.OldestDueDate).Hours() / 24)
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

// LineItemsForStudent is the itemized breakdown behind a student's dues
// figure -- what the office (and, via a thinner projection, the parent app)
// actually needs to show why a total is what it is.
func (r *Repository) LineItemsForStudent(ctx context.Context, studentID uuid.UUID, academicYearID *uuid.UUID) ([]LineItem, error) {
	out := []LineItem{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT li.id, li.assignment_id, li.student_id, li.fee_head_id, fh.name, li.label, li.due_date,
			       li.gross_amount_paise, li.concession_amount_paise, li.net_amount_paise, li.paid_amount_paise, li.status
			FROM fee_line_items li
			JOIN fee_heads fh ON fh.id = li.fee_head_id
			JOIN fee_assignments fa ON fa.id = li.assignment_id
			WHERE li.student_id = $1 AND ($2::uuid IS NULL OR fa.academic_year_id = $2)
			ORDER BY li.due_date
		`, studentID, academicYearID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var li LineItem
			if err := rows.Scan(&li.ID, &li.AssignmentID, &li.StudentID, &li.FeeHeadID, &li.FeeHeadName, &li.Label, &li.DueDate,
				&li.GrossAmountPaise, &li.ConcessionAmountPaise, &li.NetAmountPaise, &li.PaidAmountPaise, &li.Status); err != nil {
				return err
			}
			out = append(out, li)
		}
		return rows.Err()
	})
	return out, err
}

// RealDuesForStudent is the Phase 3 ledger's answer to the same question the
// Phase 2 fee_dues_snapshot import answers (PRD 4.8: parent dashboard's
// "outstanding dues with due date") -- computed live from fee_line_items
// instead of a manually re-imported snapshot. found is false when the
// student has no fee_assignments at all yet (this school/student hasn't
// been moved onto the real fee engine), which is the caller's signal to fall
// back to the snapshot rather than showing a false "nothing owed".
func (r *Repository) RealDuesForStudent(ctx context.Context, studentID uuid.UUID) (outstandingPaise, creditPaise int64, oldestDueDate *time.Time, found bool, err error) {
	err = db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fee_assignments WHERE student_id = $1)`, studentID).Scan(&found); err != nil {
			return err
		}
		if !found {
			return nil
		}
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(net_amount_paise - paid_amount_paise), 0),
			       MIN(due_date) FILTER (WHERE net_amount_paise > paid_amount_paise)
			FROM fee_line_items WHERE student_id = $1 AND status <> 'waived'
		`, studentID).Scan(&outstandingPaise, &oldestDueDate); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT balance_paise FROM student_credit_balances WHERE student_id = $1`, studentID).Scan(&creditPaise)
		if err == pgx.ErrNoRows {
			creditPaise = 0
			return nil
		}
		return err
	})
	return outstandingPaise, creditPaise, oldestDueDate, found, err
}

// CreditBalance returns the student's current unallocated credit (PRD
// 4.4.3.3: "appears in the dues report as a distinct column, never netted
// silently into a student's outstanding figure").
func (r *Repository) CreditBalance(ctx context.Context, studentID uuid.UUID) (int64, error) {
	var balance int64
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT balance_paise FROM student_credit_balances WHERE student_id = $1`, studentID).Scan(&balance)
		if err == pgx.ErrNoRows {
			balance = 0
			return nil
		}
		return err
	})
	return balance, err
}
