// Package game manages quiz question banks and game lifecycle.
// It owns the HTTP handlers, business logic, and database calls for
// everything related to banks, questions, and games.
package game

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benbotsford/trivia/internal/auth"
	"github.com/benbotsford/trivia/internal/billing"
	"github.com/benbotsford/trivia/internal/realtime"
	"github.com/benbotsford/trivia/internal/store"
	"github.com/benbotsford/trivia/internal/user"
)

// Character limits applied at the API layer (tighter than the DB constraint).
const (
	maxPromptLen = 500 // question prompt characters
	maxChoiceLen = 200 // each MC choice text characters
	maxAnswerLen = 150 // each accepted text answer characters
	maxChoices   = 6   // maximum MC options per question
	minChoices   = 2   // minimum MC options per question
	maxAnswers   = 10  // maximum accepted answers for text questions
)

// codeChars is the alphabet used when generating 6-character game codes.
// Visually ambiguous characters (0, O, 1, I, L) are excluded to reduce
// transcription errors when players type a code by hand.
const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Service handles question bank and game CRUD.
// In Go, a struct with methods is roughly equivalent to a class in Java/Python.
// Dependencies (database, user service, billing, realtime hub) are injected
// via New() rather than being globals or singletons.
type Service struct {
	q            *store.Queries             // sqlc-generated database access layer
	users        *user.Service              // resolves Auth0 identities to DB users
	entitlements billing.EntitlementChecker // gates features behind subscription checks
	hub          *realtime.Hub              // WebSocket broadcast layer
}

// New creates a Service with its dependencies.
func New(q *store.Queries, users *user.Service, ent billing.EntitlementChecker, hub *realtime.Hub) *Service {
	return &Service{q: q, users: users, entitlements: ent, hub: hub}
}

