package fees

import (
	"time"

	"github.com/google/uuid"
)

// StatutoryCategory is PRD 4.4.1's fixed enumeration -- every fee head must
// carry one, because the state fee determination committee's income filing
// is head-wise and this makes that export a query, not a manual
// reclassification exercise every filing cycle.
type StatutoryCategory string

const (
	CategoryTuition     StatutoryCategory = "tuition"
	CategorySpecial     StatutoryCategory = "special"
	CategoryLaboratory  StatutoryCategory = "laboratory"
	CategoryLibrary     StatutoryCategory = "library"
	CategoryComputer    StatutoryCategory = "computer"
	CategoryDevelopment StatutoryCategory = "development"
	CategoryTransport   StatutoryCategory = "transport"
	CategoryExamination StatutoryCategory = "examination"
	CategoryOther       StatutoryCategory = "other"
)

type FeeHead struct {
	ID                uuid.UUID         `json:"id"`
	AcademicYearID    uuid.UUID         `json:"academic_year_id"`
	Name              string            `json:"name"`
	StatutoryCategory StatutoryCategory `json:"statutory_category"`
	CreatedAt         time.Time         `json:"created_at"`
}

type FeeStructure struct {
	ID             uuid.UUID `json:"id"`
	AcademicYearID uuid.UUID `json:"academic_year_id"`
	ClassID        uuid.UUID `json:"class_id"`
	Version        int       `json:"version"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

type Instalment struct {
	ID          uuid.UUID `json:"id"`
	StructureID uuid.UUID `json:"structure_id"`
	FeeHeadID   uuid.UUID `json:"fee_head_id"`
	Label       string    `json:"label"`
	AmountPaise int64     `json:"amount_paise"`
	DueDate     time.Time `json:"due_date"`
}

// InstalmentInput is what a caller supplies when defining/replacing a
// structure's instalment schedule -- FeeHeadID by value (not a DB id lookup)
// since a structure is authored in one call, not one head at a time.
type InstalmentInput struct {
	FeeHeadID   uuid.UUID `json:"fee_head_id"`
	Label       string    `json:"label"`
	AmountPaise int64     `json:"amount_paise"`
	DueDate     string    `json:"due_date"` // YYYY-MM-DD
}

type ConcessionType string

const (
	ConcessionSibling    ConcessionType = "sibling"
	ConcessionStaffWard  ConcessionType = "staff_ward"
	ConcessionMerit      ConcessionType = "merit"
	ConcessionManagement ConcessionType = "management"
	ConcessionRTE        ConcessionType = "rte"
)

type ConcessionStatus string

const (
	ConcessionActive         ConcessionStatus = "active"
	ConcessionReviewRequired ConcessionStatus = "review_required"
	ConcessionCancelled      ConcessionStatus = "cancelled"
	ConcessionConverted      ConcessionStatus = "converted"
)

type Concession struct {
	ID               uuid.UUID        `json:"id"`
	StudentID        uuid.UUID        `json:"student_id"`
	ConcessionType   ConcessionType   `json:"concession_type"`
	FeeHeadID        *uuid.UUID       `json:"fee_head_id,omitempty"`
	Percentage       *float64         `json:"percentage,omitempty"`
	FlatAmountPaise  *int64           `json:"flat_amount_paise,omitempty"`
	ElderStudentID   *uuid.UUID       `json:"elder_student_id,omitempty"`
	DependentStaffID *uuid.UUID       `json:"dependent_staff_id,omitempty"`
	Status           ConcessionStatus `json:"status"`
	ApproverID       uuid.UUID        `json:"approver_id"`
	Reason           string           `json:"reason"`
	ResolutionNote   string           `json:"resolution_note,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}

type FeeAssignment struct {
	ID             uuid.UUID  `json:"id"`
	StudentID      uuid.UUID  `json:"student_id"`
	AcademicYearID uuid.UUID  `json:"academic_year_id"`
	StructureID    *uuid.UUID `json:"structure_id,omitempty"`
	IsRTE          bool       `json:"is_rte"`
	CreatedAt      time.Time  `json:"created_at"`
}

type LineItemStatus string

const (
	LineItemPending       LineItemStatus = "pending"
	LineItemPartiallyPaid LineItemStatus = "partially_paid"
	LineItemPaid          LineItemStatus = "paid"
	LineItemWaived        LineItemStatus = "waived"
)

type LineItem struct {
	ID                    uuid.UUID      `json:"id"`
	AssignmentID          uuid.UUID      `json:"assignment_id"`
	StudentID             uuid.UUID      `json:"student_id"`
	FeeHeadID             uuid.UUID      `json:"fee_head_id"`
	FeeHeadName           string         `json:"fee_head_name,omitempty"`
	Label                 string         `json:"label"`
	DueDate               time.Time      `json:"due_date"`
	GrossAmountPaise      int64          `json:"gross_amount_paise"`
	ConcessionAmountPaise int64          `json:"concession_amount_paise"`
	NetAmountPaise        int64          `json:"net_amount_paise"`
	PaidAmountPaise       int64          `json:"paid_amount_paise"`
	Status                LineItemStatus `json:"status"`
}

type PaymentMode string

const (
	ModeCash   PaymentMode = "cash"
	ModeCheque PaymentMode = "cheque"
	ModeUPI    PaymentMode = "upi"
	ModeCard   PaymentMode = "card"
)

type ChequeStatus string

const (
	ChequeReceived  ChequeStatus = "received"
	ChequeDeposited ChequeStatus = "deposited"
	ChequeCleared   ChequeStatus = "cleared"
	ChequeBounced   ChequeStatus = "bounced"
)

// AllocationInput is one caller-specified split of a payment across a line
// item -- the clerk (or the auto-allocation-by-due-date default) decides
// this, never the server unilaterally, per PRD 4.4.3.3's insistence that the
// receipt show an allocation the parent actually agreed to at the counter.
type AllocationInput struct {
	LineItemID  uuid.UUID `json:"line_item_id"`
	AmountPaise int64     `json:"amount_paise"`
}

type CollectPaymentInput struct {
	StudentID   uuid.UUID         `json:"student_id"`
	Mode        PaymentMode       `json:"mode"`
	AmountPaise int64             `json:"amount_paise"`
	Allocations []AllocationInput `json:"allocations"`
	// AdvanceAmountPaise is whatever part of AmountPaise the clerk explicitly
	// wants held as credit rather than allocated -- PRD 4.4.3.3's "the
	// clerk to keep the balance for next term" scenario. Must equal
	// AmountPaise minus the sum of Allocations; validated in the repository,
	// not silently recomputed, so a caller's arithmetic mistake surfaces as
	// an error rather than a mismatched receipt.
	AdvanceAmountPaise int64  `json:"advance_amount_paise"`
	ChequeNumber       string `json:"cheque_number,omitempty"`
	ChequeBank         string `json:"cheque_bank,omitempty"`
	DeviceID           string `json:"device_id,omitempty"`
}

type Payment struct {
	ID                   uuid.UUID    `json:"id"`
	StudentID            uuid.UUID    `json:"student_id"`
	ReceiptNumber        int64        `json:"receipt_number"`
	Mode                 PaymentMode  `json:"mode"`
	AmountPaise          int64        `json:"amount_paise"`
	AllocatedAmountPaise int64        `json:"allocated_amount_paise"`
	AdvanceAmountPaise   int64        `json:"advance_amount_paise"`
	CollectedBy          uuid.UUID    `json:"collected_by"`
	CollectedAt          time.Time    `json:"collected_at"`
	ChequeNumber         string       `json:"cheque_number,omitempty"`
	ChequeBank           string       `json:"cheque_bank,omitempty"`
	ChequeStatus         ChequeStatus `json:"cheque_status,omitempty"`
	IsVoid               bool         `json:"is_void"`
	VoidReason           string       `json:"void_reason,omitempty"`
}

type DuesRow struct {
	StudentID          uuid.UUID  `json:"student_id"`
	StudentName        string     `json:"student_name"`
	OutstandingPaise   int64      `json:"outstanding_paise"`
	CreditBalancePaise int64      `json:"credit_balance_paise"`
	OldestDueDate      *time.Time `json:"oldest_due_date,omitempty"`
	DaysOverdue        int        `json:"days_overdue"`
}
