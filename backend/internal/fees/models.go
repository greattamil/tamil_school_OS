// Package fees is deliberately narrow in Phase 2: a read-only dues snapshot fed
// by a simple import, not the fee engine (PRD section 9, Phase 2 scope note: "It
// is a read-only window, not the fee engine: no collection, no allocation, no
// receipts. That keeps the full fee module in Phase 3 where it belongs while
// giving parents a reason to open the app from day one.").
package fees

import (
	"time"

	"github.com/google/uuid"
)

type DuesSnapshot struct {
	StudentID              uuid.UUID  `json:"student_id"`
	StudentName            string     `json:"student_name"`
	AdmissionNumber        string     `json:"admission_number"`
	OutstandingAmountPaise int64      `json:"outstanding_amount_paise"`
	DueDate                *time.Time `json:"due_date,omitempty"`
	LastPaymentDate        *time.Time `json:"last_payment_date,omitempty"`
	LastPaymentAmountPaise *int64     `json:"last_payment_amount_paise,omitempty"`
	ImportedAt             time.Time  `json:"imported_at"`
}
