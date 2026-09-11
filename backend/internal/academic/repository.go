package academic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/audit"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var (
	ErrNotFound          = errors.New("academic: not found")
	ErrDuplicateLabel    = errors.New("academic: academic year label already exists")
	ErrInvalidTransition = errors.New("academic: invalid state transition")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateYear(ctx context.Context, label string, start, end any) (AcademicYear, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return AcademicYear{}, db.ErrNoTenant
	}

	var out AcademicYear
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO academic_years (school_id, label, start_date, end_date)
			VALUES ($1, $2, $3, $4)
			RETURNING id, label, start_date, end_date, state, created_at
		`, schoolID, label, start, end).Scan(&out.ID, &out.Label, &out.StartDate, &out.EndDate, &out.State, &out.CreatedAt)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return AcademicYear{}, ErrDuplicateLabel
		}
		return AcademicYear{}, err
	}
	return out, nil
}

func (r *Repository) ListYears(ctx context.Context) ([]AcademicYear, error) {
	out := []AcademicYear{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, label, start_date, end_date, state, created_at
			FROM academic_years
			WHERE deleted_at IS NULL
			ORDER BY start_date DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var y AcademicYear
			if err := rows.Scan(&y.ID, &y.Label, &y.StartDate, &y.EndDate, &y.State, &y.CreatedAt); err != nil {
				return err
			}
			out = append(out, y)
		}
		return rows.Err()
	})
	return out, err
}

// allowedTransitions encodes the year lifecycle from PRD 3.1: active -> soft_closing
// -> locked, with locked -> active reserved for the deliberate reopening flow.
var allowedTransitions = map[YearState][]YearState{
	YearActive:      {YearSoftClosing},
	YearSoftClosing: {YearLocked, YearActive},
	YearLocked:      {YearActive}, // reopening: correspondent-only, reason mandatory, audited (enforced by caller)
}

// TransitionState moves an academic year to newState, recording who did it, when,
// and why in both the row itself and the audit log, in the same transaction
// (PRD 3.1: "transitions are recorded with who made them and when"; reopening a
// locked year "carries a mandatory reason ... and writes a prominent audit entry").
// Role/re-authentication enforcement for reopening a locked year happens in the
// HTTP handler, not here.
func (r *Repository) TransitionState(ctx context.Context, yearID uuid.UUID, newState YearState, reason string, actorUserID uuid.UUID) error {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return db.ErrNoTenant
	}

	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		var current YearState
		if err := tx.QueryRow(ctx, `SELECT state FROM academic_years WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, yearID).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		allowed := false
		for _, s := range allowedTransitions[current] {
			if s == newState {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrInvalidTransition
		}

		if _, err := tx.Exec(ctx, `
			UPDATE academic_years
			SET state = $1, state_changed_at = now(), state_changed_by = $2, state_change_reason = $3
			WHERE id = $4
		`, newState, actorUserID, reason, yearID); err != nil {
			return err
		}

		return audit.Write(ctx, tx, schoolID, audit.Entry{
			ActorUserID: actorUserID,
			Action:      "academic_year.state_transition",
			EntityType:  "academic_year",
			EntityID:    yearID,
			Before:      map[string]any{"state": current},
			After:       map[string]any{"state": newState},
			Reason:      reason,
		})
	})
}

