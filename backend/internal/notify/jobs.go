package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	appjobs "school-erp/backend/internal/jobs"
	"school-erp/backend/internal/tenancy"
)

// RegisterJobHandlers wires this package's job handlers into a worker (PRD 8.3:
// a separate Go process claims and processes jobs from the shared PostgreSQL
// table).
func RegisterJobHandlers(w *appjobs.Worker, pool *pgxpool.Pool, dispatcher Dispatcher) {
	w.Register("dispatch_notification", dispatchNotificationHandler(pool, dispatcher))
	w.Register("dispatch_absence_alerts", dispatchAbsenceAlertsHandler(pool))
}

type dispatchNotificationPayload struct {
	SchoolID       uuid.UUID `json:"school_id"`
	NotificationID uuid.UUID `json:"notification_id"`
}

// dispatchNotificationHandler fans a composed notification out to every
// recipient's registered devices. Batches of up to 500 tokens per PRD 8.4, so
// the emergency-broadcast target of "1,500 recipients reached within 3 minutes"
// is three multicast calls, not 1,500 sequential ones.
func dispatchNotificationHandler(pool *pgxpool.Pool, dispatcher Dispatcher) appjobs.Handler {
	return func(ctx context.Context, job appjobs.Job) error {
		var p dispatchNotificationPayload
		if err := json.Unmarshal(job.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}
		ctx = tenancy.WithSchoolID(ctx, p.SchoolID)

		var title, bodyEN string
		if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT title, body_en FROM notifications WHERE id = $1`, p.NotificationID).Scan(&title, &bodyEN)
		}); err != nil {
			return fmt.Errorf("load notification: %w", err)
		}

		type recipientToken struct {
			recipientUserID uuid.UUID
			token           string
		}
		var recipientTokens []recipientToken
		if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT nr.user_id, dpt.fcm_token
				FROM notification_recipients nr
				JOIN device_push_tokens dpt ON dpt.user_id = nr.user_id AND dpt.deleted_at IS NULL
				WHERE nr.notification_id = $1
			`, p.NotificationID)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var rt recipientToken
				if err := rows.Scan(&rt.recipientUserID, &rt.token); err != nil {
					return err
				}
				recipientTokens = append(recipientTokens, rt)
			}
			return rows.Err()
		}); err != nil {
			return fmt.Errorf("load recipient tokens: %w", err)
		}

		const batchSize = 500
		for start := 0; start < len(recipientTokens); start += batchSize {
			end := min(start+batchSize, len(recipientTokens))
			batch := recipientTokens[start:end]
			tokens := make([]string, len(batch))
			for i, rt := range batch {
				tokens[i] = rt.token
			}

			results := dispatcher.SendMulticast(ctx, tokens, title, bodyEN)

			for i, res := range results {
				rt := batch[i]
				if res.Sent {
					_ = db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
						_, err := tx.Exec(ctx, `
							UPDATE notification_recipients SET push_sent_at = now()
							WHERE notification_id = $1 AND user_id = $2
						`, p.NotificationID, rt.recipientUserID)
						return err
					})
				}
				if res.Invalid {
					// Stale token pruning (PRD 8.4): a token FCM reports as
					// unregistered/invalid is removed so future sends don't
					// keep paying for a dead device.
					_ = db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
						_, err := tx.Exec(ctx, `UPDATE device_push_tokens SET deleted_at = now() WHERE user_id = $1 AND fcm_token = $2`, rt.recipientUserID, res.Token)
						return err
					})
				}
			}
		}

		return nil
	}
}

