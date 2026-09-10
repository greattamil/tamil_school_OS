// Package httpmw holds cross-cutting HTTP middleware: authentication and tenant
// context attachment. Every handler that touches tenant data must sit behind
// RequireAuth so that tenancy.SchoolID(ctx) is always derived from the validated
// token, never from a URL or body parameter (PRD 6.1, 6.3).
package httpmw

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/auth"
	"school-erp/backend/internal/db"
	"school-erp/backend/internal/tenancy"
)

// RequireAuth validates the bearer access token and, if it carries revocation
// checks correctly (tokens_valid_after), attaches user/school/role to the request
// context. On a SESSION_REVOKED condition it returns 401 with that reason code so
// the mobile client treats it as a wipe signal, not a plain logout (PRD 6.2).
func RequireAuth(pool *pgxpool.Pool, issuer *auth.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				unauthorized(w, "missing_token", "missing bearer token")
				return
			}

			claims, err := issuer.ParseAccessToken(raw)
			if err != nil {
				unauthorized(w, "invalid_token", "invalid or expired token")
				return
			}

			revoked, err := tokensRevokedSince(r.Context(), pool, claims.UserID, claims.IssuedAt.Time)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if revoked {
				unauthorized(w, "SESSION_REVOKED", "session has been revoked")
				return
			}

			ctx := tenancy.WithUserID(r.Context(), claims.UserID)
			if claims.SchoolID != nil {
				ctx = tenancy.WithSchoolID(ctx, *claims.SchoolID)
				ctx = tenancy.WithRole(ctx, claims.Role)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

// tokensRevokedSince checks tokens_valid_after directly against PostgreSQL. PRD 6.2
// specifies a Redis cache in front of this for speed, failing closed on a cache
// miss/error; that cache layer is a follow-up once Redis is wired into the request
// path -- this direct check is correct, just not yet as fast as the target design.
func tokensRevokedSince(ctx context.Context, pool *pgxpool.Pool, userID any, issuedAt time.Time) (bool, error) {
	var tokensValidAfter time.Time
	err := db.WithGlobalTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT tokens_valid_after FROM users WHERE id = $1`, userID).Scan(&tokensValidAfter)
	})
	if err != nil {
		return true, err // fail closed
	}
	return issuedAt.Before(tokensValidAfter), nil
}

func unauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + code + `","message":"` + message + `"}`))
}
