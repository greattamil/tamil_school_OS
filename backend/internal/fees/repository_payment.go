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
	ErrPaymentNotFound          = errors.New("fees: payment not found")
	ErrDrawerClosed             = errors.New("fees: this payment's business day is closed; corrections must be a new dated entry")
	ErrAlreadyVoid              = errors.New("fees: payment is already void")
	ErrInsufficientLineItemRoom = errors.New("fees: allocation exceeds a line item's outstanding amount")
)

// CollectPayment answers PRD 4.4.3: "Offline collection is first-class. Cash
// and cheque entry with receipt generation is the primary flow." Allocates
// across the caller-specified line items (PRD 4.4.3.2: "with partial payment
// support"), holds any unallocated remainder as credit (PRD 4.4.3.3), and
// assigns the next receipt number atomically so two concurrent collectors
// can never receive the same one.
func (r *Repository) CollectPayment(ctx context.Context, in CollectPaymentInput, collectedBy uuid.UUID) (Payment, error) {
	var allocatedSum int64
	for _, a := range in.Allocations {
		if a.AmountPaise <= 0 {
			return Payment{}, errors.New("fees: allocation amounts must be positive")
		}
		allocatedSum += a.AmountPaise
	}
	if allocatedSum+in.AdvanceAmountPaise != in.AmountPaise {
		return Payment{}, fmt.Errorf("fees: allocations (%d) + advance (%d) must equal amount_paise (%d)", allocatedSum, in.AdvanceAmountPaise, in.AmountPaise)
	}
	if in.AmountPaise <= 0 {
		return Payment{}, errors.New("fees: amount_paise must be positive")
	}

	var out Payment
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var receiptNumber int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO fee_receipt_sequences (school_id, next_number) VALUES ($1, 2)
			ON CONFLICT (school_id) DO UPDATE SET next_number = fee_receipt_sequences.next_number + 1
			RETURNING next_number - 1
		`, schoolID).Scan(&receiptNumber); err != nil {
			return err
		}

		var chequeStatus *ChequeStatus
		if in.Mode == ModeCheque {
			s := ChequeReceived
			chequeStatus = &s
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO payments (school_id, student_id, receipt_number, mode, amount_paise,
				allocated_amount_paise, advance_amount_paise, collected_by, cheque_number, cheque_bank,
				cheque_status, device_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			RETURNING id, student_id, receipt_number, mode, amount_paise, allocated_amount_paise,
				advance_amount_paise, collected_by, collected_at, is_void
		`, schoolID, in.StudentID, receiptNumber, in.Mode, in.AmountPaise, allocatedSum, in.AdvanceAmountPaise,
			collectedBy, nullIfEmptyStr(in.ChequeNumber), nullIfEmptyStr(in.ChequeBank), chequeStatus, nullIfEmptyStr(in.DeviceID)).
			Scan(&out.ID, &out.StudentID, &out.ReceiptNumber, &out.Mode, &out.AmountPaise, &out.AllocatedAmountPaise,
				&out.AdvanceAmountPaise, &out.CollectedBy, &out.CollectedAt, &out.IsVoid); err != nil {
			return err
		}
		if chequeStatus != nil {
			out.ChequeStatus = *chequeStatus
			out.ChequeNumber = in.ChequeNumber
			out.ChequeBank = in.ChequeBank
		}

		for _, a := range in.Allocations {
			var netAmount, paidAmount int64
			var lineItemStudent uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT net_amount_paise, paid_amount_paise, student_id FROM fee_line_items WHERE id = $1 FOR UPDATE
			`, a.LineItemID).Scan(&netAmount, &paidAmount, &lineItemStudent); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			if lineItemStudent != in.StudentID {
				return fmt.Errorf("fees: line item %s does not belong to student %s", a.LineItemID, in.StudentID)
			}
			if paidAmount+a.AmountPaise > netAmount {
				return ErrInsufficientLineItemRoom
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO payment_allocations (school_id, payment_id, line_item_id, amount_paise)
				VALUES ($1,$2,$3,$4)
			`, schoolID, out.ID, a.LineItemID, a.AmountPaise); err != nil {
				return err
			}
			newPaid := paidAmount + a.AmountPaise
			status := LineItemPending
			switch {
			case newPaid >= netAmount:
				status = LineItemPaid
			case newPaid > 0:
				status = LineItemPartiallyPaid
			}
			if _, err := tx.Exec(ctx, `
				UPDATE fee_line_items SET paid_amount_paise = $1, status = $2, updated_at = now() WHERE id = $3
			`, newPaid, status, a.LineItemID); err != nil {
				return err
			}
		}

		if in.AdvanceAmountPaise > 0 {
			if err := creditStudent(ctx, tx, schoolID, in.StudentID, in.AdvanceAmountPaise, CreditOverpayment, &out.ID, nil, collectedBy); err != nil {
				return err
			}
		}

		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: collectedBy, Action: "payment.collect", EntityType: "payment", EntityID: out.ID,
			After: map[string]any{
				"student_id": in.StudentID, "mode": in.Mode, "amount_paise": in.AmountPaise,
				"receipt_number": receiptNumber,
			},
		})
	})
	if err != nil {
		return Payment{}, err
	}
	return out, nil
}

