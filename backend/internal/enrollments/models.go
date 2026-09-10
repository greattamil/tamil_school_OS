package enrollments

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive         Status = "active"
	StatusTransferredOut Status = "transferred_out"
	StatusDroppedOut     Status = "dropped_out"
	StatusGraduated      Status = "graduated"
)

type Enrollment struct {
	ID             uuid.UUID  `json:"id"`
	StudentID      uuid.UUID  `json:"student_id"`
	AcademicYearID uuid.UUID  `json:"academic_year_id"`
	ClassID        uuid.UUID  `json:"class_id"`
	SectionID      uuid.UUID  `json:"section_id"`
	RollNumber     *string    `json:"roll_number,omitempty"`
	Medium         string     `json:"medium"`
	GroupCode      string     `json:"group_code"`
	StartDate      time.Time  `json:"start_date"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	Status         Status     `json:"status"`
}

type CreateInput struct {
	StudentID      uuid.UUID
	AcademicYearID uuid.UUID
	ClassID        uuid.UUID
	SectionID      uuid.UUID
	RollNumber     *string
	Medium         string
	GroupCode      string
	StartDate      time.Time
}
