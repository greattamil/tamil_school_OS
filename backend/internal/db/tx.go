package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"school-erp/backend/internal/tenancy"
)

// ErrNoTenant is returned when WithTenantTx is called on a context carrying no
// tenant. Every RLS-dependent query must go through here or through WithGlobalTx --
// there is no other path to the database (PRD 6.1 points 1-3).
var ErrNoTenant = errors.New("db: no tenant in context")

type TxFn func(ctx context.Context, tx pgx.Tx) error

// WithTenantTx runs fn inside a single transaction with the tenant's school_id set
// via SET LOCAL (through set_config, so the value is bound as a parameter rather
// than interpolated). SET LOCAL is scoped to the transaction and is discarded at
// commit or rollback, so nothing leaks to the next request that borrows this pooled
// connection (PRD 6.1).
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, fn TxFn) error {
	schoolID, ok := tenancy.SchoolID(ctx)
	if !ok {
		return ErrNoTenant
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_school_id', $1, true)`, schoolID.String()); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// WithGlobalTx runs fn inside a transaction against the global identity tables
// (users, credentials, sessions, otp_codes, user_school_roles, schools) with no
// tenant context set. These tables carry no RLS and no school_id, so every query
// against them must be explicitly and correctly scoped in code -- this is the one
// place in the system where that discipline is the only control (PRD 3.2.1), which
// is why it is confined to this single function.
func WithGlobalTx(ctx context.Context, pool *pgxpool.Pool, fn TxFn) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
