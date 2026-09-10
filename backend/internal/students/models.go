package students

import (
	"time"

	"github.com/google/uuid"
)

type Student struct {
	ID              uuid.UUID `json:"id"`
	AdmissionNumber string    `json:"admission_number"`
	NameEnglish     string    `json:"name_english"`
	NameTamil       *string   `json:"name_tamil,omitempty"`
	DateOfBirth     time.Time `json:"date_of_birth"`
	Gender          string    `json:"gender"`
	MotherTongue    *string   `json:"mother_tongue,omitempty"`
	BloodGroup      *string   `json:"blood_group,omitempty"`
	Address         *string   `json:"address,omitempty"`
	PreviousSchool  *string   `json:"previous_school,omitempty"`
	RTEQuota        bool      `json:"rte_quota"`
	PENNumber       *string   `json:"pen_number,omitempty"`
	APAARID         *string   `json:"apaar_id,omitempty"`
	EMISNumber      *string   `json:"emis_number,omitempty"`
	AdmissionDate   time.Time `json:"admission_date"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateStudentInput struct {
	AdmissionNumber string
	NameEnglish     string
	NameTamil       *string
	DateOfBirth     time.Time
	Gender          string
	MotherTongue    *string
	BloodGroup      *string
	Address         *string
	PreviousSchool  *string
	RTEQuota        bool
	AdmissionDate   time.Time
}
