package fees

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/audit"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrUPINotConfigured       = errors.New("fees: this school's UPI VPA is not configured yet")
	ErrUPIRequestNotFound     = errors.New("fees: UPI payment request not found")
	ErrUPIRequestNotPending   = errors.New("fees: this UPI request is not awaiting a UTR")
	ErrUPIRequestNotSubmitted = errors.New("fees: this UPI request has no submitted UTR to verify")
)

type UPIRequestStatus string

const (
	UPIPending      UPIRequestStatus = "pending"
	UPIUTRSubmitted UPIRequestStatus = "utr_submitted"
	UPIVerified     UPIRequestStatus = "verified"
	UPIRejected     UPIRequestStatus = "rejected"
)

type UPIPaymentRequest struct {
	ID                uuid.UUID        `json:"id"`
	StudentID         uuid.UUID        `json:"student_id"`
	Reference         string           `json:"reference"`
	AmountPaise       int64            `json:"amount_paise"`
	IntentLink        string           `json:"intent_link"`
	Status            UPIRequestStatus `json:"status"`
	SubmittedUTR      string           `json:"submitted_utr,omitempty"`
	VerifiedPaymentID *uuid.UUID       `json:"verified_payment_id,omitempty"`
	RejectionReason   string           `json:"rejection_reason,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
}

// CreateUPIRequest answers PRD 4.4.3.1's honest fallback: generate the
// upi://pay link with a unique reference and declared allocations up front,
// so verification later only has to confirm the money arrived, not re-decide
// the split. Refuses outright if the school hasn't configured its own VPA
// (fee_settings.upi_vpa) rather than emitting a link to nowhere.
func (r *Repository) CreateUPIRequest(ctx context.Context, studentID uuid.UUID, allocations []AllocationInput, note string, createdBy uuid.UUID) (UPIPaymentRequest, error) {
	var amountPaise int64
	for _, a := range allocations {
		if a.AmountPaise <= 0 {
			return UPIPaymentRequest{}, errors.New("fees: allocation amounts must be positive")
		}
		amountPaise += a.AmountPaise
	}
	if amountPaise <= 0 {
		return UPIPaymentRequest{}, errors.New("fees: at least one allocation is required")
	}

	var out UPIPaymentRequest
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var vpa, payeeName *string
		if err := tx.QueryRow(ctx, `SELECT upi_vpa, upi_payee_name FROM fee_settings WHERE school_id = $1`, schoolID).Scan(&vpa, &payeeName); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if vpa == nil || *vpa == "" {
			return ErrUPINotConfigured
		}
		name := "School"
		if payeeName != nil && *payeeName != "" {
			name = *payeeName
		}

		reference, err := randomReference()
		if err != nil {
			return err
		}
		link := UPIIntentLink(*vpa, name, amountPaise, reference, note)

		if err := tx.QueryRow(ctx, `
			INSERT INTO upi_payment_requests (school_id, student_id, reference, amount_paise, intent_link, created_by)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, student_id, reference, amount_paise, intent_link, status, created_at
		`, schoolID, studentID, reference, amountPaise, link, createdBy).
			Scan(&out.ID, &out.StudentID, &out.Reference, &out.AmountPaise, &out.IntentLink, &out.Status, &out.CreatedAt); err != nil {
			return err
		}

		for _, a := range allocations {
			if _, err := tx.Exec(ctx, `
				INSERT INTO upi_payment_request_allocations (school_id, request_id, line_item_id, amount_paise)
				VALUES ($1,$2,$3,$4)
			`, schoolID, out.ID, a.LineItemID, a.AmountPaise); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return UPIPaymentRequest{}, err
	}
	return out, nil
}

func randomReference() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "tsos" + hex.EncodeToString(b), nil
}

// SubmitUTR answers PRD 4.4.3.1's "parent-submitted UTR" step. Callable by
// office staff on the parent's behalf, or (once the parent app grows this
// screen) the parent themselves -- the repository doesn't distinguish, since
// submitting a UTR asserts nothing on its own; VerifyUPIRequest below is
// where an actual authority check matters.
func (r *Repository) SubmitUTR(ctx context.Context, requestID uuid.UUID, utr string) error {
	if utr == "" {
		return errors.New("fees: utr is required")
	}
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE upi_payment_requests SET status = 'utr_submitted', submitted_utr = $1, submitted_at = now()
			WHERE id = $2 AND status = 'pending'
		`, utr, requestID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM upi_payment_requests WHERE id = $1)`, requestID).Scan(&exists)
			if !exists {
				return ErrUPIRequestNotFound
			}
			return ErrUPIRequestNotPending
		}
		return nil
	})
}

// VerifyUPIRequest is the office's "verified against the bank statement"
// action (PRD 4.4.3.1) -- turns the declared allocations into a real
// CollectPayment call (mode=upi) at the moment, and only the moment, a human
// has actually confirmed the money arrived. This is not automatic
// reconciliation; it is the honest, manual alternative PRD 4.4.3.1 names.
func (r *Repository) VerifyUPIRequest(ctx context.Context, requestID uuid.UUID, verifiedBy uuid.UUID) (Payment, error) {
	var studentID uuid.UUID
	var amountPaise int64
	var status UPIRequestStatus
	var allocations []AllocationInput

	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT student_id, amount_paise, status FROM upi_payment_requests WHERE id = $1`, requestID).
			Scan(&studentID, &amountPaise, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUPIRequestNotFound
			}
			return err
		}
		if status != UPIUTRSubmitted {
			return ErrUPIRequestNotSubmitted
		}
		rows, err := tx.Query(ctx, `SELECT line_item_id, amount_paise FROM upi_payment_request_allocations WHERE request_id = $1`, requestID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a AllocationInput
			if err := rows.Scan(&a.LineItemID, &a.AmountPaise); err != nil {
				return err
			}
			allocations = append(allocations, a)
		}
		return rows.Err()
	})
	if err != nil {
		return Payment{}, err
	}

	// Two separate transactions, not one: CollectPayment owns its own
	// (receipt-sequence locking, allocation, credit-balance updates -- real
	// work worth keeping as one well-tested unit rather than duplicating
	// inline). The narrow gap this leaves: if the payment commits but the
	// process dies before the second transaction below marks this request
	// verified, the money is correctly recorded (the payment is real and
	// correct) but this request row would be stuck showing utr_submitted
	// forever instead of pointing at it. Acceptable for a manual,
	// human-in-the-loop verification step -- the office would notice a
	// request that never leaves the pending worklist despite a matching
	// receipt existing -- but worth stating plainly rather than presenting
	// this as atomic when it isn't.
	payment, err := r.CollectPayment(ctx, CollectPaymentInput{
		StudentID: studentID, Mode: ModeUPI, AmountPaise: amountPaise,
		Allocations: allocations, AdvanceAmountPaise: 0,
	}, verifiedBy)
	if err != nil {
		return Payment{}, err
	}

	err = db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)
		if _, err := tx.Exec(ctx, `
			UPDATE upi_payment_requests SET status = 'verified', verified_payment_id = $1, verified_by = $2, verified_at = now()
			WHERE id = $3
		`, payment.ID, verifiedBy, requestID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: verifiedBy, Action: "upi_request.verify", EntityType: "upi_payment_request", EntityID: requestID,
			After: map[string]any{"payment_id": payment.ID},
		})
	})
	if err != nil {
		return Payment{}, err
	}
	return payment, nil
}

