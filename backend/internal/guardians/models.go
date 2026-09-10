package guardians

import (
	"time"

	"github.com/google/uuid"
)

type Guardian struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Mobile     string    `json:"mobile"`
	Email      *string   `json:"email,omitempty"`
	Occupation *string   `json:"occupation,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Relationship string

const (
	RelationFather   Relationship = "father"
	RelationMother   Relationship = "mother"
	RelationGuardian Relationship = "guardian"
	RelationOther    Relationship = "other"
)

type StudentGuardianLink struct {
	StudentID        uuid.UUID    `json:"student_id"`
	GuardianID       uuid.UUID    `json:"guardian_id"`
	Relationship     Relationship `json:"relationship"`
	IsPrimaryContact bool         `json:"is_primary_contact"`
	IsFeeResponsible bool         `json:"is_fee_responsible"`
}
