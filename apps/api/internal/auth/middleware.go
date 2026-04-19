// Package auth provides Auth0 JWT validation middleware for Chi.
// In production, every protected request must carry a valid Auth0 Bearer token.
// In local development, a DEV_AUTH_TOKEN can be used to bypass JWT validation
// entirely so the frontend can hit the real API without an Auth0 tenant set up.
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// contextKey is a private type used as the key for storing claims in the
// request context. Using an unexported custom type prevents key collisions
// with other packages that also store values in the context.
type contextKey string

const claimsKey contextKey = "claims"

// Claims holds the decoded JWT fields we care about.
type Claims struct {
	Sub   string // Auth0 subject — stable unique identifier, e.g. "auth0|abc123"
	Email string // present only when the token was issued with the "profile" scope
}

// Middleware validates Auth0 JWTs on every protected request.
// When DEV_AUTH_TOKEN is configured, requests bearing that token bypass JWT
// validation and are treated as a local dev user — never set this in production.
type Middleware struct {
	jwksURL  string     // Auth0 public key set URL; empty when Auth0 is not configured
	audience string     // must match the token's "aud" claim
	issuer   string     // must match the token's "iss" claim
	cache    *jwk.Cache // auto-refreshes Auth0's signing keys in the background
	devToken string     // dev bypass token; empty in production
}

// New creates a Middleware.
// domain and audience are the Auth0 tenant settings; both can be empty when
// devToken is set (local development without an Auth0 tenant).
func New(domain, audience, devToken string) *Middleware {
	m := &Middleware{
		audience: audience,
		devToken: devToken,
	}

	// Only initialise the JWKS cache when Auth0 is actually configured.
	// This lets the server start cleanly in local dev with only a dev token.
	if domain != "" {
		m.jwksURL = fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
		m.issuer = fmt.Sprintf("https://%s/", domain)
		m.cache = jwk.NewCache(context.Background())
		if err := m.cache.Register(m.jwksURL); err != nil {
			slog.Error("failed to register JWKS URL", "url", m.jwksURL, "err", err)
		}
	}

	return m
}

// Handler returns an http.Handler middleware that validates Bearer tokens.
// Request processing order:
//  1. Extract the Bearer token from the Authorization header.
//  2. If a dev token is configured and matches, inject a dev user and continue.
//  3. If Auth0 is configured, validate the JWT against the JWKS.
//  4. Otherwise, reject the request with 401.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			http.Error(w, "missing or malformed Authorization header", http.StatusUnauthorized)
			return
		}

		// Dev bypass — skips all JWT validation.
		// Active only when DEV_AUTH_TOKEN is set in the environment.
		// The injected sub "dev|local" is stable, so GetOrCreate always finds
		// the same user row in the database across restarts.
		if m.devToken != "" && raw == m.devToken {
			ctx := context.WithValue(r.Context(), claimsKey, Claims{
				Sub:   "dev|local",
				Email: "dev@example.com",
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Without Auth0 configured, there's no way to validate a real JWT.
		if m.jwksURL == "" {
			http.Error(w, "authentication not configured", http.StatusUnauthorized)
			return
		}

		// Fetch the current signing keys from the cache (or Auth0 if stale).
		keySet, err := m.cache.Get(r.Context(), m.jwksURL)
		if err != nil {
			slog.Error("failed to fetch JWKS", "err", err)
			http.Error(w, "could not validate token", http.StatusInternalServerError)
			return
		}

		// Parse verifies the token signature, expiry, audience, and issuer.
		// Any mismatch returns an error and the request is rejected.
		token, err := jwt.Parse([]byte(raw),
			jwt.WithKeySet(keySet),
			jwt.WithValidate(true),
			jwt.WithAudience(m.audience),
			jwt.WithIssuer(m.issuer),
		)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Store the decoded claims in the context so downstream handlers can
		// call ClaimsFromContext without re-parsing the token.
		claims := Claims{Sub: token.Subject()}
		if email, ok := token.Get("email"); ok {
			claims.Email, _ = email.(string)
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves the validated claims stored by the middleware.
// Returns false if the context has no claims (unauthenticated route).
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

// bearerToken extracts the token string from an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	return parts[1], true
}
