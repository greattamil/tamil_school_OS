// Package notify handles notices (PRD 4.5.1), automated notifications (PRD 4.5.3)
// and push dispatch (PRD 8.4).
package notify

import (
	"context"
	"log"
)

// PushResult is one token's outcome from a dispatch attempt, mirroring what FCM's
// real multicast API returns per PRD 8.4: "Per-token failures returned by FCM
// (unregistered, invalid) are handled individually and stale tokens pruned,
// without failing the batch."
type PushResult struct {
	Token   string
	Sent    bool
	Invalid bool // token is permanently bad (unregistered/invalid) and should be pruned
}

// Dispatcher sends push notifications. The only implementation here is
// LogDispatcher: this environment has no real Firebase project credentials, so
// the actual FCM SDK call is stubbed, but everything around it -- the job queue,
// targeting resolution, batching up to 500 tokens per PRD 8.4, delivery tracking,
// per-token failure handling -- is real and independent of which Dispatcher is
// plugged in. Swapping in a real FCM-backed Dispatcher is the only work left to
// make push delivery actually reach a device.
type Dispatcher interface {
	// SendMulticast sends one message to up to 500 tokens in a single logical
	// call (PRD 8.4: "Use FCM's multicast send, batching up to 500 registration
	// tokens per request"). Callers are responsible for chunking larger token
	// sets into multiple calls.
	SendMulticast(ctx context.Context, tokens []string, title, body string) []PushResult
}

type LogDispatcher struct{}

func (LogDispatcher) SendMulticast(ctx context.Context, tokens []string, title, body string) []PushResult {
	results := make([]PushResult, len(tokens))
	for i, t := range tokens {
		log.Printf("notify: [stub FCM dispatch] token=%s title=%q body=%q", t, title, body)
		results[i] = PushResult{Token: t, Sent: true}
	}
	return results
}
