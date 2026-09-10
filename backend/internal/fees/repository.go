package fees

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

var ErrNotFound = errors.New("fees: not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type ImportRowError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	RowsImported int              `json:"rows_imported"`
	Errors       []ImportRowError `json:"errors"`
}

// ImportCSV replaces each named student's dues snapshot wholesale (PRD 4.2 Phase
// 2 scope: "a simple import of the school's existing fee position"). Expected
// columns: admission_number, outstanding_amount, due_date, last_payment_date,
// last_payment_amount. Amounts are rupees with an optional decimal point in the
// file and are converted to integer paise on the way in (PRD 3.3: money is never
// a float once it's in this system).
func (r *Repository) ImportCSV(ctx context.Context, importedBy uuid.UUID, file io.Reader) (ImportResult, error) {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return ImportResult{}, db.ErrNoTenant
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	// A row with fewer or more fields than the header must not abort the whole
	// file -- one malformed line (a common real-world occurrence: trailing
	// commas dropped by whatever produced the export) would otherwise silently
	// discard every row after it. Disabling the field-count check lets a short
	// row through as missing trailing values, which the existing per-field
	// required-value checks below already catch and report per row.
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return ImportResult{}, fmt.Errorf("read header: %w", err)
	}
	col := make(map[string]int, len(header))
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"admission_number", "outstanding_amount"} {
		if _, ok := col[required]; !ok {
			return ImportResult{}, fmt.Errorf("missing required column %q", required)
		}
	}

	result := ImportResult{Errors: []ImportRowError{}}
	line := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ImportResult{}, fmt.Errorf("read row after line %d: %w", line, err)
		}
		line++

		get := func(name string) string {
			i, ok := col[name]
			if !ok || i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}

		admissionNumber := get("admission_number")
		outstandingPaise, err := rupeesToPaise(get("outstanding_amount"))
		if err != nil {
			result.Errors = append(result.Errors, ImportRowError{line, "outstanding_amount: " + err.Error()})
			continue
		}

		var dueDate, lastPaymentDate *time.Time
		if v := get("due_date"); v != "" {
			d, err := time.Parse("2006-01-02", v)
			if err != nil {
				result.Errors = append(result.Errors, ImportRowError{line, "due_date must be YYYY-MM-DD"})
				continue
			}
			dueDate = &d
		}
		if v := get("last_payment_date"); v != "" {
			d, err := time.Parse("2006-01-02", v)
			if err != nil {
				result.Errors = append(result.Errors, ImportRowError{line, "last_payment_date must be YYYY-MM-DD"})
				continue
			}
			lastPaymentDate = &d
		}
		var lastPaymentPaise *int64
		if v := get("last_payment_amount"); v != "" {
			p, err := rupeesToPaise(v)
			if err != nil {
				result.Errors = append(result.Errors, ImportRowError{line, "last_payment_amount: " + err.Error()})
				continue
			}
			lastPaymentPaise = &p
		}

		err = db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
			var studentID uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT id FROM students WHERE school_id = $1 AND admission_number = $2 AND deleted_at IS NULL`, schoolID, admissionNumber).Scan(&studentID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO fee_dues_snapshot (school_id, student_id, outstanding_amount_paise, due_date, last_payment_date, last_payment_amount_paise, imported_by, imported_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,now())
				ON CONFLICT (school_id, student_id) DO UPDATE SET
					outstanding_amount_paise = EXCLUDED.outstanding_amount_paise,
					due_date = EXCLUDED.due_date,
					last_payment_date = EXCLUDED.last_payment_date,
					last_payment_amount_paise = EXCLUDED.last_payment_amount_paise,
					imported_by = EXCLUDED.imported_by,
					imported_at = now()
			`, schoolID, studentID, outstandingPaise, dueDate, lastPaymentDate, lastPaymentPaise, importedBy)
			return err
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				result.Errors = append(result.Errors, ImportRowError{line, "no student with admission number " + admissionNumber})
			} else {
				result.Errors = append(result.Errors, ImportRowError{line, "could not save: " + err.Error()})
			}
			continue
		}
		result.RowsImported++
	}

	return result, nil
}

func rupeesToPaise(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("required")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errors.New("must be a number")
	}
	return int64(math.Round(v * 100)), nil
}

// ForStudent returns the current dues snapshot for one student -- what the
// parent dashboard shows (PRD 4.8: "child's ... outstanding dues with due date").
func (r *Repository) ForStudent(ctx context.Context, studentID uuid.UUID) (DuesSnapshot, error) {
	var out DuesSnapshot
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT s.id, s.name_english, s.admission_number, f.outstanding_amount_paise,
			       f.due_date, f.last_payment_date, f.last_payment_amount_paise, f.imported_at
			FROM fee_dues_snapshot f
			JOIN students s ON s.id = f.student_id
			WHERE f.student_id = $1
		`, studentID).Scan(&out.StudentID, &out.StudentName, &out.AdmissionNumber, &out.OutstandingAmountPaise,
			&out.DueDate, &out.LastPaymentDate, &out.LastPaymentAmountPaise, &out.ImportedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DuesSnapshot{}, ErrNotFound
		}
		return DuesSnapshot{}, err
	}
	return out, nil
}

// List answers the office's view: every student with an outstanding balance,
// largest first (a lightweight stand-in for the full ageing/dues report that
// arrives with the Phase 3 fee engine).
func (r *Repository) List(ctx context.Context) ([]DuesSnapshot, error) {
	out := []DuesSnapshot{}
	err := db.WithTenantTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT s.id, s.name_english, s.admission_number, f.outstanding_amount_paise,
			       f.due_date, f.last_payment_date, f.last_payment_amount_paise, f.imported_at
			FROM fee_dues_snapshot f
			JOIN students s ON s.id = f.student_id
			WHERE f.outstanding_amount_paise > 0
			ORDER BY f.outstanding_amount_paise DESC
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row DuesSnapshot
			if err := rows.Scan(&row.StudentID, &row.StudentName, &row.AdmissionNumber, &row.OutstandingAmountPaise,
				&row.DueDate, &row.LastPaymentDate, &row.LastPaymentAmountPaise, &row.ImportedAt); err != nil {
				return err
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}
