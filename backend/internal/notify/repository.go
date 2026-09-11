package notify

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/jobs"
	"school-erp/backend/internal/tenancy"
)

var ErrNoRecipients = errors.New("notify: target resolved to zero recipients")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type recipient struct {
	userID    uuid.UUID
	studentID *uuid.UUID
}

// Compose resolves the target into concrete (guardian) recipients, writes the
// notification and one notification_recipients row per recipient, and enqueues
// the dispatch job in the SAME transaction (PRD 8.3: transactional enqueue) --
// a notice that's recorded but never queued for delivery is exactly the kind of
// silent gap that job design exists to prevent.
func (r *Repository) Compose(ctx context.Context, in ComposeInput, createdBy uuid.UUID) (uuid.UUID, int, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return uuid.Nil, 0, db.ErrNoTenant
	}

	var notificationID uuid.UUID
	var recipientCount int

	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		recipients, err := resolveRecipients(ctx, tx, in)
		if err != nil {
			return err
		}
		if len(recipients) == 0 {
			return ErrNoRecipients
		}

		targetDesc := describeTarget(in)
		if err := tx.QueryRow(ctx, `
			INSERT INTO notifications (school_id, kind, title, body_en, body_ta, created_by, target_description, is_emergency)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING id
		`, schoolID, in.Kind, in.Title, in.BodyEN, nullIfEmpty(in.BodyTA), createdBy, targetDesc, in.IsEmergency).Scan(&notificationID); err != nil {
			return err
		}

		batch := &pgx.Batch{}
		for _, rec := range recipients {
			batch.Queue(`
				INSERT INTO notification_recipients (school_id, notification_id, user_id, student_id)
				VALUES ($1,$2,$3,$4)
				ON CONFLICT (notification_id, user_id) DO NOTHING
			`, schoolID, notificationID, rec.userID, rec.studentID)
		}
		br := tx.SendBatch(ctx, batch)
		for range recipients {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("insert recipient: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return err
		}
		recipientCount = len(recipients)

		return jobs.EnqueueTx(ctx, tx, "dispatch_notification", map[string]any{
			"school_id":       schoolID,
			"notification_id": notificationID,
		})
	})
	if err != nil {
		return uuid.Nil, 0, err
	}
	return notificationID, recipientCount, nil
}

// TeacherOwnsSection reports whether userID is a currently-active teacher
// (section_teachers, scoped to the active academic year) assigned to sectionID --
// class teacher or subject teacher, either is sufficient to notice their own
// section (PRD 2.2 role matrix: "class teacher ... sends notices to their
// section"; subject teachers reasonably share that reach for their sections too).
func (r *Repository) TeacherOwnsSection(ctx context.Context, userID, sectionID uuid.UUID) (bool, error) {
	var owns bool
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM section_teachers st
				JOIN staff s ON s.id = st.staff_id AND s.deleted_at IS NULL
				JOIN academic_years ay ON ay.id = st.academic_year_id AND ay.state = 'active'
				WHERE s.user_id = $1 AND st.section_id = $2 AND st.deleted_at IS NULL
			)
		`, userID, sectionID).Scan(&owns)
	})
	return owns, err
}

// TeacherOwnsStudent reports whether userID teaches the section studentID is
// currently enrolled in.
func (r *Repository) TeacherOwnsStudent(ctx context.Context, userID, studentID uuid.UUID) (bool, error) {
	var owns bool
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM section_teachers st
				JOIN staff s ON s.id = st.staff_id AND s.deleted_at IS NULL
				JOIN academic_years ay ON ay.id = st.academic_year_id AND ay.state = 'active'
				JOIN enrollments e ON e.section_id = st.section_id AND e.deleted_at IS NULL AND upper_inf(e.period)
				WHERE s.user_id = $1 AND e.student_id = $2 AND st.deleted_at IS NULL
			)
		`, userID, studentID).Scan(&owns)
	})
	return owns, err
}

