// Package config loads runtime configuration from environment variables.
// Using environment variables (rather than config files) is the standard
// twelve-factor app approach and works cleanly with Kubernetes Secrets and
// ConfigMaps, which are injected as env vars into pods.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for the API server.
// Fields are grouped by concern; see .env.example for the full list of
// supported variables and their defaults.
type Config struct {
	// HTTP server listen address (e.g. ":8080" means all interfaces, port 8080)
	Addr string

	// Postgres connection string — format: postgres://user:pass@host/dbname
	DatabaseURL string

	// Redis address (host:port) and optional password
	RedisAddr     string
	RedisPassword string

	// Auth0 tenant domain (e.g. "your-tenant.us.auth0.com") and API audience
	// identifier (the string you set when creating the API in Auth0 dashboard)
	Auth0Domain   string
	Auth0Audience string

	// Minimum log level to emit: "debug" | "info" | "warn" | "error"
	LogLevel string
}

// Load reads configuration from environment variables and returns a Config.
// Required variables (DATABASE_URL, AUTH0_DOMAIN, AUTH0_AUDIENCE) cause a
// panic if absent — better to crash at startup with a clear message than to
// silently start with a broken config and fail on the first request.
func Load() (*Config, error) {
	c := &Config{
		Addr:          getEnv("ADDR", ":8080"),
		DatabaseURL:   mustGetEnv("DATABASE_URL"),
		RedisAddr:     getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		Auth0Domain:   mustGetEnv("AUTH0_DOMAIN"),
		Auth0Audience: mustGetEnv("AUTH0_AUDIENCE"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}
	return c, nil
}

// getEnv returns the value of the named environment variable, or fallback
// if the variable is unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mustGetEnv returns the value of the named environment variable.
// It panics if the variable is unset or empty, printing the variable name
// so the operator knows exactly what to fix.
func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q is not set", key))
	}
	return v
}
