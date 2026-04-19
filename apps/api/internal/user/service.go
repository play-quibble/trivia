// Package user manages host accounts.
// A "host" is an authenticated user who creates question banks and runs games.
// Players (who join via game code) are not represented here — they live in the
// game_players table and don't need Auth0 accounts.
package user

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benbotsford/trivia/internal/store"
)

// Service provides operations on host user accounts.
type Service struct {
	q *store.Queries // sqlc-generated database access layer
}

// New creates a Service backed by the given sqlc Queries.
func New(q *store.Queries) *Service {
	return &Service{q: q}
}

// GetOrCreate implements the "upsert on first login" pattern for Auth0 users.
//
// Auth0 is the source of truth for authentication (passwords, MFA, social
// login), but our database needs its own user records to store app-specific
// data (question banks, game history, subscriptions). This function bridges
// the two:
//
//   - If a user row already exists for this Auth0 subject, return it.
//   - If not (first login), create one and return it.
//
// auth0Sub is Auth0's stable unique identifier for the user (e.g. "auth0|abc123").
// email is optional — it's only present in the token when the "profile" scope
// was requested, so callers should pass "" if unavailable.
func (s *Service) GetOrCreate(ctx context.Context, auth0Sub, email string) (store.User, error) {
	// Happy path: user already exists in the database.
	u, err := s.q.GetUserByAuth0Sub(ctx, auth0Sub)
	if err == nil {
		return u, nil
	}

	// err != nil means no row was found (or a real DB error).
	// In both cases we attempt to create — if it was a real DB error the
	// insert will also fail and we'll surface that error to the caller.
	slog.Info("creating new user on first login", "auth0_sub", auth0Sub)
	return s.q.CreateUser(ctx, store.CreateUserParams{
		ID:       uuid.New(),
		Auth0Sub: auth0Sub,
		// pgtype.Text is Postgres's nullable text type.
		// Valid=true means the value is present (not SQL NULL).
		Email: pgtype.Text{String: email, Valid: email != ""},
	})
}
