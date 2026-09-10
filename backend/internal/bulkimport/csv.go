package bulkimport

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
)

// requiredColumns must be present in the header; everything else is optional.
var requiredColumns = []string{
	"admission_number", "name_english", "gender", "date_of_birth",
	"admission_date", "class_name", "section_name",
}

// parseCSV reads the whole file into rows keyed by header name, so column order in
// the spreadsheet doesn't matter -- only the header names do.
func parseCSV(r io.Reader) ([]row, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	// See the matching comment in internal/fees/repository.go: a row with the
	// wrong field count must become a per-row validation error, not abort every
	// row after it.
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	col := make(map[string]int, len(header))
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}

	for _, required := range requiredColumns {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
	}

	get := func(record []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	var rows []row
	lineNumber := 1 // header was line 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row after line %d: %w", lineNumber, err)
		}
		lineNumber++

		rows = append(rows, row{
			lineNumber:           lineNumber,
			admissionNumber:      get(record, "admission_number"),
			nameEnglish:          get(record, "name_english"),
			nameTamil:            get(record, "name_tamil"),
			dateOfBirth:          get(record, "date_of_birth"),
			gender:               get(record, "gender"),
			motherTongue:         get(record, "mother_tongue"),
			bloodGroup:           get(record, "blood_group"),
			address:              get(record, "address"),
			previousSchool:       get(record, "previous_school"),
			rteQuota:             get(record, "rte_quota"),
			admissionDate:        get(record, "admission_date"),
			className:            get(record, "class_name"),
			sectionName:          get(record, "section_name"),
			rollNumber:           get(record, "roll_number"),
			medium:               get(record, "medium"),
			groupCode:            get(record, "group_code"),
			guardianName:         get(record, "guardian_name"),
			guardianMobile:       get(record, "guardian_mobile"),
			guardianRelationship: get(record, "guardian_relationship"),
		})
	}

	return rows, nil
}

const dateLayout = "2006-01-02"

// validateRow performs static, per-row validation that does not require a
// database round trip (required fields, date formats, boolean parsing). Rows that
// pass this step are still checked against the database (duplicate admission
// number, unknown class/section) during commit.
func validateRow(r row) []RowError {
	var errs []RowError

	if r.admissionNumber == "" {
		errs = append(errs, RowError{r.lineNumber, "admission_number", "required"})
	}
	if r.nameEnglish == "" {
		errs = append(errs, RowError{r.lineNumber, "name_english", "required"})
	}
	if r.gender == "" {
		errs = append(errs, RowError{r.lineNumber, "gender", "required"})
	}
	if r.className == "" {
		errs = append(errs, RowError{r.lineNumber, "class_name", "required"})
	}
	if r.sectionName == "" {
		errs = append(errs, RowError{r.lineNumber, "section_name", "required"})
	}
	if _, err := time.Parse(dateLayout, r.dateOfBirth); err != nil {
		errs = append(errs, RowError{r.lineNumber, "date_of_birth", "must be YYYY-MM-DD"})
	}
	if _, err := time.Parse(dateLayout, r.admissionDate); err != nil {
		errs = append(errs, RowError{r.lineNumber, "admission_date", "must be YYYY-MM-DD"})
	}
	if r.rteQuota != "" && !isBooleanish(r.rteQuota) {
		errs = append(errs, RowError{r.lineNumber, "rte_quota", "must be true/false/yes/no/1/0"})
	}
	if r.guardianMobile != "" && r.guardianName == "" {
		errs = append(errs, RowError{r.lineNumber, "guardian_name", "required when guardian_mobile is set"})
	}

	return errs
}

func isBooleanish(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "false", "yes", "no", "1", "0":
		return true
	default:
		return false
	}
}

func parseBooleanish(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}
