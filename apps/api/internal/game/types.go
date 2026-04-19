package game

import (
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
