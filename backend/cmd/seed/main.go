// Command seed creates a school and its first correspondent user. Standalone tool
// because Phase 1 has no admin panel yet to do this by hand, and bulk import
// (PRD 4.1.3) is a later Phase 1 item. Intended for local dev and first-school
// onboarding, run once per school.
//
// Usage:
//
//	DATABASE_URL=... go run ./cmd/seed \
//	  -school "Sri Matriculation School" -short-code srims \
//	  -email correspondent@example.com -name "R. Correspondent" -password "..."
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"school-erp/backend/internal/auth"
	appdb "school-erp/backend/internal/db"
)

func main() {
	school := flag.String("school", "", "school display name")
	shortCode := flag.String("short-code", "", "unique short code for the school")
	email := flag.String("email", "", "correspondent login email")
	name := flag.String("name", "", "correspondent display name")
	password := flag.String("password", "", "correspondent login password")
	flag.Parse()

	if *school == "" || *shortCode == "" || *email == "" || *name == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "all flags are required: -school -short-code -email -name -password")
		os.Exit(2)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := appdb.NewPool(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var schoolID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO schools (name, short_code) VALUES ($1, $2) RETURNING id`,
		*school, *shortCode,
	).Scan(&schoolID); err != nil {
		log.Fatalf("create school: %v", err)
	}

	var userID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (email, display_name) VALUES ($1, $2) RETURNING id`,
		*email, *name,
	).Scan(&userID); err != nil {
		log.Fatalf("create user: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO credentials (user_id, password_hash) VALUES ($1, $2)`,
		userID, hash,
	); err != nil {
		log.Fatalf("create credentials: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO user_school_roles (user_id, school_id, role) VALUES ($1, $2, 'correspondent')`,
		userID, schoolID,
	); err != nil {
		log.Fatalf("create role: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("created school %s (id=%s) with correspondent %s (id=%s)\n", *school, schoolID, *email, userID)
}
