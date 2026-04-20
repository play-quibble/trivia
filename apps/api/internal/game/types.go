package game

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benbotsford/trivia/internal/store"
)

// --- Bank request types (inbound JSON from the client) ---

type createBankRequest struct {
	Name string `json:"name"`
	// *string (pointer to string) means the field is optional in JSON.
	// If the key is absent from the request body, Description will be nil.
	// A non-pointer string would default to "" and we couldn't distinguish
	// "not provided" from "explicitly set to empty".
	Description *string `json:"description"`
}

type updateBankRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// --- Bank response type (outbound JSON to the client) ---

// bankResponse is a clean API shape for a question bank.
// We don't serialize store.QuestionBank directly because sqlc generates fields
// using pgtype wrappers (e.g. pgtype.Text, pgtype.Timestamptz) which serialize
// to verbose objects like {"String":"...", "Valid":true} instead of plain strings.
// This response type uses standard Go types so the JSON output is clean.
type bankResponse struct {
	ID      uuid.UUID `json:"id"`
	OwnerID uuid.UUID `json:"owner_id"`
	Name    string    `json:"name"`
	// omitempty means the field is omitted entirely from JSON when nil,
	// rather than serializing as null.
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// bankFromStore converts a sqlc-generated QuestionBank into a bankResponse,
// unwrapping the pgtype fields into plain Go types along the way.
// pgtype.Text has a Valid bool (like Java's Optional or Python's None check)
// — we only set the description pointer when Valid is true.
func bankFromStore(b store.QuestionBank) bankResponse {
	resp := bankResponse{
		ID:        b.ID,
		OwnerID:   b.OwnerID,
		Name:      b.Name,
		CreatedAt: b.CreatedAt.Time, // pgtype.Timestamptz wraps a time.Time in a .Time field
		UpdatedAt: b.UpdatedAt.Time,
	}
	if b.Description.Valid {
		resp.Description = &b.Description.String
	}
	return resp
}

// --- Question request/response types ---

// mcChoice is a single option in a multiple-choice question.
// Correct flags the one right answer; only one choice per question may be true.
type mcChoice struct {
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}

// createQuestionRequest is the JSON body expected by POST /banks/{bankID}/questions.
type createQuestionRequest struct {
	// Type must be "text" or "multiple_choice".
	Type   string `json:"type"`
	Prompt string `json:"prompt"` // question text, enforced ≤ 500 chars in the handler
	Points int32  `json:"points"` // defaults to 1000 if omitted or ≤ 0

	// AcceptedAnswers is used for text questions — a list of valid responses
	// (primary spelling first, synonyms/alternates after). At least one is required.
	AcceptedAnswers []string `json:"accepted_answers"`

	// Choices is used for multiple-choice questions — 2–6 options, exactly one
	// with Correct: true.
	Choices []mcChoice `json:"choices"`
}

// updateQuestionRequest is the JSON body expected by PUT /banks/{bankID}/questions/{questionID}.
// The position field is intentionally excluded — use PATCH /questions/reorder for ordering.
type updateQuestionRequest struct {
	Prompt          string     `json:"prompt"`
	Points          int32      `json:"points"`
	AcceptedAnswers []string   `json:"accepted_answers"`
	Choices         []mcChoice `json:"choices"`
}

// reorderRequest is the body for PATCH /banks/{bankID}/questions/reorder.
// IDs lists every question UUID in the desired display order; their positions
// are updated to match their index in this slice.
type reorderRequest struct {
	IDs []string `json:"ids"`
}

// questionResponse is the clean API shape for a question.
// Like bankResponse, it avoids pgtype wrappers in the serialised JSON output.
type questionResponse struct {
	ID       uuid.UUID `json:"id"`
	BankID   uuid.UUID `json:"bank_id"`
	Type     string    `json:"type"`
	Prompt   string    `json:"prompt"`
	Points   int32     `json:"points"`
	Position int32     `json:"position"`

	// Only one of these is populated, determined by Type:
	AcceptedAnswers []string   `json:"accepted_answers,omitempty"` // text questions
	Choices         []mcChoice `json:"choices,omitempty"`          // MC questions

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// questionFromStore converts a store.Question into a questionResponse.
// The choices JSONB field is interpreted differently depending on question type:
//   - text:             choices contains a JSON string array of accepted answers
//   - multiple_choice:  choices contains a JSON array of mcChoice objects
//
// If choices is absent (legacy rows), we fall back to correct_answer for text type.
func questionFromStore(q store.Question) questionResponse {
	resp := questionResponse{
		ID:        q.ID,
		BankID:    q.BankID,
		Type:      string(q.Type),
		Prompt:    q.Prompt,
		Points:    q.Points,
		Position:  q.Position,
		CreatedAt: q.CreatedAt.Time,
		UpdatedAt: q.UpdatedAt.Time,
	}

	if len(q.Choices) > 0 {
		switch q.Type {
		case store.QuestionTypeText:
			var answers []string
			if err := json.Unmarshal(q.Choices, &answers); err == nil && len(answers) > 0 {
				resp.AcceptedAnswers = answers
			}
		case store.QuestionTypeMultipleChoice:
			var choices []mcChoice
			if err := json.Unmarshal(q.Choices, &choices); err == nil {
				resp.Choices = choices
			}
		}
	}

	// Fallback for text questions that were created before accepted_answers were stored.
	if q.Type == store.QuestionTypeText && len(resp.AcceptedAnswers) == 0 && q.CorrectAnswer != "" {
		resp.AcceptedAnswers = []string{q.CorrectAnswer}
	}

	return resp
}

// --- Quiz request/response types ---

type createQuizRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type updateQuizRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type createRoundRequest struct {
	Title *string `json:"title"`
}

type updateRoundRequest struct {
	Title *string `json:"title"`
}

type setRoundQuestionsRequest struct {
	// Ordered list of question UUIDs for this round.
	QuestionIDs []string `json:"question_ids"`
}

type quizResponse struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func quizFromStore(q store.Quiz) quizResponse {
	resp := quizResponse{
		ID:        q.ID,
		OwnerID:   q.OwnerID,
		Name:      q.Name,
		CreatedAt: q.CreatedAt.Time,
		UpdatedAt: q.UpdatedAt.Time,
	}
	if q.Description.Valid {
		resp.Description = &q.Description.String
	}
	return resp
}

type roundResponse struct {
	ID          uuid.UUID          `json:"id"`
	QuizID      uuid.UUID          `json:"quiz_id"`
	RoundNumber int32              `json:"round_number"`
	Title       *string            `json:"title,omitempty"`
	Questions   []questionResponse `json:"questions"`
	CreatedAt   time.Time          `json:"created_at"`
}

func roundFromStore(r store.QuizRound, questions []questionResponse) roundResponse {
	resp := roundResponse{
		ID:          r.ID,
		QuizID:      r.QuizID,
		RoundNumber: r.RoundNumber,
		Questions:   questions,
		CreatedAt:   r.CreatedAt.Time,
	}
	if r.Title.Valid {
		resp.Title = &r.Title.String
	}
	return resp
}

type quizDetailResponse struct {
	quizResponse
	Rounds []roundResponse `json:"rounds"`
}

// --- Game request/response types ---

// createGameRequest is the body for POST /games.
// Either bank_id (legacy) or quiz_id must be provided.
type createGameRequest struct {
	BankID    string `json:"bank_id"`    // legacy: UUID of a question bank
	RoundSize int32  `json:"round_size"` // legacy: questions per round
	QuizID    string `json:"quiz_id"`    // new: UUID of a quiz
}

// joinGameRequest is the body for POST /join (unauthenticated player endpoint).
type joinGameRequest struct {
	Code        string `json:"code"`         // 6-character game code
	DisplayName string `json:"display_name"` // player's chosen name, max 32 chars
}

// joinGameResponse is returned to the player after a successful join.
type joinGameResponse struct {
	GameCode     string `json:"game_code"`
	SessionToken string `json:"session_token"` // stored by the player; used for WebSocket auth
	DisplayName  string `json:"display_name"`
}

// gameResponse is the clean API shape for a game record.
type gameResponse struct {
	ID                 uuid.UUID  `json:"id"`
	Code               string     `json:"code"`
	Status             string     `json:"status"`
	BankID             *uuid.UUID `json:"bank_id,omitempty"`
	QuizID             *uuid.UUID `json:"quiz_id,omitempty"`
	CurrentQuestionIdx int32      `json:"current_question_idx"`
	CurrentRoundIdx    int32      `json:"current_round_idx"`
	RoundSize          int32      `json:"round_size"`
	CreatedAt          time.Time  `json:"created_at"`
}

func gameFromStore(g store.Game) gameResponse {
	resp := gameResponse{
		ID:                 g.ID,
		Code:               g.Code,
		Status:             string(g.Status),
		CurrentQuestionIdx: g.CurrentQuestionIdx,
		CurrentRoundIdx:    g.CurrentRoundIdx,
		RoundSize:          g.RoundSize,
		CreatedAt:          g.CreatedAt.Time,
	}
	if g.BankID.Valid {
		id := uuid.UUID(g.BankID.Bytes)
		resp.BankID = &id
	}
	if g.QuizID.Valid {
		id := uuid.UUID(g.QuizID.Bytes)
		resp.QuizID = &id
	}
	return resp
}

// nullText converts an optional *string into pgtype.Text for use in sqlc queries.
// pgtype.Text is Postgres's nullable text type — Valid=false means SQL NULL.
// When the caller passes nil (field not provided in the request), we produce
// an invalid (NULL) pgtype.Text. When a string is provided, we set it as valid.
func nullText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{} // zero value: Valid=false → SQL NULL
	}
	return pgtype.Text{String: *s, Valid: true}
}
