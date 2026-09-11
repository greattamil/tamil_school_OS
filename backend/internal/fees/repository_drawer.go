package fees

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/audit"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrDrawerAlreadyClosed         = errors.New("fees: this drawer is already closed")
	ErrTooManyRecounts             = errors.New("fees: two recounts already used; the variance is locked and needs an explanation to close")
	ErrVarianceExplanationRequired = errors.New("fees: variance exceeds tolerance on the final count; a written explanation is required")
	ErrDrawerNotReadyToClose       = errors.New("fees: submit at least one count before closing")
)

// SubmitCount answers PRD 4.4.6.1's blind-count flow: the clerk's counted
// denomination breakdown is recorded, and only then is it compared against
// the expected total computed from that collector's actual cash receipts for
// the day -- "Showing the expected figure first turns counting into
// confirmation bias." Up to three attempts total (the initial count plus PRD's
// own "two recounts allowed"); the third is rejected outright if it varies
// beyond tolerance and carries no explanation, so the caller can resubmit
// that same third attempt with one rather than losing it as a used attempt.
func (r *Repository) SubmitCount(ctx context.Context, collectorID uuid.UUID, businessDate time.Time, d Denomination, explanation string, submittedBy uuid.UUID) (CashDrawerClosing, error) {
	var out CashDrawerClosing
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var closingID uuid.UUID
		var status DrawerStatus
		var recountNumber int
		err := tx.QueryRow(ctx, `
			SELECT id, status, recount_number FROM cash_drawer_closings
			WHERE school_id = $1 AND collector_id = $2 AND business_date = $3 FOR UPDATE
		`, schoolID, collectorID, businessDate).Scan(&closingID, &status, &recountNumber)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(ctx, `
				INSERT INTO cash_drawer_closings (school_id, collector_id, business_date, status)
				VALUES ($1,$2,$3,'open') RETURNING id, status, recount_number
			`, schoolID, collectorID, businessDate).Scan(&closingID, &status, &recountNumber); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if status == DrawerClosed {
			return ErrDrawerAlreadyClosed
		}

		attemptNumber := recountNumber + 1
		if attemptNumber > 3 {
			return ErrTooManyRecounts
		}

		var expected int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_paise), 0) FROM payments
			WHERE school_id = $1 AND collected_by = $2 AND mode = 'cash' AND is_void = false
			  AND collected_at::date = $3 AND cash_drawer_closing_id IS NULL
		`, schoolID, collectorID, businessDate).Scan(&expected); err != nil {
			return err
		}
		counted := d.TotalPaise()
		variance := counted - expected

		var tolerance int64 = 500
		if err := tx.QueryRow(ctx, `SELECT cash_variance_tolerance_paise FROM fee_settings WHERE school_id = $1`, schoolID).Scan(&tolerance); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if attemptNumber == 3 {
			absVariance := variance
			if absVariance < 0 {
				absVariance = -absVariance
			}
			if absVariance > tolerance && explanation == "" {
				return ErrVarianceExplanationRequired
			}
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_drawer_recount_attempts (school_id, closing_id, attempt_number, denomination_breakdown, counted_total_paise, submitted_by)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, schoolID, closingID, attemptNumber, denominationJSON(d), counted, submittedBy); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE cash_drawer_closings SET
				denomination_breakdown = $1, counted_total_paise = $2, expected_total_paise = $3,
				variance_paise = $4, recount_number = $5, variance_explanation = NULLIF($6, '')
			WHERE id = $7
		`, denominationJSON(d), counted, expected, variance, attemptNumber, explanation, closingID); err != nil {
			return err
		}

		out = CashDrawerClosing{
			ID: closingID, CollectorID: collectorID, BusinessDate: businessDate, Status: DrawerOpen,
			Denomination: &d, CountedTotalPaise: &counted, ExpectedTotalPaise: &expected,
			VariancePaise: &variance, RecountNumber: attemptNumber, VarianceExplanation: explanation,
		}
		return nil
	})
	if err != nil {
		return CashDrawerClosing{}, err
	}
	return out, nil
}

