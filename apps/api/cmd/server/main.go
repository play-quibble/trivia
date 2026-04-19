package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/benbotsford/trivia/internal/auth"
	"github.com/benbotsford/trivia/internal/billing"
	"github.com/benbotsford/trivia/internal/config"
	"github.com/benbotsford/trivia/internal/game"
	"github.com/benbotsford/trivia/internal/realtime"
	"github.com/benbotsford/trivia/internal/store"
	"github.com/benbotsford/trivia/internal/user"
)

func main() {
	// -------------------------------------------------------------------------
	// Logger
	// -------------------------------------------------------------------------
	// slog is Go's structured logging library (standard library since 1.21).
	// JSON format produces log lines like: {"time":"...","level":"INFO","msg":"...","key":"value"}
	// which are easy to ingest with tools like Loki or Datadog.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger) // sets the global logger used by slog.Info(), slog.Error(), etc.

	// -------------------------------------------------------------------------
	// Config
	// -------------------------------------------------------------------------
	// Load() reads environment variables. Missing required vars cause a panic
	// here at startup, which is preferable to a confusing failure later.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// -------------------------------------------------------------------------
	// Postgres
	// -------------------------------------------------------------------------
	// context.Background() creates a root context with no deadline or values.
	// Contexts are how Go propagates cancellation and deadlines through a call
	// chain — similar to passing a timeout object through every function in Java.
	ctx := context.Background()

	// pgxpool is a connection pool — it manages a set of reusable database
	// connections, equivalent to a HikariCP or c3p0 pool in Java.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	// defer schedules pool.Close() to run when main() returns (i.e. on shutdown).
	// This is Go's equivalent of try-with-resources in Java or a context manager in Python.
	defer pool.Close()

	// Ping verifies the connection is actually usable before we start serving traffic.
	if err := pool.Ping(ctx); err != nil {
		slog.Error("postgres ping failed", "err", err)
		os.Exit(1)
	}
	slog.Info("postgres connected")

	// -------------------------------------------------------------------------
	// Redis
	// -------------------------------------------------------------------------
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer rdb.Close()

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		slog.Error("redis ping failed", "err", err)
		os.Exit(1)
	}
	slog.Info("redis connected")

	// -------------------------------------------------------------------------
	// Services
	// -------------------------------------------------------------------------
	// Dependencies are wired together manually here (no DI framework).
	// store.New wraps the connection pool in a sqlc query object.
	// Each service receives the dependencies it needs — no globals or singletons.
	queries := store.New(pool)
	userSvc := user.New(queries)
	entitlements := billing.NoopChecker{} // always grants access until Stripe is implemented
	gameSvc := game.New(queries, userSvc, entitlements)
	hub := realtime.New()

	// -------------------------------------------------------------------------
	// Auth middleware
	// -------------------------------------------------------------------------
	// This validates Auth0 JWTs on every authenticated route.
	// It's created once here and reused across all requests.
	authMiddleware := auth.New(cfg.Auth0Domain, cfg.Auth0Audience)

	// -------------------------------------------------------------------------
	// Router
	// -------------------------------------------------------------------------
	r := chi.NewRouter()

	// Global middleware runs on every request, in the order listed.
	// These are equivalent to servlet filters in Java or Django middleware.
	r.Use(middleware.RequestID)  // attaches a unique ID to each request for log correlation
	r.Use(middleware.RealIP)     // sets r.RemoteAddr to the real client IP (from X-Forwarded-For)
	r.Use(middleware.Logger)     // logs each request: method, path, status, duration
	r.Use(middleware.Recoverer)  // catches panics and returns 500 instead of crashing the server
	r.Use(middleware.Timeout(30 * time.Second)) // cancels slow requests after 30s

	// Health and readiness probes — used by Kubernetes to decide whether to
	// send traffic to this pod. Both check Postgres and Redis connectivity.
	// /healthz: "is the process alive?" (liveness probe)
	// /readyz:  "is the process ready to serve traffic?" (readiness probe)
	r.Get("/healthz", healthz(pool, rdb))
	r.Get("/readyz", healthz(pool, rdb))

	// Prometheus scrapes /metrics to collect request counts, durations, etc.
	// The kube-prometheus-stack in the cluster will scrape this endpoint.
	r.Handle("/metrics", promhttp.Handler())

	// WebSocket endpoint — intentionally unauthenticated so players can connect
	// with only a game code and display name (no Auth0 account required).
	hub.RegisterRoutes(r)

	// Authenticated API routes — the auth middleware runs first for this group,
	// rejecting any request without a valid Bearer token before it reaches a handler.
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handler)
		gameSvc.RegisterRoutes(r)
	})

	// -------------------------------------------------------------------------
	// HTTP server with graceful shutdown
	// -------------------------------------------------------------------------
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,  // max time to read the full request headers + body
		WriteTimeout: 30 * time.Second,  // max time to write the response
		IdleTimeout:  120 * time.Second, // max time to wait for the next request on a keep-alive connection
	}

	// quit is a channel that receives OS signals. make(chan os.Signal, 1) creates
	// a buffered channel with capacity 1 — the buffer ensures the signal isn't
	// dropped if we're not blocked on the receive yet.
	quit := make(chan os.Signal, 1)
	// signal.Notify routes SIGINT (Ctrl+C) and SIGTERM (kubectl delete pod) into quit.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start the HTTP server in a goroutine so it runs concurrently with the
	// signal-waiting code below. A goroutine is like a lightweight thread —
	// Go can run thousands of them with minimal overhead.
	go func() {
		slog.Info("server starting", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// http.ErrServerClosed is the expected "error" when Shutdown() is called —
			// we only treat it as a real error if something else went wrong.
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Block here until a signal arrives. <-quit reads from the channel and
	// pauses execution until a value is available (i.e. until SIGINT or SIGTERM).
	<-quit
	slog.Info("shutting down...")

	// Graceful shutdown: stop accepting new connections and wait for in-flight
	// requests to complete (up to 15 seconds). This prevents dropping requests
	// that are mid-flight when the pod is terminated.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
	slog.Info("server stopped")
}

// healthz returns an HTTP handler that checks both Postgres and Redis
// connectivity. Kubernetes calls this endpoint to decide whether the pod is
// healthy. A 503 response causes Kubernetes to restart the pod (liveness) or
// stop sending it traffic (readiness).
func healthz(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// context.WithTimeout creates a child context that automatically cancels
		// after 3 seconds. If the DB or Redis is hanging, the ping returns an
		// error rather than blocking indefinitely.
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "postgres unhealthy: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		if _, err := rdb.Ping(ctx).Result(); err != nil {
			http.Error(w, "redis unhealthy: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