func (r *Repository) CreateClass(ctx context.Context, academicYearID uuid.UUID, name string, sequence int) (Class, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Class{}, db.ErrNoTenant
	}

	var out Class
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO classes (school_id, academic_year_id, name, sequence)
			VALUES ($1, $2, $3, $4)
			RETURNING id, academic_year_id, name, sequence
		`, schoolID, academicYearID, name, sequence).Scan(&out.ID, &out.AcademicYearID, &out.Name, &out.Sequence)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Class{}, ErrDuplicateLabel
		}
		return Class{}, err
	}
	return out, nil
}

func (r *Repository) ListClasses(ctx context.Context, academicYearID uuid.UUID) ([]Class, error) {
	out := []Class{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, academic_year_id, name, sequence
			FROM classes
			WHERE academic_year_id = $1 AND deleted_at IS NULL
			ORDER BY sequence
		`, academicYearID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Class
			if err := rows.Scan(&c.ID, &c.AcademicYearID, &c.Name, &c.Sequence); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) CreateSection(ctx context.Context, classID uuid.UUID, name string) (Section, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return Section{}, db.ErrNoTenant
	}

	var out Section
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO sections (school_id, class_id, name)
			VALUES ($1, $2, $3)
			RETURNING id, class_id, name
		`, schoolID, classID, name).Scan(&out.ID, &out.ClassID, &out.Name)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Section{}, ErrDuplicateLabel
		}
		return Section{}, err
	}
	return out, nil
}

func (r *Repository) ListSections(ctx context.Context, classID uuid.UUID) ([]Section, error) {
	out := []Section{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, class_id, name
			FROM sections
			WHERE class_id = $1 AND deleted_at IS NULL
			ORDER BY name
		`, classID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s Section
			if err := rows.Scan(&s.ID, &s.ClassID, &s.Name); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

// FindSectionByName resolves a class+section by human-readable names, used by bulk
// import which works from a spreadsheet's text columns rather than UUIDs.
func (r *Repository) FindSectionByName(ctx context.Context, academicYearID uuid.UUID, className, sectionName string) (Section, error) {
	var out Section
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT s.id, s.class_id, s.name
			FROM sections s
			JOIN classes c ON c.id = s.class_id
			WHERE c.academic_year_id = $1 AND c.name = $2 AND s.name = $3
			  AND c.deleted_at IS NULL AND s.deleted_at IS NULL
		`, academicYearID, className, sectionName).Scan(&out.ID, &out.ClassID, &out.Name)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Section{}, ErrNotFound
		}
		return Section{}, err
	}
	return out, nil
}

// SetCalendarDays upserts one day_type per date (PRD 3.1: "the office sets the
// year's calendar at the start and amends it as holidays are declared" --
// amending is exactly what the ON CONFLICT UPDATE is for, since a date already
// classified, e.g. regular_working, is routinely reclassified to holiday later).
// No separate recomputation step is needed for the "declaring a rain holiday
// retroactively ... recomputation of affected percentages happens automatically"
// requirement: AttendancePercentage reads this table live on every call.
func (r *Repository) SetCalendarDays(ctx context.Context, days []CalendarDay, updatedBy uuid.UUID) error {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return db.ErrNoTenant
	}
	return db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, d := range days {
			batch.Queue(`
				INSERT INTO academic_calendar_days (school_id, date, day_type, note, updated_by)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (school_id, date) DO UPDATE
				SET day_type = EXCLUDED.day_type, note = EXCLUDED.note,
				    updated_by = EXCLUDED.updated_by, updated_at = now()
			`, schoolID, d.Date, d.DayType, nullIfEmptyString(d.Note), updatedBy)
		}
		br := tx.SendBatch(ctx, batch)
		defer br.Close()
		for range days {
			if _, err := br.Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListCalendarDays returns every explicitly-set day in [from, to], ordered by
// date. A date in range with no row here has never been classified -- the
// caller (e.g. AttendancePercentage) treats an entirely-unset range as "no
// calendar configured yet" and falls back accordingly, rather than assuming
// every unlisted date is a working day.
func (r *Repository) ListCalendarDays(ctx context.Context, from, to any) ([]CalendarDay, error) {
	out := []CalendarDay{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT date, day_type, COALESCE(note, '')
			FROM academic_calendar_days
			WHERE date BETWEEN $1 AND $2
			ORDER BY date
		`, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d CalendarDay
			if err := rows.Scan(&d.Date, &d.DayType, &d.Note); err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func nullIfEmptyString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
