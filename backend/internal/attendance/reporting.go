package attendance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/db"
)

// DailySummaryRow is present/absent counts for one section on one date, or the
// school-wide total when SectionID is nil.
type DailySummaryRow struct {
	SectionID *uuid.UUID `json:"section_id,omitempty"`
	Present   int        `json:"present"`
	Absent    int        `json:"absent"`
	Total     int        `json:"total"`
}

// DailySummary answers PRD 4.2.4: "Daily section-wise and school-wise
// present/absent counts." Passing a nil sectionID returns one row per section for
// the whole school; a non-nil sectionID returns just that section's row.
func (r *Repository) DailySummary(ctx context.Context, sectionID *uuid.UUID, date time.Time) ([]DailySummaryRow, error) {
	out := []DailySummaryRow{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var rows pgx.Rows
		var err error
		if sectionID != nil {
			rows, err = tx.Query(ctx, `
				SELECT ae.register_id, ar.section_id,
				       count(*) FILTER (WHERE ae.status = 'present') AS present,
				       count(*) FILTER (WHERE ae.status = 'absent') AS absent
				FROM attendance_entries ae
				JOIN attendance_registers ar ON ar.id = ae.register_id
				WHERE ae.date = $1 AND ar.section_id = $2 AND ae.deleted_at IS NULL
				GROUP BY ae.register_id, ar.section_id
			`, date, *sectionID)
		} else {
			rows, err = tx.Query(ctx, `
				SELECT ar.section_id,
				       count(*) FILTER (WHERE ae.status = 'present') AS present,
				       count(*) FILTER (WHERE ae.status = 'absent') AS absent
				FROM attendance_entries ae
				JOIN attendance_registers ar ON ar.id = ae.register_id
				WHERE ae.date = $1 AND ae.deleted_at IS NULL
				GROUP BY ar.section_id
			`, date)
		}
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row DailySummaryRow
			var sid uuid.UUID
			if sectionID != nil {
				var registerID uuid.UUID
				if err := rows.Scan(&registerID, &sid, &row.Present, &row.Absent); err != nil {
					return err
				}
			} else {
				if err := rows.Scan(&sid, &row.Present, &row.Absent); err != nil {
					return err
				}
			}
			row.SectionID = &sid
			row.Total = row.Present + row.Absent
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}

// AttendancePercentage answers PRD 4.2.4: "Per-student attendance percentage over
// a date range."
//
// Known simplification, tracked in docs/PROGRESS.md: this counts days with a
// recorded attendance entry as the working-day denominator, rather than resolving
// against the academic_calendar_days entity PRD 3.1 specifies (explicit
// day_type per date: regular_working / holiday / compensatory_working / etc).
// That entity does not exist yet. Today's calculation is at least immune to the
// worst version of the bug PRD 3.1 warns about (a Monday-to-Friday weekday
// assumption), because a date with no attendance entry recorded simply isn't
// counted -- but it is NOT yet correct for a retroactively-declared holiday
// whose attendance entries were preserved rather than deleted (PRD 3.1: "excluded
// from totals rather than deleted"), since nothing here knows to exclude them.
// This must be revisited before the figure is trusted on a TC or official
// register export.
func (r *Repository) AttendancePercentage(ctx context.Context, enrollmentID uuid.UUID, from, to time.Time) (presentDays, totalDays int, err error) {
	err = db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE status = 'present'), count(*)
			FROM attendance_entries
			WHERE enrollment_id = $1 AND date BETWEEN $2 AND $3 AND deleted_at IS NULL
		`, enrollmentID, from, to).Scan(&presentDays, &totalDays)
	})
	return presentDays, totalDays, err
}

// AttendancePercentageForStudent is the student-facing equivalent of
// AttendancePercentage: it aggregates across every enrollment the student has
// held (joining via enrollments.student_id rather than a single enrollment_id),
// so a student transferred mid-range between sections still gets one correct
// figure instead of a partial one scoped to whichever enrollment happened to be
// active when the query was written (PRD 3.3: history stays attached to the
// enrollment that was active when it was recorded, but a student's own summary
// should still read across all of it).
func (r *Repository) AttendancePercentageForStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time) (presentDays, totalDays int, err error) {
	err = db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE ae.status = 'present'), count(*)
			FROM attendance_entries ae
			JOIN enrollments e ON e.id = ae.enrollment_id
			WHERE e.student_id = $1 AND ae.date BETWEEN $2 AND $3 AND ae.deleted_at IS NULL
		`, studentID, from, to).Scan(&presentDays, &totalDays)
	})
	return presentDays, totalDays, err
}

