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

	"school-erp/backend/internal/academic"
	"school-erp/backend/internal/attendance"
	"school-erp/backend/internal/auth"
	"school-erp/backend/internal/bulkimport"
	"school-erp/backend/internal/config"
	appdb "school-erp/backend/internal/db"
	"school-erp/backend/internal/enrollments"
	"school-erp/backend/internal/fees"
	"school-erp/backend/internal/guardians"
	"school-erp/backend/internal/httpmw"
	"school-erp/backend/internal/notify"
	"school-erp/backend/internal/staff"
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

	academicRepo := academic.NewRepository(pool)
	academicHandlers := academic.NewHandlers(academicRepo)

	staffRepo := staff.NewRepository(pool)
	staffHandlers := staff.NewHandlers(staffRepo)

	guardianRepo := guardians.NewRepository(pool)
	guardianHandlers := guardians.NewHandlers(guardianRepo)

	enrollmentRepo := enrollments.NewRepository(pool)
	enrollmentHandlers := enrollments.NewHandlers(enrollmentRepo)

	importService := bulkimport.NewService(studentRepo, guardianRepo, enrollmentRepo, academicRepo)
	importHandlers := bulkimport.NewHandlers(importService)

	attendanceRepo := attendance.NewRepository(pool)
	attendanceHandlers := attendance.NewHandlers(attendanceRepo)

	notifyRepo := notify.NewRepository(pool)
	notifyHandlers := notify.NewHandlers(notifyRepo)

	feesRepo := fees.NewRepository(pool)
	feesHandlers := fees.NewHandlers(feesRepo, notifyRepo)

	rootMux := http.NewServeMux()
	rootMux.HandleFunc("GET /healthz", healthHandler(pool))
	fees.RegisterWebhookRoutes(rootMux, feesRepo, fees.LogGatewayVerifier{})
	authHandlers.Register(rootMux)

	protectedMux := http.NewServeMux()
	studentHandlers.Register(protectedMux)
	academicHandlers.Register(protectedMux)
	staffHandlers.Register(protectedMux)
	guardianHandlers.Register(protectedMux)
	enrollmentHandlers.Register(protectedMux)
	importHandlers.Register(protectedMux)
	attendanceHandlers.Register(protectedMux)
	notifyHandlers.Register(protectedMux)
	feesHandlers.Register(protectedMux)

	// Any path not matched by an exact route above (health check, auth) falls
	// through to the protected mux, which sits behind RequireAuth. Go's ServeMux
	// resolves the literal auth/health patterns ahead of this catch-all
	// regardless of registration order, so unauthenticated requests never reach
	// tenant-scoped handlers.
	rootMux.Handle("/", httpmw.RequireAuth(pool, tokenIssuer)(protectedMux))

	handler := httpmw.CORS(cfg.AllowedOrigins)(rootMux)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
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
