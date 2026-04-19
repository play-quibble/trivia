// Package game manages quiz question banks and game lifecycle.
// It owns the HTTP handlers, business logic, and database calls for
// everything related to banks, questions, and games.
package game

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/benbotsford/trivia/internal/auth"
	"github.com/benbotsford/trivia/internal/billing"
	"github.com/benbotsford/trivia/internal/store"
	"github.com/benbotsford/trivia/internal/user"
)

// codeChars is the alphabet used when generating 6-character game codes.
// Visually ambiguous characters (0, O, 1, I, L) are excluded to reduce
// transcription errors when players type a code by hand.
const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Service handles question bank and game CRUD.
// In Go, a struct with methods is roughly equivalent to a class in Java/Python.
// Dependencies (database, user service, billing) are injected via New() rather
// than being globals or singletons.
type Service struct {
	q            *store.Queries          // sqlc-generated database access layer
	users        *user.Service           // resolves Auth0 identities to DB users
	entitlements billing.EntitlementChecker // gates features behind subscription checks
}

// New creates a Service with its dependencies.
// This is the idiomatic Go constructor pattern — there's no "new" keyword;
// you just define a function that returns a pointer to the struct.
func New(q *store.Queries, users *user.Service, ent billing.EntitlementChecker) *Service {
	return &Service{q: q, users: users, entitlements: ent}
}

// RegisterRoutes mounts all game-related routes onto r.
// Chi's r.Route() creates a scoped sub-router, so /{bankID} here becomes
// /banks/{bankID} in the final URL tree. All routes require an authenticated
// host — the auth middleware must already be applied on the parent router
// before RegisterRoutes is called (see main.go).
func (s *Service) RegisterRoutes(r chi.Router) {
	r.Route("/banks", func(r chi.Router) {
		r.Get("/", s.listBanks)
		r.Post("/", s.createBank)
		r.Route("/{bankID}", func(r chi.Router) {
			r.Get("/", s.getBank)
			r.Put("/", s.updateBank)
			r.Delete("/", s.deleteBank)
			r.Get("/questions", s.listQuestions)
			r.Post("/questions", s.createQuestion)
		})
	})

	r.Route("/games", func(r chi.Router) {
		r.Post("/", s.createGame)
		r.Get("/", s.listGames)
		r.Route("/{gameID}", func(r chi.Router) {
			r.Get("/", s.getGame)
			r.Post("/start", s.startGame)
			r.Post("/next", s.nextQuestion)
			r.Post("/end", s.endGame)
		})
	})
}

// --- Question Banks ---

