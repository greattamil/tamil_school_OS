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

var ErrInsufficientCredit = errors.New("fees: refund amount exceeds the student's credit balance")

// RefundPayment answers PRD 4.4.4: "Refund creates a negative allocation,
// never deletes the original payment" and PRD 4.4.3.3: "Credit is
// refundable, subject to the same authority and audit rules as any refund."
// A refund always comes out of the student's credit balance -- an
// overpayment or advance both land there the moment they're recorded (see
// CollectPayment), so "refund this payment's overpayment" and "refund from
// accumulated credit" are the same operation; paymentID is kept only as an
// optional reference for which receipt the parent is asking about.
func (r *Repository) RefundPayment(ctx context.Context, paymentID *uuid.UUID, studentID uuid.UUID, amountPaise int64, reason string, approvedBy, createdBy uuid.UUID) (uuid.UUID, error) {
	if amountPaise <= 0 {
		return uuid.Nil, errors.New("fees: refund amount must be positive")
	}
	if reason == "" {
		return uuid.Nil, errors.New("fees: refund reason is required")
	}

	var refundID uuid.UUID
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var balance int64
		if err := tx.QueryRow(ctx, `SELECT balance_paise FROM student_credit_balances WHERE student_id = $1 FOR UPDATE`, studentID).Scan(&balance); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				balance = 0
			} else {
				return err
			}
		}
		if amountPaise > balance {
			return fmt.Errorf("%w: balance is %d, requested %d", ErrInsufficientCredit, balance, amountPaise)
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO refunds (school_id, payment_id, student_id, amount_paise, reason, approved_by, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id
		`, schoolID, paymentID, studentID, amountPaise, reason, approvedBy, createdBy).Scan(&refundID); err != nil {
			return err
		}

		if err := creditStudent(ctx, tx, schoolID, studentID, -amountPaise, CreditRefund, paymentID, nil, createdBy); err != nil {
			return err
		}

		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: createdBy, Action: "payment.refund", EntityType: "refund", EntityID: refundID,
			After:  map[string]any{"student_id": studentID, "amount_paise": amountPaise, "approved_by": approvedBy},
			Reason: reason,
		})
	})
	if err != nil {
		return uuid.Nil, err
	}
	return refundID, nil
}
