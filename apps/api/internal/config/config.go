// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for the API server.
type Config struct {
	// HTTP server listen address (e.g. ":8080")
	Addr string

	// Postgres connection string — format: postgres://user:pass@host/dbname
	DatabaseURL string

	// Redis address (host:port) and optional password
	RedisAddr     string
	RedisPassword string

	// Auth0 tenant domain and API audience identifier.
	// Optional when DevAuthToken is set — leave blank during local development
	// to skip JWT validation entirely and rely solely on the dev bypass.
	Auth0Domain   string
	Auth0Audience string

	// DevAuthToken is a shared secret used to bypass JWT validation in local
	// development. Any request bearing "Authorization: Bearer <DevAuthToken>"
	// is treated as a hardcoded dev user without contacting Auth0.
	// NEVER set this in production — leave it blank or unset.
	DevAuthToken string

	// Minimum log level: "debug" | "info" | "warn" | "error"
	LogLevel string

	// AutoMigrate runs embedded goose migrations on startup when true.
	// Leave off in environments where migrations are managed out-of-band
	// (e.g. a dedicated ops step before a rolling deploy).
	AutoMigrate bool

	// BootstrapSamples seeds sample question banks and questions on startup
	// when true. Intended for local development; leave off in production.
	BootstrapSamples bool
}

// Load reads configuration from environment variables.
// Only DATABASE_URL is strictly required. Auth0 vars are optional when
// DEV_AUTH_TOKEN is set for local development.
func Load() (*Config, error) {
	c := &Config{
		Addr:          getEnv("ADDR", ":8080"),
		DatabaseURL:   mustGetEnv("DATABASE_URL"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		Auth0Domain:   getEnv("AUTH0_DOMAIN", ""),
		Auth0Audience: getEnv("AUTH0_AUDIENCE", ""),
		DevAuthToken:     getEnv("DEV_AUTH_TOKEN", ""),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		AutoMigrate:      getEnvBool("AUTO_MIGRATE", false),
		BootstrapSamples: getEnvBool("BOOTSTRAP_SAMPLES", false),
	}
	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	switch os.Getenv(key) {
	case "1", "true", "TRUE", "True", "yes", "YES":
		return true
	case "0", "false", "FALSE", "False", "no", "NO":
		return false
	default:
		return fallback
	}
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q is not set", key))
	}
	return v
}
