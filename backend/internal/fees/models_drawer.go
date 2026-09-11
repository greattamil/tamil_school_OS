package fees

import (
	"time"

	"github.com/google/uuid"
)

type DrawerStatus string

const (
	DrawerOpen   DrawerStatus = "open"
	DrawerClosed DrawerStatus = "closed"
)

// Denomination is PRD 4.4.6.1's "blind count": counts of each note/coin
// value, entered before the system reveals its own expected total.
type Denomination struct {
	Note500 int `json:"note_500"`
	Note200 int `json:"note_200"`
	Note100 int `json:"note_100"`
	Note50  int `json:"note_50"`
	Note20  int `json:"note_20"`
	Note10  int `json:"note_10"`
	Coins   int `json:"coins_paise"` // coin total in paise, not a count -- coin denominations vary too much to enumerate
}

func (d Denomination) TotalPaise() int64 {
	return int64(d.Note500)*50000 + int64(d.Note200)*20000 + int64(d.Note100)*10000 +
		int64(d.Note50)*5000 + int64(d.Note20)*2000 + int64(d.Note10)*1000 + int64(d.Coins)
}

type CashDrawerClosing struct {
	ID                    uuid.UUID     `json:"id"`
	CollectorID           uuid.UUID     `json:"collector_id"`
	BusinessDate          time.Time     `json:"business_date"`
	Status                DrawerStatus  `json:"status"`
	Denomination          *Denomination `json:"denomination_breakdown,omitempty"`
	CountedTotalPaise     *int64        `json:"counted_total_paise,omitempty"`
	ExpectedTotalPaise    *int64        `json:"expected_total_paise,omitempty"`
	VariancePaise         *int64        `json:"variance_paise,omitempty"`
	RecountNumber         int           `json:"recount_number"`
	VarianceExplanation   string        `json:"variance_explanation,omitempty"`
	ClosedAt              *time.Time    `json:"closed_at,omitempty"`
	VoidedReceiptsInRange []Payment     `json:"voided_receipts,omitempty"`
}
