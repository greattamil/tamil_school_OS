package fees

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/audit"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrNotACheque              = errors.New("fees: payment is not a cheque")
	ErrInvalidChequeTransition = errors.New("fees: invalid cheque status transition")
)

var validChequeTransitions = map[ChequeStatus][]ChequeStatus{
	ChequeReceived:  {ChequeDeposited, ChequeBounced},
	ChequeDeposited: {ChequeCleared, ChequeBounced},
}

// MarkChequeStatus answers PRD 4.4.3.3: "Cheque lifecycle: received,
// deposited, cleared, bounced. A bounced cheque reinstates the dues and
// notifies the office." Reinstating dues reuses the exact same reversal as
// VoidPayment (payment_allocations rolled back, line items recomputed), but
// the payment row itself is NOT marked void -- a bounced cheque is a
// distinct, visible outcome in its own right, not indistinguishable from a
// clerical void. addReturnCharge answers PRD 4.4.3.3's "the system offers to
// add a configurable cheque return charge line item ... optional per
// instance". Notifying the office (the PRD's own next clause) is the
// caller's job via the notify package, not this repository's.
func (r *Repository) MarkChequeStatus(ctx context.Context, paymentID uuid.UUID, newStatus ChequeStatus, addReturnCharge bool, waiverNote string, actorID uuid.UUID) error {
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var studentID uuid.UUID
		var mode PaymentMode
		var currentStatus *ChequeStatus
		var isVoid bool
		if err := tx.QueryRow(ctx, `
			SELECT student_id, mode, cheque_status, is_void FROM payments WHERE id = $1 FOR UPDATE
		`, paymentID).Scan(&studentID, &mode, &currentStatus, &isVoid); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrPaymentNotFound
			}
			return err
		}
		if mode != ModeCheque || currentStatus == nil {
			return ErrNotACheque
		}
		if isVoid {
			return ErrAlreadyVoid
		}
		allowed := validChequeTransitions[*currentStatus]
		ok := false
		for _, s := range allowed {
			if s == newStatus {
				ok = true
				break
			}
		}
		if !ok {
			return ErrInvalidChequeTransition
		}

		if _, err := tx.Exec(ctx, `UPDATE payments SET cheque_status = $1 WHERE id = $2`, newStatus, paymentID); err != nil {
			return err
		}

		if newStatus == ChequeBounced {
			rows, err := tx.Query(ctx, `SELECT line_item_id, amount_paise FROM payment_allocations WHERE payment_id = $1`, paymentID)
			if err != nil {
				return err
			}
			type alloc struct {
				lineItemID uuid.UUID
				amount     int64
			}
			var allocs []alloc
			for rows.Next() {
				var a alloc
				if err := rows.Scan(&a.lineItemID, &a.amount); err != nil {
					rows.Close()
					return err
				}
				allocs = append(allocs, a)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, a := range allocs {
				var netAmount, paidAmount int64
				if err := tx.QueryRow(ctx, `SELECT net_amount_paise, paid_amount_paise FROM fee_line_items WHERE id = $1 FOR UPDATE`, a.lineItemID).Scan(&netAmount, &paidAmount); err != nil {
					return err
				}
				newPaid := paidAmount - a.amount
				status := LineItemPending
				if newPaid >= netAmount && netAmount > 0 {
					status = LineItemPaid
				} else if newPaid > 0 {
					status = LineItemPartiallyPaid
				}
				if _, err := tx.Exec(ctx, `UPDATE fee_line_items SET paid_amount_paise = $1, status = $2, updated_at = now() WHERE id = $3`, newPaid, status, a.lineItemID); err != nil {
					return err
				}
			}

			var advanceAmount int64
			if err := tx.QueryRow(ctx, `SELECT advance_amount_paise FROM payments WHERE id = $1`, paymentID).Scan(&advanceAmount); err != nil {
				return err
			}
			if advanceAmount > 0 {
				if err := creditStudent(ctx, tx, schoolID, studentID, -advanceAmount, CreditManualAdjust, &paymentID, nil, actorID); err != nil {
					return err
				}
			}

			reason := "cheque bounced"
			if addReturnCharge {
				var chargeHeadID uuid.UUID
				var chargeAmount int64
				var academicYearID uuid.UUID
				if err := tx.QueryRow(ctx, `
					SELECT fa.academic_year_id FROM fee_assignments fa
					JOIN fee_line_items li ON li.assignment_id = fa.id
					JOIN payment_allocations pa ON pa.line_item_id = li.id
					WHERE pa.payment_id = $1 LIMIT 1
				`, paymentID).Scan(&academicYearID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				if academicYearID != uuid.Nil {
					if err := tx.QueryRow(ctx, `SELECT cheque_return_charge_paise FROM fee_settings WHERE school_id = $1`, schoolID).Scan(&chargeAmount); err != nil {
						if errors.Is(err, pgx.ErrNoRows) {
							chargeAmount = 25000
						} else {
							return err
						}
					}
					if err := tx.QueryRow(ctx, `
						INSERT INTO fee_heads (school_id, academic_year_id, name, statutory_category)
						VALUES ($1, $2, 'Cheque Return Charge', 'other')
						ON CONFLICT (school_id, academic_year_id, name) DO UPDATE SET name = EXCLUDED.name
						RETURNING id
					`, schoolID, academicYearID).Scan(&chargeHeadID); err != nil {
						return err
					}
					var assignmentID uuid.UUID
					if err := tx.QueryRow(ctx, `SELECT id FROM fee_assignments WHERE student_id = $1 AND academic_year_id = $2`, studentID, academicYearID).Scan(&assignmentID); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, `
						INSERT INTO fee_line_items (school_id, assignment_id, student_id, fee_head_id, label,
							due_date, gross_amount_paise, net_amount_paise, override_reason, override_approver_id)
						VALUES ($1,$2,$3,$4,'Cheque return charge', CURRENT_DATE, $5, $5, $6, $7)
					`, schoolID, assignmentID, studentID, chargeHeadID, chargeAmount,
						fmt.Sprintf("cheque %s bounced", paymentID), actorID); err != nil {
						return err
					}
					reason = "cheque bounced, return charge applied"
				}
			} else if waiverNote != "" {
				reason = "cheque bounced, return charge waived: " + waiverNote
			}

			if err := audit.Write(ctx, tx, schoolID, audit.Entry{
				ActorUserID: actorID, Action: "payment.cheque_bounced", EntityType: "payment", EntityID: paymentID,
				Reason: reason,
			}); err != nil {
				return err
			}
		}

		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: actorID, Action: "payment.cheque_status", EntityType: "payment", EntityID: paymentID,
			After: map[string]any{"cheque_status": newStatus},
		})
	})
}
