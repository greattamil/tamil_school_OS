package fees

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

// GatewayVerifier checks a webhook payload's signature against the
// provider's scheme (HMAC of the raw body against a per-school secret, for
// most Indian payment gateways) and extracts the fields needed to match it
// to a demand. The only implementation here is LogGatewayVerifier: this
// environment has no real gateway credentials (see PROGRESS.md -- payment
// gateway onboarding is an external business process, not something buildable
// from code), so real signature verification is stubbed exactly like
// notify.LogDispatcher/LogSMSSender -- the ingestion pipeline around it
// (idempotency, the exceptions queue, async processing) is real regardless of
// which verifier is plugged in.
type GatewayVerifier interface {
	// Verify returns whether the signature is valid and, if so, the
	// provider's own idempotency key extracted from the body.
	Verify(ctx context.Context, provider string, headers http.Header, rawBody []byte) (valid bool, idempotencyKey string)
}

type LogGatewayVerifier struct{}

func (LogGatewayVerifier) Verify(ctx context.Context, provider string, headers http.Header, rawBody []byte) (bool, string) {
	var payload struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		log.Printf("fees: [stub gateway verify] provider=%s unparseable payload", provider)
		return false, ""
	}
	log.Printf("fees: [stub gateway verify] provider=%s accepting unverified (no real gateway credentials configured)", provider)
	return true, payload.IdempotencyKey
}

// RegisterWebhookRoutes wires the payment-gateway webhook endpoint onto the
// PUBLIC mux (no user session -- the caller is an external gateway server,
// authenticated by its signature, not a JWT), unlike every other fees route.
func RegisterWebhookRoutes(mux *http.ServeMux, repo *Repository, verifier GatewayVerifier) {
	h := &webhookHandlers{repo: repo, verifier: verifier}
	mux.HandleFunc("POST /api/v1/webhooks/payments/{school_id}/{provider}", h.receive)
}

type webhookHandlers struct {
	repo     *Repository
	verifier GatewayVerifier
}

// receive answers PRD 4.4.3.2: "All inbound payment notifications land in a
// webhook_events table before any business logic runs." The school is
// identified from the URL path (the gateway account is registered per
// school, PRD 4.4.3), so real tenant context can be set before this row is
// ever written -- unlike a genuinely cross-school scan, there's no ambiguity
// to resolve later. Idempotency is enforced by the UNIQUE(school_id,
// provider, idempotency_key) constraint, not an application check: a
// duplicate delivery hits that constraint and this handler still returns 200
// (so the provider stops retrying) without processing twice.
func (h *webhookHandlers) receive(w http.ResponseWriter, r *http.Request) {
	schoolID, err := uuid.Parse(r.PathValue("school_id"))
	if err != nil {
		http.Error(w, "invalid school id", http.StatusBadRequest)
		return
	}
	provider := r.PathValue("provider")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	valid, idempotencyKey := h.verifier.Verify(r.Context(), provider, r.Header, body)
	if idempotencyKey == "" {
		// PRD 4.4.3.2 doesn't ask this handler to reject a key-less payload
		// outright, but idempotency is meaningless without one -- fall back
		// to a request-scoped value so it's at least never silently dropped
		// nor collides with another event.
		idempotencyKey = uuid.NewString()
	}

	ctx := tenancy.WithSchoolID(r.Context(), schoolID)
	err = db.WithTenantTx(ctx, h.repo.pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO webhook_events (school_id, provider, idempotency_key, raw_payload, signature_valid)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (school_id, provider, idempotency_key) DO NOTHING
		`, schoolID, provider, idempotencyKey, body, valid)
		return err
	})
	if err != nil {
		log.Printf("fees: webhook ingest failed: %v", err)
		http.Error(w, "could not record event", http.StatusInternalServerError)
		return
	}

	// 200 regardless of signature validity or match outcome -- an invalid
	// signature is recorded (signature_valid=false) for the office to see,
	// never surfaced back to the caller in a way that would help an attacker
	// iterate on forging one. Matching to a specific payment is deliberately
	// NOT done here: PRD 4.4.3.2 requires this to be asynchronous processing
	// ("so a slow allocation never causes the provider to time out and
	// retry"), which is exactly what a not-yet-built job-queue consumer of
	// unprocessed webhook_events rows would be -- not implemented, since
	// there is no real gateway payload shape to parse against yet.
	w.WriteHeader(http.StatusOK)
}