type CreditReason string

const (
	CreditOverpayment     CreditReason = "overpayment"
	CreditAppliedToDemand CreditReason = "applied_to_demand"
	CreditRefund          CreditReason = "refund"
	CreditManualAdjust    CreditReason = "manual_adjustment"
)

// creditStudent applies deltaPaise (positive credits, negative debits) to the
// student's running credit balance and appends the ledger entry that
// justifies it -- PRD 4.4.3.3: "A student_credit_balances ledger holds
// unallocated amounts as a first-class balance." Must run inside an existing
// tenant transaction (it does not open its own), since it's always one step
// of a larger operation (a payment, a refund, a demand generation).
func creditStudent(ctx context.Context, tx pgx.Tx, schoolID, studentID uuid.UUID, deltaPaise int64, reason CreditReason, paymentID, lineItemID *uuid.UUID, actorID uuid.UUID) error {
	// GREATEST(0, $3), not plain $3, for the INSERT branch's candidate value:
	// Postgres validates CHECK constraints against the VALUES-list row that
	// ON CONFLICT DO UPDATE constructs as its conflict candidate, even when a
	// negative deltaPaise would only ever be applied via the (perfectly
	// valid) UPDATE branch against an existing, larger balance -- confirmed
	// directly against this exact statement: a plain negative $3 aborted with
	// a check-constraint violation on the candidate row even though the row
	// being updated existed and had ample balance. A brand-new student
	// starting from a negative delta with no existing row is a bug
	// elsewhere (there is nothing to debit yet), so clamping that candidate
	// to zero here is correct, not just a workaround for Postgres's
	// evaluation order.
	if _, err := tx.Exec(ctx, `
		INSERT INTO student_credit_balances (school_id, student_id, balance_paise, updated_at)
		VALUES ($1, $2, GREATEST(0, $3), now())
		ON CONFLICT (school_id, student_id) DO UPDATE
		SET balance_paise = student_credit_balances.balance_paise + $3, updated_at = now()
	`, schoolID, studentID, deltaPaise); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO credit_balance_transactions (school_id, student_id, delta_paise, reason, payment_id, line_item_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, schoolID, studentID, deltaPaise, reason, paymentID, lineItemID, actorID)
	return err
}

// VoidPayment answers PRD 4.4.4: "Void requires correspondent-level
// authority and a mandatory reason" (authority checked by the caller/handler
// against the actor's role -- this method enforces the mandatory reason and
// the immutability rules) and PRD 4.4.6.1: "Closing freezes that day's cash
// batch. No further cash entry, no voiding." The payment row and its
// receipt_number are never deleted or reused (PRD 4.4.4); voiding reverses
// its effect on line items and any credit it created.
func (r *Repository) VoidPayment(ctx context.Context, paymentID uuid.UUID, reason string, voidedBy uuid.UUID) error {
	if reason == "" {
		return errors.New("fees: void reason is required")
	}
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var studentID uuid.UUID
		var isVoid bool
		var advanceAmount int64
		var drawerClosingID *uuid.UUID
		var chequeStatus *ChequeStatus
		if err := tx.QueryRow(ctx, `
			SELECT student_id, is_void, advance_amount_paise, cash_drawer_closing_id, cheque_status
			FROM payments WHERE id = $1 FOR UPDATE
		`, paymentID).Scan(&studentID, &isVoid, &advanceAmount, &drawerClosingID, &chequeStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrPaymentNotFound
			}
			return err
		}
		if isVoid {
			return ErrAlreadyVoid
		}
		if drawerClosingID != nil {
			return ErrDrawerClosed
		}

		// A bounced cheque already reversed its own effect on line items and
		// any advance credit (MarkChequeStatus) -- reversing it again here
		// would double-subtract paid_amount_paise below zero. Voiding an
		// already-bounced payment is purely the administrative/audit act of
		// removing it from active reports; there is nothing left to reinstate.
		alreadyReversed := chequeStatus != nil && *chequeStatus == ChequeBounced

		if !alreadyReversed {
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
				switch {
				case newPaid >= netAmount && netAmount > 0:
					status = LineItemPaid
				case newPaid > 0:
					status = LineItemPartiallyPaid
				}
				if _, err := tx.Exec(ctx, `UPDATE fee_line_items SET paid_amount_paise = $1, status = $2, updated_at = now() WHERE id = $3`, newPaid, status, a.lineItemID); err != nil {
					return err
				}
			}

			if advanceAmount > 0 {
				if err := creditStudent(ctx, tx, schoolID, studentID, -advanceAmount, CreditManualAdjust, &paymentID, nil, voidedBy); err != nil {
					return err
				}
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE payments SET is_void = true, void_reason = $1, voided_by = $2, voided_at = now() WHERE id = $3
		`, reason, voidedBy, paymentID); err != nil {
			return err
		}

		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: voidedBy, Action: "payment.void", EntityType: "payment", EntityID: paymentID,
			Reason: reason,
		})
	})
}

func nullIfEmptyStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
