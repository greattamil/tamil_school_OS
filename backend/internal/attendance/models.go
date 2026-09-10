package attendance

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPresent Status = "present"
	StatusAbsent  Status = "absent"
)

type Reason string

const (
	ReasonSick      Reason = "sick"
	ReasonPermitted Reason = "permitted"
	ReasonUnexcused Reason = "unexcused"
)

// EntryEdit is one student's edit within a sync request (PRD 4.2.5: "A teacher
// syncing 50 students sends 50 independent edits, each declaring its own base
// revision").
type EntryEdit struct {
	EnrollmentID uuid.UUID `json:"enrollment_id"`
	Status       Status    `json:"status"`
	Reason       *Reason   `json:"reason,omitempty"`
	DeviceID     string    `json:"device_id"`
	LocalCounter int64     `json:"local_counter"`
	ClientTime   time.Time `json:"client_timestamp"`
	BaseRevision int       `json:"base_revision"` // 0 for an entry the client has never synced before
}

type Outcome string

const (
	OutcomeApplied    Outcome = "applied"
	OutcomeSuperseded Outcome = "superseded"
	OutcomeInvalid    Outcome = "invalid"
)

// EntryResult is returned per entry, never as a single batch status (PRD 4.2.5
// point 3): a rejection tells the caller which student, why, and what the current
// server value is, so the app can show it rather than silently dropping the edit.
type EntryResult struct {
	EnrollmentID   uuid.UUID `json:"enrollment_id"`
	Outcome        Outcome   `json:"outcome"`
	ServerRevision int       `json:"server_revision"`
	CurrentStatus  Status    `json:"current_status"`
	CurrentReason  *Reason   `json:"current_reason,omitempty"`
	Message        string    `json:"message,omitempty"`
}

type SyncResult struct {
	RegisterID uuid.UUID     `json:"register_id"`
	Results    []EntryResult `json:"results"`
}