// listBanks handles GET /banks/
// Returns all question banks owned by the authenticated host, ordered by
// creation date descending. Returns an empty array (not null) when none exist.
func (s *Service) listBanks(w http.ResponseWriter, r *http.Request) {
	// Every handler starts by resolving the caller's identity. If the JWT is
	// missing or invalid the auth middleware would have already rejected the
	// request, so an error here means something unexpected happened.
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	banks, err := s.q.ListQuestionBanksByOwner(r.Context(), u.ID)
	if err != nil {
		// Log the real error server-side but send a generic message to the client
		// so internal details (table names, query structure) aren't leaked.
		slog.ErrorContext(r.Context(), "listBanks: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Convert each store.QuestionBank to a clean bankResponse.
	// make([]bankResponse, len(banks)) pre-allocates a slice of the exact size
	// needed — equivalent to new ArrayList<>(banks.size()) in Java.
	resp := make([]bankResponse, len(banks))
	for i, b := range banks {
		resp[i] = bankFromStore(b)
	}
	writeJSON(w, http.StatusOK, resp)
}

// createBank handles POST /banks/
// Creates a new question bank owned by the authenticated host.
// Returns 201 Created with the new bank on success.
func (s *Service) createBank(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Decode the request body into a createBankRequest struct.
	// var req declares a zero-value struct; readJSON fills it in via a pointer.
	var req createBankRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}

	bank, err := s.q.CreateQuestionBank(r.Context(), store.CreateQuestionBankParams{
		ID:          uuid.New(), // generate a new random UUID for the primary key
		OwnerID:     u.ID,
		Name:        req.Name,
		Description: nullText(req.Description), // converts *string → pgtype.Text (nullable)
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "createBank: insert failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, bankFromStore(bank))
}

// getBank handles GET /banks/{bankID}
// Returns a single question bank. Returns 404 if not found, 403 if the
// authenticated host doesn't own it.
func (s *Service) getBank(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// mustParseUUID extracts and parses the {bankID} path parameter.
	// Chi stores URL params in the request context; they're always strings,
	// so we validate the format here before hitting the database.
	bankID, err := mustParseUUID(r, "bankID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		// We treat any DB error as "not found" here. For finer-grained handling
		// you'd check for pgx.ErrNoRows specifically, but 404 is safe to return
		// for both "no such row" and unexpected errors on a GET.
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}

	// Ownership check: prevent one host from reading another host's banks.
	// We compare UUIDs directly — Go's == operator works on value types like uuid.UUID.
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, bankFromStore(bank))
}

// updateBank handles PUT /banks/{bankID}
// Replaces the name and description of an existing bank. Returns 404 if not
// found, 403 if not the owner, 422 if name is missing.
func (s *Service) updateBank(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	bankID, err := mustParseUUID(r, "bankID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch first to verify existence and ownership before accepting the body.
	// This prevents leaking whether a bank exists to a non-owner (they always
	// see 404 if the ID doesn't exist, 403 if they don't own it).
	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req updateBankRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}

	updated, err := s.q.UpdateQuestionBank(r.Context(), store.UpdateQuestionBankParams{
		ID:          bankID,
		Name:        req.Name,
		Description: nullText(req.Description),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "updateBank: update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, bankFromStore(updated))
}

// deleteBank handles DELETE /banks/{bankID}
// Permanently deletes the bank and returns 204 No Content on success.
// Returns 404 if not found, 403 if not the owner.
func (s *Service) deleteBank(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	bankID, err := mustParseUUID(r, "bankID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch and verify ownership before deleting, same reasoning as updateBank.
	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := s.q.DeleteQuestionBank(r.Context(), bankID); err != nil {
		slog.ErrorContext(r.Context(), "deleteBank: delete failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 204 No Content — success with no response body.
	// WriteHeader alone is enough; writing a body after 204 is technically
	// invalid per the HTTP spec.
	w.WriteHeader(http.StatusNoContent)
}

// listQuestions and createQuestion are stubbed — implemented in the next feature.
func (s *Service) listQuestions(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Service) createQuestion(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// --- Games (stubbed — implemented after questions) ---

func (s *Service) createGame(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Service) listGames(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Service) getGame(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Service) startGame(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Service) nextQuestion(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Service) endGame(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// --- Helpers ---

// currentUser resolves the authenticated caller to a store.User.
// The auth middleware already validated the JWT and stored the decoded claims
// in the request context. Here we take the Auth0 subject (a stable unique ID
// like "auth0|abc123") and either fetch the matching DB user or create one on
// first login — so the rest of the handler can work with our internal UUID.
func (s *Service) currentUser(ctx context.Context) (store.User, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		// This should never happen on an authenticated route because the
		// middleware rejects requests without valid JWTs before they reach here.
		return store.User{}, fmt.Errorf("no auth claims in context")
	}
	return s.users.GetOrCreate(ctx, claims.Sub, claims.Email)
}

// generateCode returns a cryptographically random 6-character game code.
// crypto/rand is used instead of math/rand to avoid predictable sequences —
// a guessable game code would let uninvited players join.
// rand.Int returns a random integer in [0, max), which we use as an index
// into codeChars to pick each character.
func generateCode(ctx context.Context) (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeChars))))
		if err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}
		code[i] = codeChars[n.Int64()]
	}
	slog.DebugContext(ctx, "generated game code", "code", string(code))
	return string(code), nil
}

// mustParseUUID extracts a named URL parameter from the Chi router context
// and parses it as a UUID. Returns an error if the parameter is missing or
// malformed, so the caller can send a 400 Bad Request before touching the DB.
func mustParseUUID(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s %q: %w", param, raw, err)
	}
	return id, nil
}