func resolveRecipients(ctx context.Context, tx pgx.Tx, in ComposeInput) ([]recipient, error) {
	var rows pgx.Rows
	var err error

	// PRD 4.5.2: "per-category opt-out (fee reminders, attendance, general
	// notices), which is also a DPDP consent requirement." "notice" is the
	// only human-composed kind Compose ever handles (every automated kind --
	// absence_alert, fee_due_reminder, payment_receipt, etc. -- either goes
	// through here as TargetStudent for a specific student, like
	// payment_receipt, or bypasses Compose entirely, like the absence-alert
	// and fee-reminder scans which check their own category's opt-out
	// directly). Applying the general-notices opt-out only when kind is
	// actually "notice" keeps a payment receipt (also TargetStudent) from
	// being silently filtered by a guardian's *notices* preference, which
	// would be the wrong category entirely.
	isGeneralNotice := in.Kind == KindNotice

	switch in.TargetType {
	case TargetWholeSchool:
		rows, err = tx.Query(ctx, `
			SELECT DISTINCT g.user_id, sg.student_id
			FROM student_guardians sg
			JOIN guardians g ON g.id = sg.guardian_id
			WHERE g.user_id IS NOT NULL AND sg.deleted_at IS NULL AND g.deleted_at IS NULL
			  AND (NOT $1::boolean OR g.opt_out_general_notices = false)
		`, isGeneralNotice)
	case TargetClass:
		if in.ClassID == nil {
			return nil, errors.New("class_id is required for a class-targeted notice")
		}
		rows, err = tx.Query(ctx, `
			SELECT DISTINCT g.user_id, sg.student_id
			FROM enrollments e
			JOIN student_guardians sg ON sg.student_id = e.student_id AND sg.deleted_at IS NULL
			JOIN guardians g ON g.id = sg.guardian_id AND g.deleted_at IS NULL
			WHERE e.class_id = $1 AND e.deleted_at IS NULL AND g.user_id IS NOT NULL AND upper_inf(e.period)
			  AND (NOT $2::boolean OR g.opt_out_general_notices = false)
		`, *in.ClassID, isGeneralNotice)
	case TargetSection:
		if in.SectionID == nil {
			return nil, errors.New("section_id is required for a section-targeted notice")
		}
		rows, err = tx.Query(ctx, `
			SELECT DISTINCT g.user_id, sg.student_id
			FROM enrollments e
			JOIN student_guardians sg ON sg.student_id = e.student_id AND sg.deleted_at IS NULL
			JOIN guardians g ON g.id = sg.guardian_id AND g.deleted_at IS NULL
			WHERE e.section_id = $1 AND e.deleted_at IS NULL AND g.user_id IS NOT NULL AND upper_inf(e.period)
			  AND (NOT $2::boolean OR g.opt_out_general_notices = false)
		`, *in.SectionID, isGeneralNotice)
	case TargetStudent:
		if in.StudentID == nil {
			return nil, errors.New("student_id is required for a student-targeted notice")
		}
		rows, err = tx.Query(ctx, `
			SELECT DISTINCT g.user_id, sg.student_id
			FROM student_guardians sg
			JOIN guardians g ON g.id = sg.guardian_id AND g.deleted_at IS NULL
			WHERE sg.student_id = $1 AND sg.deleted_at IS NULL AND g.user_id IS NOT NULL
			  AND (NOT $2::boolean OR g.opt_out_general_notices = false)
		`, *in.StudentID, isGeneralNotice)
	case TargetCustom:
		recipients := make([]recipient, 0, len(in.CustomUserIDs))
		for _, uid := range in.CustomUserIDs {
			id := uid
			recipients = append(recipients, recipient{userID: id})
		}
		return recipients, nil
	default:
		return nil, fmt.Errorf("unknown target type %q", in.TargetType)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []recipient
	for rows.Next() {
		var rec recipient
		if err := rows.Scan(&rec.userID, &rec.studentID); err != nil {
			return nil, err
		}
		recipients = append(recipients, rec)
	}
	return recipients, rows.Err()
}

func describeTarget(in ComposeInput) string {
	switch in.TargetType {
	case TargetWholeSchool:
		return "Whole school"
	case TargetClass:
		return "Class"
	case TargetSection:
		return "Section"
	case TargetStudent:
		return "Individual student"
	case TargetCustom:
		return fmt.Sprintf("Custom list (%d recipients)", len(in.CustomUserIDs))
	default:
		return ""
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// RegisterDeviceToken upserts an FCM token for the current user+school. Called by
// the mobile client on login and on token refresh.
func (r *Repository) RegisterDeviceToken(ctx context.Context, userID uuid.UUID, token, platform string) error {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return db.ErrNoTenant
	}
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO device_push_tokens (school_id, user_id, fcm_token, platform)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (school_id, user_id, fcm_token) DO UPDATE SET updated_at = now(), deleted_at = NULL
		`, schoolID, userID, token, platform)
		return err
	})
}

// Inbox returns a recipient's own notifications feed, most recent first (PRD 4.8:
// parent dashboard shows recent notices; PRD 4.5.1 delivery/read tracking is
// per-recipient, which is exactly what read_at here reflects).
func (r *Repository) Inbox(ctx context.Context, userID uuid.UUID, limit int) ([]InboxItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	out := []InboxItem{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT n.id, n.kind, n.title, n.body_en, n.body_ta, nr.student_id, nr.read_at, n.created_at
			FROM notification_recipients nr
			JOIN notifications n ON n.id = nr.notification_id
			WHERE nr.user_id = $1
			ORDER BY n.created_at DESC
			LIMIT $2
		`, userID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item InboxItem
			var bodyTA *string
			if err := rows.Scan(&item.NotificationID, &item.Kind, &item.Title, &item.BodyEN, &bodyTA, &item.StudentID, &item.ReadAt, &item.CreatedAt); err != nil {
				return err
			}
			if bodyTA != nil {
				item.BodyTA = *bodyTA
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

// AckPush records that a recipient's device received a push, so the SMS-rollover
// scan (PRD 4.5.3) can stop treating it as undelivered. Idempotent: acking twice,
// or acking after an SMS already went out, is harmless.
func (r *Repository) AckPush(ctx context.Context, userID, notificationID uuid.UUID) error {
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE notification_recipients SET push_acknowledged_at = now()
			WHERE user_id = $1 AND notification_id = $2 AND push_acknowledged_at IS NULL
		`, userID, notificationID)
		return err
	})
}

func (r *Repository) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE notification_recipients SET read_at = now()
			WHERE user_id = $1 AND notification_id = $2 AND read_at IS NULL
		`, userID, notificationID)
		return err
	})
}