// EntryView is one student's attendance for a register, joined with enough
// enrollment detail for a client to render a marking screen without a second
// round trip.
type EntryView struct {
	EnrollmentID   uuid.UUID `json:"enrollment_id"`
	StudentID      uuid.UUID `json:"student_id"`
	StudentName    string    `json:"student_name"`
	RollNumber     *string   `json:"roll_number,omitempty"`
	Status         *Status   `json:"status,omitempty"` // nil if not yet marked
	Reason         *Reason   `json:"reason,omitempty"`
	ServerRevision int       `json:"server_revision"`
}

// GetRegisterEntries returns every active-enrolled student in sectionID as of
// date, left-joined with whatever attendance entry already exists for that date.
// This is what a client fetches before marking -- both to render the roster (the
// PRD 4.2.1 grid of students) and to learn the server_revision each entry is
// currently at, which the client must echo back as base_revision on its next
// sync for the conflict check in conflict.go to mean anything.
func (r *Repository) GetRegisterEntries(ctx context.Context, sectionID uuid.UUID, date time.Time) ([]EntryView, error) {
	out := []EntryView{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT e.id, e.student_id, s.name_english, e.roll_number,
			       ae.status, ae.reason, COALESCE(ae.server_revision, 0)
			FROM enrollments e
			JOIN students s ON s.id = e.student_id
			LEFT JOIN attendance_entries ae ON ae.enrollment_id = e.id AND ae.date = $2 AND ae.deleted_at IS NULL
			WHERE e.section_id = $1 AND e.period @> $2::date AND e.deleted_at IS NULL
			ORDER BY e.roll_number NULLS LAST, s.name_english
		`, sectionID, date)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v EntryView
			if err := rows.Scan(&v.EnrollmentID, &v.StudentID, &v.StudentName, &v.RollNumber, &v.Status, &v.Reason, &v.ServerRevision); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

type ConsecutiveAbsenceRow struct {
	EnrollmentID    uuid.UUID `json:"enrollment_id"`
	ConsecutiveDays int       `json:"consecutive_days"`
	LastAbsentDate  time.Time `json:"last_absent_date"`
}

// ConsecutiveAbsences answers PRD 4.2.4's consecutive-absence report (default
// threshold 3 days). It finds each enrollment's current unbroken run of 'absent'
// entries ending at that enrollment's most recently recorded date, using the
// classic gaps-and-islands technique: subtracting a per-enrollment row number
// (ordered by date) from the date itself is constant within an unbroken run of
// consecutive calendar dates, which groups the run into one partition.
func (r *Repository) ConsecutiveAbsences(ctx context.Context, sectionID uuid.UUID, threshold int) ([]ConsecutiveAbsenceRow, error) {
	if threshold <= 0 {
		threshold = 3
	}
	out := []ConsecutiveAbsenceRow{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH ordered AS (
				SELECT ae.enrollment_id, ae.date, ae.status,
				       ae.date - (row_number() OVER (PARTITION BY ae.enrollment_id ORDER BY ae.date))::int AS grp
				FROM attendance_entries ae
				JOIN attendance_registers ar ON ar.id = ae.register_id
				WHERE ar.section_id = $1 AND ae.deleted_at IS NULL
			),
			runs AS (
				SELECT enrollment_id, grp, count(*) AS run_length, max(date) AS last_date,
				       bool_and(status = 'absent') AS all_absent
				FROM ordered
				GROUP BY enrollment_id, grp
			),
			latest_run_per_enrollment AS (
				SELECT DISTINCT ON (enrollment_id) enrollment_id, run_length, last_date, all_absent
				FROM runs
				ORDER BY enrollment_id, last_date DESC
			)
			SELECT enrollment_id, run_length, last_date
			FROM latest_run_per_enrollment
			WHERE all_absent AND run_length >= $2
			ORDER BY run_length DESC
		`, sectionID, threshold)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row ConsecutiveAbsenceRow
			if err := rows.Scan(&row.EnrollmentID, &row.ConsecutiveDays, &row.LastAbsentDate); err != nil {
				return err
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}