func (r *Repository) RejectUPIRequest(ctx context.Context, requestID uuid.UUID, reason string, rejectedBy uuid.UUID) error {
	if reason == "" {
		return errors.New("fees: rejection reason is required")
	}
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)
		tag, err := tx.Exec(ctx, `
			UPDATE upi_payment_requests SET status = 'rejected', rejection_reason = $1, verified_by = $2, verified_at = now()
			WHERE id = $3 AND status IN ('pending', 'utr_submitted')
		`, reason, rejectedBy, requestID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrUPIRequestNotFound
		}
		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: rejectedBy, Action: "upi_request.reject", EntityType: "upi_payment_request", EntityID: requestID,
			Reason: reason,
		})
	})
}

func (r *Repository) ListUPIRequestsForStudent(ctx context.Context, studentID uuid.UUID) ([]UPIPaymentRequest, error) {
	out := []UPIPaymentRequest{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, student_id, reference, amount_paise, intent_link, status,
			       COALESCE(submitted_utr, ''), verified_payment_id, COALESCE(rejection_reason, ''), created_at
			FROM upi_payment_requests WHERE student_id = $1 ORDER BY created_at DESC
		`, studentID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var req UPIPaymentRequest
			if err := rows.Scan(&req.ID, &req.StudentID, &req.Reference, &req.AmountPaise, &req.IntentLink, &req.Status,
				&req.SubmittedUTR, &req.VerifiedPaymentID, &req.RejectionReason, &req.CreatedAt); err != nil {
				return err
			}
			out = append(out, req)
		}
		return rows.Err()
	})
	return out, err
}

// PendingUPIRequests is the office worklist: everything awaiting a UTR
// submission or a verification decision, across every student.
func (r *Repository) PendingUPIRequests(ctx context.Context) ([]UPIPaymentRequest, error) {
	out := []UPIPaymentRequest{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, student_id, reference, amount_paise, intent_link, status,
			       COALESCE(submitted_utr, ''), verified_payment_id, COALESCE(rejection_reason, ''), created_at
			FROM upi_payment_requests WHERE status IN ('pending', 'utr_submitted') ORDER BY created_at
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var req UPIPaymentRequest
			if err := rows.Scan(&req.ID, &req.StudentID, &req.Reference, &req.AmountPaise, &req.IntentLink, &req.Status,
				&req.SubmittedUTR, &req.VerifiedPaymentID, &req.RejectionReason, &req.CreatedAt); err != nil {
				return err
			}
			out = append(out, req)
		}
		return rows.Err()
	})
	return out, err
}
