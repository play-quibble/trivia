package user

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benbotsford/trivia/internal/auth"
	"github.com/benbotsford/trivia/internal/store"
)

// RegisterRoutes mounts the /me endpoints onto r.
// Must be called inside an auth-middleware-protected group so that
// ClaimsFromContext is always populated.
func (s *Service) RegisterRoutes(r chi.Router) {
	r.Get("/me", s.getMe)
	r.Patch("/me", s.updateMe)
}

// getMe handles GET /me
// Returns the authenticated host's profile. Creates a DB row on first call
// (same GetOrCreate logic used by game handlers).
func (s *Service) getMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	u, err := s.GetOrCreate(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "getMe: lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profileFromStore(u))
}

// updateMe handles PATCH /me
// Accepts a JSON body with optional display_name and/or email fields.
// Omitted fields are preserved from the existing DB record.
func (s *Service) updateMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	u, err := s.GetOrCreate(r.Context(), claims.Sub, claims.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "updateMe: lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req updateProfileRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DisplayName != nil {
		n := utf8.RuneCountInString(*req.DisplayName)
		if n == 0 || n > 50 {
			writeError(w, http.StatusUnprocessableEntity, "display_name must be 1–50 characters")
			return
		}
	}

	// Build params, preserving existing values for any field not sent by the client.
	params := store.UpdateUserProfileParams{ID: u.ID}

	if req.DisplayName != nil {
		params.DisplayName = pgtype.Text{String: *req.DisplayName, Valid: true}
	} else {
		params.DisplayName = u.DisplayName
	}

	if req.Email != nil {
		params.Email = pgtype.Text{String: *req.Email, Valid: *req.Email != ""}
	} else {
		params.Email = u.Email
	}

	updated, err := s.q.UpdateUserProfile(r.Context(), params)
	if err != nil {
		slog.ErrorContext(r.Context(), "updateMe: update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profileFromStore(updated))
}

// ---- request / response types -----------------------------------------------

type updateProfileRequest struct {
	DisplayName *string `json:"display_name"` // optional; nil = don't change
	Email       *string `json:"email"`        // optional; nil = don't change
}

type profileResponse struct {
	ID          string  `json:"id"`
	Email       *string `json:"email,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func profileFromStore(u store.User) profileResponse {
	resp := profileResponse{
		ID:        u.ID.String(),
		CreatedAt: u.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
	if u.Email.Valid {
		resp.Email = &u.Email.String
	}
	if u.DisplayName.Valid {
		resp.DisplayName = &u.DisplayName.String
	}
	return resp
}

// ---- shared helpers ---------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
