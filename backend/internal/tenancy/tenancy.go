// Package tenancy carries the authenticated request's school (tenant) and role
// through context.Context. The tenant is derived exclusively from the validated
// auth token in middleware -- never from a request parameter or body (PRD 6.1).
package tenancy

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	schoolIDKey ctxKey = iota
	userIDKey
	roleKey
)

// WithSchoolID attaches the authenticated request's tenant to ctx.
func WithSchoolID(ctx context.Context, schoolID uuid.UUID) context.Context {
	return context.WithValue(ctx, schoolIDKey, schoolID)
}

// SchoolID returns the tenant on ctx. ok is false if no tenant has been set, which
// callers must treat as "no tenant" rather than defaulting to a zero UUID.
func SchoolID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(schoolIDKey).(uuid.UUID)
	return v, ok
}

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(userIDKey).(uuid.UUID)
	return v, ok
}

func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey, role)
}

func Role(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(roleKey).(string)
	return v, ok
}
