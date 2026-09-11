package academic

import (
	"time"

	"github.com/google/uuid"
)

type YearState string

const (
	YearActive      YearState = "active"
	YearSoftClosing YearState = "soft_closing"
	YearLocked      YearState = "locked"
)

type AcademicYear struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	State     YearState `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

type Class struct {
	ID             uuid.UUID `json:"id"`
	AcademicYearID uuid.UUID `json:"academic_year_id"`
	Name           string    `json:"name"`
	Sequence       int       `json:"sequence"`
}

type Section struct {
	ID      uuid.UUID `json:"id"`
	ClassID uuid.UUID `json:"class_id"`
	Name    string    `json:"name"`
}

// DayType is PRD 3.1's academic_calendar_days classification: "All working-day
// totals, attendance percentages and register exports compute from these rows,
// never from the calendar week."
type DayType string

const (
	DayRegularWorking      DayType = "regular_working"
	DayHoliday             DayType = "holiday"
	DayCompensatoryWorking DayType = "compensatory_working"
	DayHalf                DayType = "half_day"
	DayExam                DayType = "exam_day"
)

// IsWorking reports whether attendance is expected to be taken on a day of this
// type -- used both to build the working-day denominator for percentages and,
// eventually, to gate whether the mobile app's section list even shows a
// register-taking action for a given date.
func (d DayType) IsWorking() bool {
	return d == DayRegularWorking || d == DayCompensatoryWorking || d == DayHalf || d == DayExam
}

type CalendarDay struct {
	Date    time.Time `json:"date"`
	DayType DayType   `json:"day_type"`
	Note    string    `json:"note,omitempty"`
}