// CloseDrawer answers PRD 4.4.6.1: "Closing freezes that day's cash batch.
// No further cash entry, no voiding, and no back-dating into a closed day."
// Every not-yet-closed cash payment collected by this collector on this date
// is stamped with the closing so VoidPayment/MarkChequeStatus refuse to
// touch it afterward.
func (r *Repository) CloseDrawer(ctx context.Context, closingID uuid.UUID, closedBy uuid.UUID) (CashDrawerClosing, error) {
	var out CashDrawerClosing
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		schoolID, _ := tenancy.SchoolID(ctx)

		var collectorID uuid.UUID
		var businessDate time.Time
		var status DrawerStatus
		var countedTotal, expectedTotal, variance *int64
		var varianceExplanation *string
		if err := tx.QueryRow(ctx, `
			SELECT collector_id, business_date, status, counted_total_paise, expected_total_paise, variance_paise, variance_explanation
			FROM cash_drawer_closings WHERE id = $1 FOR UPDATE
		`, closingID).Scan(&collectorID, &businessDate, &status, &countedTotal, &expectedTotal, &variance, &varianceExplanation); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == DrawerClosed {
			return ErrDrawerAlreadyClosed
		}
		if countedTotal == nil {
			return ErrDrawerNotReadyToClose
		}

		var tolerance int64 = 500
		if err := tx.QueryRow(ctx, `SELECT cash_variance_tolerance_paise FROM fee_settings WHERE school_id = $1`, schoolID).Scan(&tolerance); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if variance != nil {
			v := *variance
			if v < 0 {
				v = -v
			}
			if v > tolerance && (varianceExplanation == nil || *varianceExplanation == "") {
				return ErrVarianceExplanationRequired
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE cash_drawer_closings SET status = 'closed', closed_at = now(), closed_by = $1 WHERE id = $2
		`, closedBy, closingID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE payments SET cash_drawer_closing_id = $1
			WHERE school_id = $2 AND collected_by = $3 AND mode = 'cash' AND is_void = false
			  AND collected_at::date = $4 AND cash_drawer_closing_id IS NULL
		`, closingID, schoolID, collectorID, businessDate); err != nil {
			return err
		}

		if err := audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: closedBy, Action: "cash_drawer.close", EntityType: "cash_drawer_closing", EntityID: closingID,
			After: map[string]any{"counted": countedTotal, "expected": expectedTotal, "variance": variance},
		}); err != nil {
			return err
		}

		out = CashDrawerClosing{ID: closingID, CollectorID: collectorID, BusinessDate: businessDate, Status: DrawerClosed,
			CountedTotalPaise: countedTotal, ExpectedTotalPaise: expectedTotal, VariancePaise: variance}
		return nil
	})
	if err != nil {
		return CashDrawerClosing{}, err
	}
	return out, nil
}

// OpenDrawers answers PRD 4.4.6.1: "Unclosed drawers from prior days appear
// as a blocking item on the office dashboard." now, not just yesterday --
// the moment a business day rolls over, its drawer either got closed or it
// didn't.
func (r *Repository) OpenDrawers(ctx context.Context) ([]CashDrawerClosing, error) {
	out := []CashDrawerClosing{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, collector_id, business_date, status, counted_total_paise, expected_total_paise, variance_paise, recount_number
			FROM cash_drawer_closings WHERE status = 'open' ORDER BY business_date
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c CashDrawerClosing
			if err := rows.Scan(&c.ID, &c.CollectorID, &c.BusinessDate, &c.Status, &c.CountedTotalPaise, &c.ExpectedTotalPaise, &c.VariancePaise, &c.RecountNumber); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func denominationJSON(d Denomination) []byte {
	b, _ := json.Marshal(d)
	return b
}
