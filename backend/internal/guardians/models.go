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
	NotificationPreferences
}

// NotificationPreferences answers PRD 4.5.2: "Per-guardian channel
// preference and per-category opt-out (fee reminders, attendance, general
// notices), which is also a DPDP consent requirement." Emergency broadcasts
// (PRD 4.5.3) are never subject to any of these -- that exemption is
// enforced where those are sent, not by adding a field here that would
// invite someone to wire it in by mistake.
type NotificationPreferences struct {
	OptOutFeeReminders     bool `json:"opt_out_fee_reminders"`
	OptOutAttendanceAlerts bool `json:"opt_out_attendance_alerts"`
	OptOutGeneralNotices   bool `json:"opt_out_general_notices"`
	SMSOptOut              bool `json:"sms_opt_out"`
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
