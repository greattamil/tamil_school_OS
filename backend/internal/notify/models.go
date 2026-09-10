package notify

import (
	"time"

	"github.com/google/uuid"
)

type Kind string

const (
	KindNotice                Kind = "notice"
	KindAbsenceAlert          Kind = "absence_alert"
	KindFeeDueReminder        Kind = "fee_due_reminder"
	KindFeeOverdueReminder    Kind = "fee_overdue_reminder"
	KindPaymentReceipt        Kind = "payment_receipt"
	KindReportCardPublished   Kind = "report_card_published"
	KindExamSchedulePublished Kind = "exam_schedule_published"
	KindEmergencyBroadcast    Kind = "emergency_broadcast"
)

// TargetType selects who a composed notice reaches (PRD 4.5.1: "whole school,
// class, section, individual student, or a custom list").
type TargetType string

const (
	TargetWholeSchool TargetType = "whole_school"
	TargetClass       TargetType = "class"
	TargetSection     TargetType = "section"
	TargetStudent     TargetType = "student"
	TargetCustom      TargetType = "custom" // explicit list of user IDs
)

type ComposeInput struct {
	Kind          Kind
	Title         string
	BodyEN        string
	BodyTA        string
	TargetType    TargetType
	ClassID       *uuid.UUID
	SectionID     *uuid.UUID
	StudentID     *uuid.UUID
	CustomUserIDs []uuid.UUID
	IsEmergency   bool
}

type Notification struct {
	ID                uuid.UUID `json:"id"`
	Kind              Kind      `json:"kind"`
	Title             string    `json:"title"`
	BodyEN            string    `json:"body_en"`
	BodyTA            string    `json:"body_ta,omitempty"`
	TargetDescription string    `json:"target_description,omitempty"`
	IsEmergency       bool      `json:"is_emergency"`
	RecipientCount    int       `json:"recipient_count"`
	CreatedAt         time.Time `json:"created_at"`
}

// InboxItem is a notification as it appears in a recipient's own feed (PRD 4.5.1:
// "Delivery and read tracking per recipient" -- read_at is per this recipient row,
// not shared across everyone the notice went to).
type InboxItem struct {
	NotificationID uuid.UUID  `json:"notification_id"`
	Kind           Kind       `json:"kind"`
	Title          string     `json:"title"`
	BodyEN         string     `json:"body_en"`
	BodyTA         string     `json:"body_ta,omitempty"`
	StudentID      *uuid.UUID `json:"student_id,omitempty"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
