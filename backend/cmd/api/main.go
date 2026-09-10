// Command api is the HTTP entrypoint for the school-erp backend: a modular
// monolith (PRD 8.2) exposing REST under /api/v1 (PRD 8.5).
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"school-erp/backend/internal/auth"
	"school-erp/backend/internal/config"
	appdb "school-erp/backend/internal/db"
	"school-erp/backend/internal/httpmw"
	"school-erp/backend/internal/students"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := appdb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	tokenIssuer := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL)
	authService := auth.NewService(pool, tokenIssuer, cfg.RefreshTokenTTL)
	authHandlers := auth.NewHandlers(authService)

	studentRepo := students.NewRepository(pool)
	studentHandlers := students.NewHandlers(studentRepo)

	rootMux := http.NewServeMux()
	rootMux.HandleFunc("GET /healthz", healthHandler(pool))
	authHandlers.Register(rootMux)

	protectedMux := http.NewServeMux()
	studentHandlers.Register(protectedMux)

	// Any path not matched by an exact route above (health check, auth) falls
	// through to the protected mux, which sits behind RequireAuth. Go's ServeMux
	// resolves the literal auth/health patterns ahead of this catch-all
	// regardless of registration order, so unauthenticated requests never reach
	// tenant-scoped handlers.
	rootMux.Handle("/", httpmw.RequireAuth(pool, tokenIssuer)(protectedMux))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           rootMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func healthHandler(pool interface{ Ping(context.Context) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
