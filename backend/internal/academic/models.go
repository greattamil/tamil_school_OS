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