// ScanAbsenceNotifications finds schools whose configured absence-notification
// time (PRD 4.2.3, default 11:00) has passed for today and which have at least
// one synced, un-notified absence, and enqueues one dispatch_absence_alerts job
// per such school. Runs with no tenant context -- it deliberately looks across
// every school, which is why it queries school_settings and attendance_entries
// directly rather than going through db.WithTenantTx.
func ScanAbsenceNotifications(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ae.school_id
		FROM attendance_entries ae
		JOIN school_settings ss ON ss.school_id = ae.school_id
		WHERE ae.status = 'absent'
		  AND ae.absence_notified_at IS NULL
		  AND ae.date = (now() AT TIME ZONE 'Asia/Kolkata')::date
		  AND (now() AT TIME ZONE 'Asia/Kolkata')::time >= ss.absence_notification_time
	`)
	if err != nil {
		return 0, fmt.Errorf("scan schools for absence notification: %w", err)
	}
	var schoolIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		schoolIDs = append(schoolIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	today := time.Now().In(time.FixedZone("IST", 5*3600+30*60)).Format("2006-01-02")
	for _, schoolID := range schoolIDs {
		if err := appjobs.Enqueue(ctx, pool, "dispatch_absence_alerts", map[string]any{
			"school_id": schoolID,
			"date":      today,
		}); err != nil {
			return 0, fmt.Errorf("enqueue absence alerts for school %s: %w", schoolID, err)
		}
	}
	return len(schoolIDs), nil
}

type dispatchAbsenceAlertsPayload struct {
	SchoolID uuid.UUID `json:"school_id"`
	Date     string    `json:"date"`
}

// dispatchAbsenceAlertsHandler creates one absence_alert notification per
// still-un-notified absent entry for the school+date in the payload, targeted at
// that student's primary guardian, then enqueues the push dispatch for each.
// Entries with no primary guardian on file are marked notified anyway (nothing to
// send to) rather than re-scanned forever -- a data-quality gap for the office to
// close by completing the guardian linkage, not a retry condition.
func dispatchAbsenceAlertsHandler(pool *pgxpool.Pool) appjobs.Handler {
	return func(ctx context.Context, job appjobs.Job) error {
		var p dispatchAbsenceAlertsPayload
		if err := json.Unmarshal(job.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}
		ctx = tenancy.WithSchoolID(ctx, p.SchoolID)

		type absentStudent struct {
			entryID               uuid.UUID
			studentID             uuid.UUID
			studentName           string
			primaryGuardianUserID *uuid.UUID
		}
		var absences []absentStudent

		err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT ae.id, e.student_id, s.name_english,
				       (SELECT g.user_id FROM student_guardians sg
				        JOIN guardians g ON g.id = sg.guardian_id
				        WHERE sg.student_id = e.student_id AND sg.is_primary_contact = true
				          AND sg.deleted_at IS NULL AND g.deleted_at IS NULL
				        LIMIT 1)
				FROM attendance_entries ae
				JOIN enrollments e ON e.id = ae.enrollment_id
				JOIN students s ON s.id = e.student_id
				WHERE ae.school_id = $1 AND ae.date = $2::date AND ae.status = 'absent' AND ae.absence_notified_at IS NULL
			`, p.SchoolID, p.Date)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var a absentStudent
				if err := rows.Scan(&a.entryID, &a.studentID, &a.studentName, &a.primaryGuardianUserID); err != nil {
					return err
				}
				absences = append(absences, a)
			}
			return rows.Err()
		})
		if err != nil {
			return fmt.Errorf("load absent entries: %w", err)
		}

		for _, a := range absences {
			err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
				if a.primaryGuardianUserID != nil {
					var notificationID uuid.UUID
					if err := tx.QueryRow(ctx, `
						INSERT INTO notifications (school_id, kind, title, body_en, created_by, target_description)
						VALUES ($1, 'absence_alert', 'Absence recorded', $2, NULL, $3)
						RETURNING id
					`, p.SchoolID, fmt.Sprintf("%s was marked absent today.", a.studentName), a.studentName).Scan(&notificationID); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, `
						INSERT INTO notification_recipients (school_id, notification_id, user_id, student_id)
						VALUES ($1,$2,$3,$4)
					`, p.SchoolID, notificationID, *a.primaryGuardianUserID, a.studentID); err != nil {
						return err
					}
					if err := appjobs.EnqueueTx(ctx, tx, "dispatch_notification", map[string]any{
						"school_id":       p.SchoolID,
						"notification_id": notificationID,
					}); err != nil {
						return err
					}
				}
				_, err := tx.Exec(ctx, `UPDATE attendance_entries SET absence_notified_at = now() WHERE id = $1`, a.entryID)
				return err
			})
			if err != nil {
				return fmt.Errorf("dispatch absence alert for entry %s: %w", a.entryID, err)
			}
		}

		return nil
	}
}
