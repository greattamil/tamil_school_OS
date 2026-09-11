package fees

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	appjobs "school-erp/backend/internal/jobs"
	"school-erp/backend/internal/notify"
	"school-erp/backend/internal/tenancy"
)

// ScanFeeReminders answers PRD 4.4.5: "Automated reminders to fee-responsible
// guardians, schedulable, with a per-school opt-out and a hard cap on
// frequency." The opt-out and cap are already enforced generically by the
// notify module (quiet hours, the daily automated-message cap, both built
// earlier this phase) the moment this creates a notification of kind
// fee_due_reminder/fee_overdue_reminder -- this function's only job is
// deciding *which* line items qualify and *when* each one has already been
// reminded about.
//
// Loops every school under its own real tenant context (db.WithTenantTx),
// the same lesson already learned twice this phase: fee_line_items and
// student_guardians are RLS-protected, and this worker's pool has no
// BYPASSRLS, so a single cross-school query with no context set would
// silently match nothing rather than error.
func ScanFeeReminders(ctx context.Context, pool *pgxpool.Pool, dueWindow time.Duration, overdueCooldown time.Duration) (int, error) {
	schoolIDs, err := activeSchoolIDs(ctx, pool)
	if err != nil {
		return 0, fmt.Errorf("scan for fee reminders: %w", err)
	}

	sent := 0
	for _, schoolID := range schoolIDs {
		sctx := tenancy.WithSchoolID(ctx, schoolID)
		n, err := remindOneSchool(sctx, pool, schoolID, dueWindow, overdueCooldown)
		if err != nil {
			return sent, fmt.Errorf("fee reminders for school %s: %w", schoolID, err)
		}
		sent += n
	}
	return sent, nil
}

type reminderCandidate struct {
	lineItemID  uuid.UUID
	studentID   uuid.UUID
	dueDate     time.Time
	outstanding int64
	headName    string
	kind        notify.Kind
}

func remindOneSchool(ctx context.Context, pool *pgxpool.Pool, schoolID uuid.UUID, dueWindow, overdueCooldown time.Duration) (int, error) {
	var candidates []reminderCandidate
	err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		dueRows, err := tx.Query(ctx, `
			SELECT li.id, li.student_id, li.due_date, li.net_amount_paise - li.paid_amount_paise, fh.name
			FROM fee_line_items li
			JOIN fee_heads fh ON fh.id = li.fee_head_id
			WHERE li.status IN ('pending', 'partially_paid')
			  AND li.due_reminder_sent_at IS NULL
			  AND li.due_date BETWEEN CURRENT_DATE AND (CURRENT_DATE + make_interval(secs => $1))
		`, dueWindow.Seconds())
		if err != nil {
			return err
		}
		for dueRows.Next() {
			var c reminderCandidate
			if err := dueRows.Scan(&c.lineItemID, &c.studentID, &c.dueDate, &c.outstanding, &c.headName); err != nil {
				dueRows.Close()
				return err
			}
			c.kind = notify.KindFeeDueReminder
			candidates = append(candidates, c)
		}
		dueRows.Close()
		if err := dueRows.Err(); err != nil {
			return err
		}

		overdueRows, err := tx.Query(ctx, `
			SELECT li.id, li.student_id, li.due_date, li.net_amount_paise - li.paid_amount_paise, fh.name
			FROM fee_line_items li
			JOIN fee_heads fh ON fh.id = li.fee_head_id
			WHERE li.status IN ('pending', 'partially_paid')
			  AND li.due_date < CURRENT_DATE
			  AND (li.overdue_reminder_sent_at IS NULL OR li.overdue_reminder_sent_at <= now() - make_interval(secs => $1))
		`, overdueCooldown.Seconds())
		if err != nil {
			return err
		}
		for overdueRows.Next() {
			var c reminderCandidate
			if err := overdueRows.Scan(&c.lineItemID, &c.studentID, &c.dueDate, &c.outstanding, &c.headName); err != nil {
				overdueRows.Close()
				return err
			}
			c.kind = notify.KindFeeOverdueReminder
			candidates = append(candidates, c)
		}
		overdueRows.Close()
		return overdueRows.Err()
	})
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, c := range candidates {
		if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			// PRD 4.4.5 says "fee-responsible guardians" specifically, not
			// just the primary contact (PRD 3.3: is_fee_responsible is its
			// own flag precisely because the family may want dues addressed
			// to whichever parent actually handles money, not necessarily
			// whoever is the primary point of contact for everything else).
			// PRD 4.5.2: per-category opt-out is a DPDP consent requirement --
			// a guardian who has opted out of fee reminders specifically must
			// never receive one, independent of the quiet-hours/daily-cap
			// volume controls notify.Compose already enforces elsewhere.
			var guardianUserID *uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT g.user_id FROM student_guardians sg
				JOIN guardians g ON g.id = sg.guardian_id
				WHERE sg.student_id = $1 AND sg.is_fee_responsible = true
				  AND sg.deleted_at IS NULL AND g.deleted_at IS NULL
				  AND g.opt_out_fee_reminders = false
				LIMIT 1
			`, c.studentID).Scan(&guardianUserID); err != nil && err != pgx.ErrNoRows {
				return err
			}

			if guardianUserID != nil {
				title := "Fee due reminder"
				body := fmt.Sprintf("%s: %s due on %s.", c.headName, formatPaiseForNotify(c.outstanding), c.dueDate.Format("2 Jan 2006"))
				if c.kind == notify.KindFeeOverdueReminder {
					title = "Fee overdue"
					body = fmt.Sprintf("%s: %s was due on %s and is still outstanding.", c.headName, formatPaiseForNotify(c.outstanding), c.dueDate.Format("2 Jan 2006"))
				}

				var notificationID uuid.UUID
				if err := tx.QueryRow(ctx, `
					INSERT INTO notifications (school_id, kind, title, body_en, created_by, target_description)
					VALUES ($1, $2, $3, $4, NULL, $5)
					RETURNING id
				`, schoolID, c.kind, title, body, "Fee reminder").Scan(&notificationID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO notification_recipients (school_id, notification_id, user_id, student_id)
					VALUES ($1,$2,$3,$4)
				`, schoolID, notificationID, *guardianUserID, c.studentID); err != nil {
					return err
				}
				if err := appjobs.EnqueueTx(ctx, tx, "dispatch_notification", map[string]any{
					"school_id":       schoolID,
					"notification_id": notificationID,
				}); err != nil {
					return err
				}
				sent++
			}

			column := "due_reminder_sent_at"
			if c.kind == notify.KindFeeOverdueReminder {
				column = "overdue_reminder_sent_at"
			}
			_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE fee_line_items SET %s = now() WHERE id = $1`, column), c.lineItemID)
			return err
		}); err != nil {
			return sent, fmt.Errorf("remind for line item %s: %w", c.lineItemID, err)
		}
	}
	return sent, nil
}

func formatPaiseForNotify(paise int64) string {
	return fmt.Sprintf("Rs %d.%02d", paise/100, paise%100)
}

// activeSchoolIDs lists every non-deleted school. schools carries no
// school_id and is not RLS-protected (PRD 3.2.1: identity/tenant-registry
// tables sit outside the tenant boundary), so this is safe to query with no
// tenant context -- a small duplicate of notify's own unexported helper of
// the same name, kept local rather than exported cross-package for two
// nearly-identical three-line queries.
func activeSchoolIDs(ctx context.Context, pool *pgxpool.Pool) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `SELECT id FROM schools WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
