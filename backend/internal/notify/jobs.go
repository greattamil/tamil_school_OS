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

		var title, bodyEN, kind string
		var isEmergency bool
		if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT title, body_en, kind, is_emergency FROM notifications WHERE id = $1`, p.NotificationID).
				Scan(&title, &bodyEN, &kind, &isEmergency)
		}); err != nil {
			return fmt.Errorf("load notification: %w", err)
		}

		// PRD 4.5.3/4.5.4: emergency broadcasts bypass quiet hours and frequency
		// caps entirely -- the only message class permitted to. "notice" is the
		// only human-composed kind (every other kind is system-generated), so
		// the "automated notification" message-discipline rules in 4.5.4 apply
		// to everything except it.
		automated := !isEmergency && kind != string(KindNotice)

		if automated {
			var inQuietHours bool
			var nextAllowed time.Time
			var maxDaily int
			if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT
					  CASE WHEN quiet_hours_start <= quiet_hours_end
					       THEN (now() AT TIME ZONE 'Asia/Kolkata')::time >= quiet_hours_start
					            AND (now() AT TIME ZONE 'Asia/Kolkata')::time < quiet_hours_end
					       ELSE (now() AT TIME ZONE 'Asia/Kolkata')::time >= quiet_hours_start
					            OR (now() AT TIME ZONE 'Asia/Kolkata')::time < quiet_hours_end
					  END,
					  ((((now() AT TIME ZONE 'Asia/Kolkata')::date
					      + CASE WHEN (now() AT TIME ZONE 'Asia/Kolkata')::time < quiet_hours_end THEN 0 ELSE 1 END
					    ) + quiet_hours_end) AT TIME ZONE 'Asia/Kolkata'),
					  max_daily_automated_messages
					FROM school_settings WHERE school_id = $1
				`, p.SchoolID).Scan(&inQuietHours, &nextAllowed, &maxDaily)
			}); err != nil {
				return fmt.Errorf("load school_settings for quiet hours: %w", err)
			}

			if inQuietHours {
				// Defer rather than drop: re-enqueue the same dispatch for the
				// moment quiet hours end, and let this claim complete as done --
				// it has been fully "handled" by scheduling its replacement.
				return db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
					return appjobs.EnqueueTxAt(ctx, tx, "dispatch_notification", p, nextAllowed)
				})
			}
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

		// PRD 4.5.4: cap automated pushes per guardian per day. The in-app
		// notification row always exists regardless (per-recipient delivery/read
		// tracking, PRD 4.5.1) -- the cap governs push/SMS noise, not visibility.
		overCap := map[uuid.UUID]bool{}
		if automated && len(recipientTokens) > 0 {
			var maxDaily int
			if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
				return tx.QueryRow(ctx, `SELECT max_daily_automated_messages FROM school_settings WHERE school_id = $1`, p.SchoolID).Scan(&maxDaily)
			}); err != nil {
				return fmt.Errorf("load max_daily_automated_messages: %w", err)
			}
			userIDs := make([]uuid.UUID, len(recipientTokens))
			for i, rt := range recipientTokens {
				userIDs[i] = rt.recipientUserID
			}
			if err := db.WithTenantTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
				rows, err := tx.Query(ctx, `
					SELECT nr.user_id, count(*)
					FROM notification_recipients nr
					JOIN notifications n ON n.id = nr.notification_id
					WHERE nr.user_id = ANY($1) AND n.kind != 'notice' AND n.is_emergency = false
					  AND nr.push_sent_at IS NOT NULL
					  AND (nr.push_sent_at AT TIME ZONE 'Asia/Kolkata')::date = (now() AT TIME ZONE 'Asia/Kolkata')::date
					GROUP BY nr.user_id
				`, userIDs)
				if err != nil {
					return err
				}
				defer rows.Close()
				for rows.Next() {
					var uid uuid.UUID
					var count int
					if err := rows.Scan(&uid, &count); err != nil {
						return err
					}
					if count >= maxDaily {
						overCap[uid] = true
					}
				}
				return rows.Err()
			}); err != nil {
				return fmt.Errorf("load daily automated-message counts: %w", err)
			}
		}

		const batchSize = 500
		for start := 0; start < len(recipientTokens); start += batchSize {
			end := min(start+batchSize, len(recipientTokens))
			batch := recipientTokens[start:end]
			var tokens []string
			var sendable []recipientToken
			for _, rt := range batch {
				if overCap[rt.recipientUserID] {
					continue
				}
				tokens = append(tokens, rt.token)
				sendable = append(sendable, rt)
			}
			if len(tokens) == 0 {
				continue
			}

			results := dispatcher.SendMulticast(ctx, tokens, title, bodyEN)

			for i, res := range results {
				rt := sendable[i]
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
	allSchoolIDs, err := activeSchoolIDs(ctx, pool)
	if err != nil {
		return 0, fmt.Errorf("scan schools for absence notification: %w", err)
	}

	// attendance_entries and school_settings are both RLS-protected tenant
	// tables (PRD 6.1), and this worker's pool connects as app_user, which
	// never gets BYPASSRLS (PRD 6.1 point 6) -- so this cannot be one
	// cross-school query with no tenant context set (that would silently see
	// zero rows on every school, not an error, which is what made this
	// exact bug invisible until traced with a direct app_user query). Loop
	// every school instead, checking each under its own tenant context, same
	// as dispatchAbsenceAlertsHandler and dispatchNotificationHandler already
	// do correctly for their own per-school work.
	var matched []uuid.UUID
	for _, schoolID := range allSchoolIDs {
		sctx := tenancy.WithSchoolID(ctx, schoolID)
		var hasUnnotified bool
		if err := db.WithTenantTx(sctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM attendance_entries ae, school_settings ss
					WHERE ae.status = 'absent'
					  AND ae.absence_notified_at IS NULL
					  AND ae.date = (now() AT TIME ZONE 'Asia/Kolkata')::date
					  AND (now() AT TIME ZONE 'Asia/Kolkata')::time >= ss.absence_notification_time
				)
			`).Scan(&hasUnnotified)
		}); err != nil {
			return 0, fmt.Errorf("check absence notification for school %s: %w", schoolID, err)
		}
		if hasUnnotified {
			matched = append(matched, schoolID)
		}
	}

	today := time.Now().In(time.FixedZone("IST", 5*3600+30*60)).Format("2006-01-02")
	for _, schoolID := range matched {
		if err := appjobs.Enqueue(ctx, pool, "dispatch_absence_alerts", map[string]any{
			"school_id": schoolID,
			"date":      today,
		}); err != nil {
			return 0, fmt.Errorf("enqueue absence alerts for school %s: %w", schoolID, err)
		}
	}
	return len(matched), nil
}

// activeSchoolIDs lists every non-deleted school. schools itself carries no
// school_id and is not RLS-protected (PRD 3.2.1: identity/tenant-registry
// tables sit outside the tenant boundary), so this is safe to query with no
// tenant context -- unlike the tenant tables the per-school loops above touch.
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

// ScanSMSRollover implements PRD 4.5.3's automatic SMS rollover: any recipient of
// an emergency broadcast whose push was sent but not acknowledged within window
// gets a fallback SMS. Scoped to is_emergency = true only -- routine notices and
// automated notifications don't get this treatment, per PRD 4.5.3's own framing
// ("this is the only message type permitted to" bypass quiet hours/caps; the SMS
// rollover sits in the same emergency-only paragraph). Runs with no tenant
// context, like ScanAbsenceNotifications, since it deliberately looks across
// every school; users.mobile is a global (non-RLS) column so it can be read here
// directly.
func ScanSMSRollover(ctx context.Context, pool *pgxpool.Pool, sender SMSSender, window time.Duration) (int, error) {
	allSchoolIDs, err := activeSchoolIDs(ctx, pool)
	if err != nil {
		return 0, fmt.Errorf("scan for sms rollover: %w", err)
	}

	type candidate struct {
		recipientID    uuid.UUID
		schoolID       uuid.UUID
		notificationID uuid.UUID
		mobile         string
		title          string
	}
	var candidates []candidate

	// notification_recipients and notifications are RLS-protected tenant
	// tables (PRD 6.1); the worker's pool has no BYPASSRLS (PRD 6.1 point 6).
	// A single cross-school query with no tenant context set would silently
	// match zero rows on every school rather than error -- the same class of
	// bug traced (via a direct app_user query showing 0 rows against real
	// data) in ScanAbsenceNotifications above. Loop per school under its own
	// tenant context instead. users.mobile is a global, non-RLS column
	// (PRD 3.2.1), so joining it inside each school's tx is still safe.
	for _, schoolID := range allSchoolIDs {
		sctx := tenancy.WithSchoolID(ctx, schoolID)
		if err := db.WithTenantTx(sctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT nr.id, nr.notification_id, u.mobile, n.title
				FROM notification_recipients nr
				JOIN notifications n ON n.id = nr.notification_id
				JOIN users u ON u.id = nr.user_id
				WHERE n.is_emergency = true
				  AND nr.push_sent_at IS NOT NULL
				  AND nr.push_sent_at <= now() - make_interval(secs => $1)
				  AND nr.push_acknowledged_at IS NULL
				  AND nr.sms_sent_at IS NULL
				  AND u.mobile IS NOT NULL
			`, window.Seconds())
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				c := candidate{schoolID: schoolID}
				if err := rows.Scan(&c.recipientID, &c.notificationID, &c.mobile, &c.title); err != nil {
					return err
				}
				candidates = append(candidates, c)
			}
			return rows.Err()
		}); err != nil {
			return 0, fmt.Errorf("scan sms rollover candidates for school %s: %w", schoolID, err)
		}
	}

	sent := 0
	for _, c := range candidates {
		// EMERGENCY_HOLIDAY DLT template (PRD 9, week 1): registered with fixed
		// variable slots, so the message body here must match what was actually
		// approved -- title is the one variable this template carries.
		message := fmt.Sprintf("School alert: %s. Open the Tamil School OS app for details.", c.title)
		if !sender.Send(ctx, c.mobile, message) {
			continue
		}
		sctx := tenancy.WithSchoolID(ctx, c.schoolID)
		if err := db.WithTenantTx(sctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE notification_recipients SET sms_sent_at = now() WHERE id = $1`, c.recipientID)
			return err
		}); err != nil {
			return sent, fmt.Errorf("mark sms sent for recipient %s: %w", c.recipientID, err)
		}
		sent++
	}
	return sent, nil
}