// RegisterPublicRoutes mounts endpoints that are accessible without authentication.
// These are used by players who join a game with only a code and display name.
func (s *Service) RegisterPublicRoutes(r chi.Router) {
	r.Post("/join", s.joinGame)
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
			r.Route("/questions", func(r chi.Router) {
				r.Get("/", s.listQuestions)
				r.Post("/", s.createQuestion)
				r.Patch("/reorder", s.reorderQuestions)
				r.Route("/{questionID}", func(r chi.Router) {
					r.Get("/", s.getQuestion)
					r.Put("/", s.updateQuestion)
					r.Delete("/", s.deleteQuestion)
				})
			})
		})
	})

	r.Route("/quizzes", func(r chi.Router) {
		r.Get("/", s.listQuizzes)
		r.Post("/", s.createQuiz)
		r.Route("/{quizID}", func(r chi.Router) {
			r.Get("/", s.getQuiz)
			r.Put("/", s.updateQuiz)
			r.Delete("/", s.deleteQuiz)
			r.Route("/rounds", func(r chi.Router) {
				r.Post("/", s.createRound)
				r.Route("/{roundID}", func(r chi.Router) {
					r.Put("/", s.updateRound)
					r.Delete("/", s.deleteRound)
					r.Put("/questions", s.setRoundQuestions)
				})
			})
		})
	})

	r.Route("/games", func(r chi.Router) {
		r.Post("/", s.createGame)
		r.Get("/", s.listGames)
		r.Route("/{gameID}", func(r chi.Router) {
			r.Get("/", s.getGame)
			r.Get("/players", s.listPlayers)
			r.Delete("/", s.cancelGame)
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

// --- Questions ---

// listQuestions handles GET /banks/{bankID}/questions
// Returns all questions in the bank ordered by position, including their
// choices (MC) or accepted answers (text) decoded from the choices JSONB field.
func (s *Service) listQuestions(w http.ResponseWriter, r *http.Request) {
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

	// Ownership check: verify the bank belongs to this user before listing.
	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	questions, err := s.q.ListQuestionsByBank(r.Context(), bankID)
	if err != nil {
		slog.ErrorContext(r.Context(), "listQuestions: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]questionResponse, len(questions))
	for i, q := range questions {
		resp[i] = questionFromStore(q)
	}
	writeJSON(w, http.StatusOK, resp)
}

// createQuestion handles POST /banks/{bankID}/questions
// Validates the request, marshals choices/accepted-answers to JSONB, then
// inserts a new question appended to the end of the bank's question list.
func (s *Service) createQuestion(w http.ResponseWriter, r *http.Request) {
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

	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req createQuestionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate and resolve the question type.
	var qType store.QuestionType
	switch req.Type {
	case "text":
		qType = store.QuestionTypeText
	case "multiple_choice":
		qType = store.QuestionTypeMultipleChoice
	default:
		writeError(w, http.StatusUnprocessableEntity, "type must be 'text' or 'multiple_choice'")
		return
	}

	// Validate the prompt — rune count handles multi-byte Unicode correctly.
	if utf8.RuneCountInString(req.Prompt) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "prompt is required")
		return
	}
	if utf8.RuneCountInString(req.Prompt) > maxPromptLen {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("prompt must be %d characters or fewer", maxPromptLen))
		return
	}

	// Default points when the client omits or zeroes the field.
	if req.Points <= 0 {
		req.Points = 1000
	}

	// Validate type-specific fields and build the JSONB payload.
	var correctAnswer string
	var choicesJSON []byte

	switch qType {
	case store.QuestionTypeText:
		if len(req.AcceptedAnswers) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "at least one accepted answer is required")
			return
		}
		if len(req.AcceptedAnswers) > maxAnswers {
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("at most %d accepted answers allowed", maxAnswers))
			return
		}
		for _, a := range req.AcceptedAnswers {
			if utf8.RuneCountInString(a) == 0 {
				writeError(w, http.StatusUnprocessableEntity, "accepted answers cannot be empty")
				return
			}
			if utf8.RuneCountInString(a) > maxAnswerLen {
				writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("each accepted answer must be %d characters or fewer", maxAnswerLen))
				return
			}
		}
		// The first entry is the canonical answer shown to hosts in the game UI.
		correctAnswer = req.AcceptedAnswers[0]
		choicesJSON, _ = json.Marshal(req.AcceptedAnswers)

	case store.QuestionTypeMultipleChoice:
		if len(req.Choices) < minChoices || len(req.Choices) > maxChoices {
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("multiple choice questions require %d–%d options", minChoices, maxChoices))
			return
		}
		var numCorrect int
		for _, c := range req.Choices {
			if utf8.RuneCountInString(c.Text) == 0 {
				writeError(w, http.StatusUnprocessableEntity, "choice text cannot be empty")
				return
			}
			if utf8.RuneCountInString(c.Text) > maxChoiceLen {
				writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("each choice must be %d characters or fewer", maxChoiceLen))
				return
			}
			if c.Correct {
				numCorrect++
				correctAnswer = c.Text
			}
		}
		if numCorrect != 1 {
			writeError(w, http.StatusUnprocessableEntity, "exactly one choice must be marked correct")
			return
		}
		choicesJSON, _ = json.Marshal(req.Choices)
	}

	// Append to end: count existing questions to set the next position.
	count, err := s.q.CountQuestionsInBank(r.Context(), bankID)
	if err != nil {
		slog.ErrorContext(r.Context(), "createQuestion: count failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	q, err := s.q.CreateQuestion(r.Context(), store.CreateQuestionParams{
		ID:            uuid.New(),
		BankID:        bankID,
		Type:          qType,
		Prompt:        req.Prompt,
		CorrectAnswer: correctAnswer,
		Choices:       choicesJSON,
		Points:        req.Points,
		Position:      count, // 0-indexed: count of existing = next available slot
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "createQuestion: insert failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, questionFromStore(q))
}

// getQuestion handles GET /banks/{bankID}/questions/{questionID}
func (s *Service) getQuestion(w http.ResponseWriter, r *http.Request) {
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
	questionID, err := mustParseUUID(r, "questionID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify bank ownership before exposing the question.
	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	q, err := s.q.GetQuestion(r.Context(), questionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}
	// Confirm the question belongs to the requested bank (prevents cross-bank access).
	if q.BankID != bankID {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}

	writeJSON(w, http.StatusOK, questionFromStore(q))
}

// updateQuestion handles PUT /banks/{bankID}/questions/{questionID}
// Replaces the question's content (prompt, points, answers/choices).
// Position is intentionally not updated here — use reorderQuestions for that.
func (s *Service) updateQuestion(w http.ResponseWriter, r *http.Request) {
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
	questionID, err := mustParseUUID(r, "questionID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	existing, err := s.q.GetQuestion(r.Context(), questionID)
	if err != nil || existing.BankID != bankID {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}

	var req updateQuestionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Same validation as createQuestion, re-using the existing type.
	if utf8.RuneCountInString(req.Prompt) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "prompt is required")
		return
	}
	if utf8.RuneCountInString(req.Prompt) > maxPromptLen {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("prompt must be %d characters or fewer", maxPromptLen))
		return
	}
	if req.Points <= 0 {
		req.Points = 1000
	}

	var correctAnswer string
	var choicesJSON []byte

	switch existing.Type {
	case store.QuestionTypeText:
		if len(req.AcceptedAnswers) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "at least one accepted answer is required")
			return
		}
		if len(req.AcceptedAnswers) > maxAnswers {
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("at most %d accepted answers allowed", maxAnswers))
			return
		}
		for _, a := range req.AcceptedAnswers {
			if utf8.RuneCountInString(a) == 0 || utf8.RuneCountInString(a) > maxAnswerLen {
				writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("each accepted answer must be 1–%d characters", maxAnswerLen))
				return
			}
		}
		correctAnswer = req.AcceptedAnswers[0]
		choicesJSON, _ = json.Marshal(req.AcceptedAnswers)

	case store.QuestionTypeMultipleChoice:
		if len(req.Choices) < minChoices || len(req.Choices) > maxChoices {
			writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("multiple choice questions require %d–%d options", minChoices, maxChoices))
			return
		}
		var numCorrect int
		for _, c := range req.Choices {
			if utf8.RuneCountInString(c.Text) == 0 || utf8.RuneCountInString(c.Text) > maxChoiceLen {
				writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("each choice must be 1–%d characters", maxChoiceLen))
				return
			}
			if c.Correct {
				numCorrect++
				correctAnswer = c.Text
			}
		}
		if numCorrect != 1 {
			writeError(w, http.StatusUnprocessableEntity, "exactly one choice must be marked correct")
			return
		}
		choicesJSON, _ = json.Marshal(req.Choices)
	}

	updated, err := s.q.UpdateQuestion(r.Context(), store.UpdateQuestionParams{
		ID:            questionID,
		Type:          existing.Type, // type is immutable — use the stored value
		Prompt:        req.Prompt,
		CorrectAnswer: correctAnswer,
		Choices:       choicesJSON,
		Points:        req.Points,
		Position:      existing.Position, // position unchanged here
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "updateQuestion: update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, questionFromStore(updated))
}

// deleteQuestion handles DELETE /banks/{bankID}/questions/{questionID}
func (s *Service) deleteQuestion(w http.ResponseWriter, r *http.Request) {
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
	questionID, err := mustParseUUID(r, "questionID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	existing, err := s.q.GetQuestion(r.Context(), questionID)
	if err != nil || existing.BankID != bankID {
		writeError(w, http.StatusNotFound, "question not found")
		return
	}

	if err := s.q.DeleteQuestion(r.Context(), questionID); err != nil {
		slog.ErrorContext(r.Context(), "deleteQuestion: delete failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// reorderQuestions handles PATCH /banks/{bankID}/questions/reorder
// Accepts a JSON body {"ids": ["uuid1", "uuid2", ...]} where the slice represents
// the desired order. Each question's position is set to its index in the slice.
// Updates are issued individually rather than in a transaction — acceptable for
// a small list (typically ≤ 50 questions), and avoids adding pool access to the service.
func (s *Service) reorderQuestions(w http.ResponseWriter, r *http.Request) {
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

	bank, err := s.q.GetQuestionBank(r.Context(), bankID)
	if err != nil {
		writeError(w, http.StatusNotFound, "bank not found")
		return
	}
	if bank.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req reorderRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "ids is required")
		return
	}

	// Update each question's position to match its index in the provided list.
	for i, rawID := range req.IDs {
		qID, err := uuid.Parse(rawID)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid question id %q", rawID))
			return
		}
		if _, err := s.q.ReorderQuestion(r.Context(), store.ReorderQuestionParams{
			ID:       qID,
			Position: int32(i),
		}); err != nil {
			slog.ErrorContext(r.Context(), "reorderQuestions: update failed", "id", rawID, "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Games ---

// createGame handles POST /games
// Supports two modes:
//   - Quiz-based (new): provide quiz_id — questions come from the quiz's rounds
//   - Bank-based (legacy): provide bank_id + optional round_size
func (s *Service) createGame(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createGameRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var quizID uuid.UUID
	var bankID uuid.UUID
	var roundSize int32

	if req.QuizID != "" {
		// Quiz-based game.
		quizID, err = uuid.Parse(req.QuizID)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid quiz_id")
			return
		}
		quiz, err := s.q.GetQuizByID(r.Context(), quizID)
		if err != nil {
			writeError(w, http.StatusNotFound, "quiz not found")
			return
		}
		if quiz.OwnerID != u.ID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		// Verify quiz has at least one round with at least one question.
		rounds, err := s.q.ListQuizRounds(r.Context(), quizID)
		if err != nil || len(rounds) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "quiz must have at least one round before starting a game")
			return
		}
		totalQ, err := s.q.CountQuestionsInRound(r.Context(), rounds[0].ID)
		if err != nil || totalQ == 0 {
			writeError(w, http.StatusUnprocessableEntity, "quiz rounds must have at least one question each")
			return
		}
		roundSize = 0 // unused for quiz-based games
		// bankID stays zero — allowed because quiz_id is set
	} else if req.BankID != "" {
		// Legacy bank-based game.
		bankID, err = uuid.Parse(req.BankID)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid bank_id")
			return
		}
		bank, err := s.q.GetQuestionBank(r.Context(), bankID)
		if err != nil {
			writeError(w, http.StatusNotFound, "bank not found")
			return
		}
		if bank.OwnerID != u.ID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		count, err := s.q.CountQuestionsInBank(r.Context(), bankID)
		if err != nil || count == 0 {
			writeError(w, http.StatusUnprocessableEntity, "bank must have at least one question before starting a game")
			return
		}
		roundSize = req.RoundSize
		if roundSize <= 0 {
			roundSize = 5
		}
	} else {
		writeError(w, http.StatusUnprocessableEntity, "either quiz_id or bank_id is required")
		return
	}

	code, err := generateCode(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "createGame: code generation failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	gameID := uuid.New()
	game, err := s.q.CreateGame(r.Context(), store.CreateGameParams{
		ID:        gameID,
		Code:      code,
		HostID:    u.ID,
		BankID:    pgtype.UUID{Bytes: bankID, Valid: bankID != uuid.Nil},
		RoundSize: roundSize,
		QuizID:    pgtype.UUID{Bytes: quizID, Valid: quizID != uuid.Nil},
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "createGame: insert failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.hub.InitRoom(gameID, quizID, bankID, code, roundSize)
	writeJSON(w, http.StatusCreated, gameFromStore(game))
}

// --- Quizzes ---

func (s *Service) listQuizzes(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizzes, err := s.q.ListQuizzesByOwner(r.Context(), u.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "listQuizzes: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]quizResponse, len(quizzes))
	for i, q := range quizzes {
		resp[i] = quizFromStore(q)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) createQuiz(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createQuizRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	quiz, err := s.q.CreateQuiz(r.Context(), store.CreateQuizParams{
		ID:          uuid.New(),
		OwnerID:     u.ID,
		Name:        req.Name,
		Description: nullText(req.Description),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "createQuiz: insert failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, quizFromStore(quiz))
}

func (s *Service) getQuiz(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	// Load rounds + questions.
	storeRounds, err := s.q.ListQuizRoundsWithQuestions(r.Context(), quizID)
	if err != nil {
		slog.ErrorContext(r.Context(), "getQuiz: list rounds failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	rounds := make([]roundResponse, len(storeRounds))
	for i, sr := range storeRounds {
		qs := make([]questionResponse, len(sr.Questions))
		for j, q := range sr.Questions {
			qs[j] = questionFromStore(q)
		}
		rounds[i] = roundFromStore(sr.Round, qs)
	}
	writeJSON(w, http.StatusOK, quizDetailResponse{
		quizResponse: quizFromStore(quiz),
		Rounds:        rounds,
	})
}

func (s *Service) updateQuiz(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req updateQuizRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	updated, err := s.q.UpdateQuiz(r.Context(), store.UpdateQuizParams{
		ID:          quizID,
		Name:        req.Name,
		Description: nullText(req.Description),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "updateQuiz: update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, quizFromStore(updated))
}

func (s *Service) deleteQuiz(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	// Block deletion if any games have been run from this quiz.
	gameCount, err := s.q.CountGamesByQuiz(r.Context(), quizID)
	if err != nil {
		slog.ErrorContext(r.Context(), "deleteQuiz: count games failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if gameCount > 0 {
		writeError(w, http.StatusConflict, "quiz has associated games and cannot be deleted")
		return
	}
	if err := s.q.DeleteQuiz(r.Context(), quizID); err != nil {
		slog.ErrorContext(r.Context(), "deleteQuiz: delete failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Rounds ---

func (s *Service) createRound(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var req createRoundRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Determine next round number.
	count, err := s.q.CountQuizRounds(r.Context(), quizID)
	if err != nil {
		slog.ErrorContext(r.Context(), "createRound: count failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	round, err := s.q.CreateQuizRound(r.Context(), store.CreateQuizRoundParams{
		ID:          uuid.New(),
		QuizID:      quizID,
		RoundNumber: count + 1,
		Title:       nullText(req.Title),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "createRound: insert failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, roundFromStore(round, nil))
}

func (s *Service) updateRound(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	roundID, err := mustParseUUID(r, "roundID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req updateRoundRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := s.q.UpdateQuizRound(r.Context(), roundID, nullText(req.Title))
	if err != nil {
		slog.ErrorContext(r.Context(), "updateRound: update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, roundFromStore(updated, nil))
}

func (s *Service) deleteRound(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	roundID, err := mustParseUUID(r, "roundID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.q.DeleteQuizRound(r.Context(), roundID); err != nil {
		slog.ErrorContext(r.Context(), "deleteRound: delete failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setRoundQuestions handles PUT /quizzes/{quizID}/rounds/{roundID}/questions
// Replaces the round's question list with the provided ordered slice of question IDs.
func (s *Service) setRoundQuestions(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	quizID, err := mustParseUUID(r, "quizID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	quiz, err := s.q.GetQuizByID(r.Context(), quizID)
	if err != nil {
		writeError(w, http.StatusNotFound, "quiz not found")
		return
	}
	if quiz.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	roundID, err := mustParseUUID(r, "roundID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req setRoundQuestionsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ids := make([]uuid.UUID, 0, len(req.QuestionIDs))
	for _, raw := range req.QuestionIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid question id %q", raw))
			return
		}
		ids = append(ids, id)
	}
	if err := s.q.SetRoundQuestionsOrdered(r.Context(), roundID, ids); err != nil {
		slog.ErrorContext(r.Context(), "setRoundQuestions: update failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listGames handles GET /games
// Returns the host's games, newest first, with a default limit of 20.
func (s *Service) listGames(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	games, err := s.q.ListGamesByHost(r.Context(), store.ListGamesByHostParams{
		HostID: u.ID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "listGames: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]gameResponse, len(games))
	for i, g := range games {
		resp[i] = gameFromStore(g)
	}
	writeJSON(w, http.StatusOK, resp)
}

// getGame handles GET /games/{gameID}
func (s *Service) getGame(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	gameID, err := mustParseUUID(r, "gameID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	game, err := s.q.GetGameByID(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if game.HostID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, gameFromStore(game))
}

// cancelGame handles DELETE /games/{gameID}
// Transitions the game to 'cancelled' status. Only works on lobby or in_progress games.
func (s *Service) cancelGame(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	gameID, err := mustParseUUID(r, "gameID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	game, err := s.q.GetGameByID(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if game.HostID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if _, err := s.q.CancelGame(r.Context(), gameID); err != nil {
		// CancelGame only matches lobby/in_progress rows — no rows updated means
		// the game was already completed or cancelled.
		writeError(w, http.StatusConflict, "game cannot be cancelled in its current state")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listPlayers handles GET /games/{gameID}/players
// Returns the current player roster for the lobby page.
func (s *Service) listPlayers(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	gameID, err := mustParseUUID(r, "gameID")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	game, err := s.q.GetGameByID(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if game.HostID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	players, err := s.q.ListActivePlayersInGame(r.Context(), gameID)
	if err != nil {
		slog.ErrorContext(r.Context(), "listPlayers: query failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type playerEntry struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Score       int32  `json:"score"`
	}
	resp := make([]playerEntry, len(players))
	for i, p := range players {
		resp[i] = playerEntry{ID: p.ID.String(), DisplayName: p.DisplayName, Score: p.Score}
	}
	writeJSON(w, http.StatusOK, resp)
}

// joinGame handles POST /join (unauthenticated)
// Creates a game_player record and returns the session token the player uses
// to authenticate their WebSocket connection.
func (s *Service) joinGame(w http.ResponseWriter, r *http.Request) {
	var req joinGameRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" {
		writeError(w, http.StatusUnprocessableEntity, "game code is required")
		return
	}
	if utf8.RuneCountInString(req.DisplayName) == 0 || utf8.RuneCountInString(req.DisplayName) > 32 {
		writeError(w, http.StatusUnprocessableEntity, "display name must be 1–32 characters")
		return
	}

	// Look up the active game by code.
	game, err := s.q.GetActiveGameByCode(r.Context(), req.Code)
	if err != nil {
		writeError(w, http.StatusNotFound, "game not found — check the code and try again")
		return
	}
	if game.Status != "lobby" {
		writeError(w, http.StatusConflict, "game has already started")
		return
	}

	// Generate a random UUID as the session token — it's long enough to be unguessable.
	sessionToken := uuid.New().String()

	_, err = s.q.AddPlayer(r.Context(), store.AddPlayerParams{
		ID:           uuid.New(),
		GameID:       game.ID,
		DisplayName:  req.DisplayName,
		SessionToken: sessionToken,
	})
	if err != nil {
		// Duplicate display name produces a unique-constraint violation.
		slog.InfoContext(r.Context(), "joinGame: add player failed", "err", err)
		writeError(w, http.StatusConflict, "that display name is already taken in this game")
		return
	}

	// Notify everyone in the room that a new player joined.
	s.hub.BroadcastPlayerJoined(game.Code, req.DisplayName)

	writeJSON(w, http.StatusCreated, joinGameResponse{
		GameCode:     game.Code,
		SessionToken: sessionToken,
		DisplayName:  req.DisplayName,
	})
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
