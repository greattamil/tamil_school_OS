package staff

import (
	"time"

	"github.com/google/uuid"
)

type Staff struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Designation   *string    `json:"designation,omitempty"`
	Qualification *string    `json:"qualification,omitempty"`
	DateOfJoining *time.Time `json:"date_of_joining,omitempty"`
	EMISStaffID   *string    `json:"emis_staff_id,omitempty"`
	IsTeaching    bool       `json:"is_teaching"`
	CreatedAt     time.Time  `json:"created_at"`
}

type CreateStaffInput struct {
	Name          string
	Designation   *string
	Qualification *string
	DateOfJoining *time.Time
	IsTeaching    bool
}
