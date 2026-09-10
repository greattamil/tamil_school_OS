// Package config loads process configuration from the environment. Kept as plain
// env vars (no config files, no secrets in git) per the single-VPS Docker Compose
// deployment model in PRD 8.1.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	HTTPAddr        string
	AllowedOrigins  map[string]bool
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		HTTPAddr:        envOr("HTTP_ADDR", ":8080"),
		AllowedOrigins:  parseOrigins(envOr("ALLOWED_ORIGINS", "http://localhost:3000")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseOrigins(csv string) map[string]bool {
	origins := make(map[string]bool)
	for _, o := range strings.Split(csv, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = true
		}
	}
	return origins
}
