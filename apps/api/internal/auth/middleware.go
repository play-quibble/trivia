// Package auth provides Auth0 JWT validation middleware for Chi.
// Every request to a protected route passes through Middleware.Handler, which
// validates the Bearer token and stores the decoded claims in the request
// context so downstream handlers can access the caller's identity.
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

// contextKey is a private type used as the key for storing claims in a
// request context. Using a unexported custom type (rather than a plain string)
// prevents collisions with keys set by other packages.
type contextKey string

const claimsKey contextKey = "claims"

// Claims holds the decoded JWT fields we care about.
// Auth0 JWTs contain many more fields, but these are the only ones we read.
type Claims struct {
	Sub   string // Auth0 subject — a stable, unique identifier for the user, e.g. "auth0|abc123"
	Email string // email claim — only present if the token was issued with the "profile" scope
}

// Middleware validates Auth0 JWTs on every request.
// It fetches Auth0's public signing keys (JWKS) and uses them to verify that
// the token was genuinely issued by our Auth0 tenant and hasn't been tampered
// with. Unauthenticated or invalid requests receive a 401 before reaching the
// handler. Validated claims are stored in the request context for retrieval
// via ClaimsFromContext.
type Middleware struct {
	jwksURL  string     // URL to Auth0's public JSON Web Key Set
	audience string     // the API identifier configured in Auth0 — must match the token's "aud" claim
	issuer   string     // our Auth0 tenant URL — must match the token's "iss" claim
	cache    *jwk.Cache // automatically refreshes the key set in the background
}

// New creates a Middleware for the given Auth0 domain and API audience.
// The JWKS cache is initialised here; keys are fetched lazily on the first
// request and then refreshed automatically by the cache in the background.
func New(domain, audience string) *Middleware {
	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	// jwk.NewCache creates a background-refreshing key set cache.
	// context.Background() here means the cache lives for the lifetime of the
	// process — appropriate since this middleware is created once at startup.
	cache := jwk.NewCache(context.Background())
	if err := cache.Register(jwksURL); err != nil {
		slog.Error("failed to register JWKS URL", "url", jwksURL, "err", err)
	}
	return &Middleware{
		jwksURL:  jwksURL,
		audience: audience,
		issuer:   fmt.Sprintf("https://%s/", domain),
		cache:    cache,
	}
}

// Handler returns an http.Handler middleware that validates Bearer tokens.
// In Go, middleware is a function that wraps an http.Handler — it runs before
// the inner handler and calls next.ServeHTTP to pass control downstream.
// Returning early (without calling next) stops the request chain, equivalent
// to raising an exception that short-circuits a filter chain in Java/Spring.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			http.Error(w, "missing or malformed Authorization header", http.StatusUnauthorized)
			return
		}

		// Fetch the current signing keys from the cache (or Auth0 if expired).
		keySet, err := m.cache.Get(r.Context(), m.jwksURL)
		if err != nil {
			slog.Error("failed to fetch JWKS", "err", err)
			http.Error(w, "could not validate token", http.StatusInternalServerError)
			return
		}

		// jwt.Parse verifies the token signature against the key set and
		// validates standard claims: expiry, audience, and issuer.
		// Any mismatch (wrong tenant, expired, tampered signature) returns an error.
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

		// Extract the fields we need and store them in the request context.
		// context.WithValue returns a new context derived from the parent —
		// it doesn't mutate the original. The new context is attached to the
		// request with r.WithContext before passing it downstream.
		claims := Claims{Sub: token.Subject()}
		if email, ok := token.Get("email"); ok {
			claims.Email, _ = email.(string) // type assertion: cast interface{} → string
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves the validated claims stored by the middleware.
// The second return value (bool) follows the Go "comma ok" idiom — similar to
// checking if a key exists in a Python dict or Java Map. Returns false if the
// context has no claims (i.e. the route is unauthenticated).
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

// bearerToken extracts the token string from an "Authorization: Bearer <token>"
// header. Returns the token and true on success, or empty string and false if
// the header is missing or malformed.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	// SplitN with n=2 splits into at most 2 parts, so a token containing spaces
	// isn't accidentally truncated.
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	return parts[1], true
}
