// Manually written quiz store queries (quiz tables were added after initial sqlc generation).

package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---- Quiz CRUD ---------------------------------------------------------------

const createQuiz = `
INSERT INTO quizzes (id, owner_id, name, description)
VALUES ($1, $2, $3, $4)
RETURNING id, owner_id, name, description, created_at, updated_at
`

type CreateQuizParams struct {
	ID          uuid.UUID   `json:"id"`
	OwnerID     uuid.UUID   `json:"owner_id"`
	Name        string      `json:"name"`
	Description pgtype.Text `json:"description"`
}

func (q *Queries) CreateQuiz(ctx context.Context, arg CreateQuizParams) (Quiz, error) {
	row := q.db.QueryRow(ctx, createQuiz, arg.ID, arg.OwnerID, arg.Name, arg.Description)
	var i Quiz
	err := row.Scan(&i.ID, &i.OwnerID, &i.Name, &i.Description, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const getQuizByID = `
SELECT id, owner_id, name, description, created_at, updated_at
FROM quizzes WHERE id = $1
`

func (q *Queries) GetQuizByID(ctx context.Context, id uuid.UUID) (Quiz, error) {
	row := q.db.QueryRow(ctx, getQuizByID, id)
	var i Quiz
	err := row.Scan(&i.ID, &i.OwnerID, &i.Name, &i.Description, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const listQuizzesByOwner = `
SELECT id, owner_id, name, description, created_at, updated_at
FROM quizzes
WHERE owner_id = $1
ORDER BY created_at DESC
`

func (q *Queries) ListQuizzesByOwner(ctx context.Context, ownerID uuid.UUID) ([]Quiz, error) {
	rows, err := q.db.Query(ctx, listQuizzesByOwner, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Quiz
	for rows.Next() {
		var i Quiz
		if err := rows.Scan(&i.ID, &i.OwnerID, &i.Name, &i.Description, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

const updateQuiz = `
UPDATE quizzes
SET name = $2, description = $3, updated_at = now()
WHERE id = $1
RETURNING id, owner_id, name, description, created_at, updated_at
`

type UpdateQuizParams struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	Description pgtype.Text `json:"description"`
}

func (q *Queries) UpdateQuiz(ctx context.Context, arg UpdateQuizParams) (Quiz, error) {
	row := q.db.QueryRow(ctx, updateQuiz, arg.ID, arg.Name, arg.Description)
	var i Quiz
	err := row.Scan(&i.ID, &i.OwnerID, &i.Name, &i.Description, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const deleteQuiz = `DELETE FROM quizzes WHERE id = $1`

func (q *Queries) DeleteQuiz(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteQuiz, id)
	return err
}

// ---- Quiz Rounds -------------------------------------------------------------

const createQuizRound = `
INSERT INTO quiz_rounds (id, quiz_id, round_number, title)
VALUES ($1, $2, $3, $4)
RETURNING id, quiz_id, round_number, title, created_at, updated_at
`

type CreateQuizRoundParams struct {
	ID          uuid.UUID   `json:"id"`
	QuizID      uuid.UUID   `json:"quiz_id"`
	RoundNumber int32       `json:"round_number"`
	Title       pgtype.Text `json:"title"`
}

func (q *Queries) CreateQuizRound(ctx context.Context, arg CreateQuizRoundParams) (QuizRound, error) {
	row := q.db.QueryRow(ctx, createQuizRound, arg.ID, arg.QuizID, arg.RoundNumber, arg.Title)
	var i QuizRound
	err := row.Scan(&i.ID, &i.QuizID, &i.RoundNumber, &i.Title, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const listQuizRounds = `
SELECT id, quiz_id, round_number, title, created_at, updated_at
FROM quiz_rounds
WHERE quiz_id = $1
ORDER BY round_number ASC
`

func (q *Queries) ListQuizRounds(ctx context.Context, quizID uuid.UUID) ([]QuizRound, error) {
	rows, err := q.db.Query(ctx, listQuizRounds, quizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []QuizRound
	for rows.Next() {
		var i QuizRound
		if err := rows.Scan(&i.ID, &i.QuizID, &i.RoundNumber, &i.Title, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

const getQuizRound = `
SELECT id, quiz_id, round_number, title, created_at, updated_at
FROM quiz_rounds WHERE id = $1
`

func (q *Queries) GetQuizRound(ctx context.Context, id uuid.UUID) (QuizRound, error) {
	row := q.db.QueryRow(ctx, getQuizRound, id)
	var i QuizRound
	err := row.Scan(&i.ID, &i.QuizID, &i.RoundNumber, &i.Title, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const updateQuizRound = `
UPDATE quiz_rounds
SET title = $2, updated_at = now()
WHERE id = $1
RETURNING id, quiz_id, round_number, title, created_at, updated_at
`

func (q *Queries) UpdateQuizRound(ctx context.Context, id uuid.UUID, title pgtype.Text) (QuizRound, error) {
	row := q.db.QueryRow(ctx, updateQuizRound, id, title)
	var i QuizRound
	err := row.Scan(&i.ID, &i.QuizID, &i.RoundNumber, &i.Title, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

const deleteQuizRound = `DELETE FROM quiz_rounds WHERE id = $1`

func (q *Queries) DeleteQuizRound(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteQuizRound, id)
	return err
}

const countQuizRounds = `SELECT COUNT(*) FROM quiz_rounds WHERE quiz_id = $1`

func (q *Queries) CountQuizRounds(ctx context.Context, quizID uuid.UUID) (int32, error) {
	var n int32
	err := q.db.QueryRow(ctx, countQuizRounds, quizID).Scan(&n)
	return n, err
}

// ---- Round Questions ---------------------------------------------------------

const addQuestionToRound = `
INSERT INTO quiz_round_questions (round_id, question_id, position)
VALUES ($1, $2, $3)
ON CONFLICT (round_id, question_id) DO UPDATE SET position = EXCLUDED.position
`

type AddQuestionToRoundParams struct {
	RoundID    uuid.UUID `json:"round_id"`
	QuestionID uuid.UUID `json:"question_id"`
	Position   int32     `json:"position"`
}

func (q *Queries) AddQuestionToRound(ctx context.Context, arg AddQuestionToRoundParams) error {
	_, err := q.db.Exec(ctx, addQuestionToRound, arg.RoundID, arg.QuestionID, arg.Position)
	return err
}

const removeQuestionFromRound = `
DELETE FROM quiz_round_questions WHERE round_id = $1 AND question_id = $2
`

func (q *Queries) RemoveQuestionFromRound(ctx context.Context, roundID, questionID uuid.UUID) error {
	_, err := q.db.Exec(ctx, removeQuestionFromRound, roundID, questionID)
	return err
}

const countQuestionsInRound = `SELECT COUNT(*) FROM quiz_round_questions WHERE round_id = $1`

func (q *Queries) CountQuestionsInRound(ctx context.Context, roundID uuid.UUID) (int32, error) {
	var n int32
	err := q.db.QueryRow(ctx, countQuestionsInRound, roundID).Scan(&n)
	return n, err
}

// ListRoundQuestions returns the full question rows for a round, ordered by position.
const listRoundQuestions = `
SELECT q.id, q.bank_id, q.type, q.prompt, q.correct_answer, q.choices, q.points, q.position, q.created_at, q.updated_at
FROM quiz_round_questions rq
JOIN questions q ON q.id = rq.question_id
WHERE rq.round_id = $1
ORDER BY rq.position ASC
`

func (q *Queries) ListRoundQuestions(ctx context.Context, roundID uuid.UUID) ([]Question, error) {
	rows, err := q.db.Query(ctx, listRoundQuestions, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Question
	for rows.Next() {
		var i Question
		if err := rows.Scan(
			&i.ID, &i.BankID, &i.Type, &i.Prompt,
			&i.CorrectAnswer, &i.Choices, &i.Points, &i.Position,
			&i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// ListQuizRoundsWithQuestions loads all rounds for a quiz, each with their
// ordered questions. Used by the hub at game start to preload all quiz content.
func (q *Queries) ListQuizRoundsWithQuestions(ctx context.Context, quizID uuid.UUID) ([]RoundWithQuestions, error) {
	rounds, err := q.ListQuizRounds(ctx, quizID)
	if err != nil {
		return nil, err
	}

	result := make([]RoundWithQuestions, len(rounds))
	for i, r := range rounds {
		qs, err := q.ListRoundQuestions(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		result[i] = RoundWithQuestions{Round: r, Questions: qs}
	}
	return result, nil
}

// SetRoundQuestionsOrdered replaces all questions for a round with the given
// ordered list. Each question's position is its index in the slice.
func (q *Queries) SetRoundQuestionsOrdered(ctx context.Context, roundID uuid.UUID, questionIDs []uuid.UUID) error {
	// Delete existing assignments for this round.
	if _, err := q.db.Exec(ctx, `DELETE FROM quiz_round_questions WHERE round_id = $1`, roundID); err != nil {
		return err
	}
	// Insert in order.
	for pos, qID := range questionIDs {
		if err := q.AddQuestionToRound(ctx, AddQuestionToRoundParams{
			RoundID:    roundID,
			QuestionID: qID,
			Position:   int32(pos),
		}); err != nil {
			return err
		}
	}
	return nil
}
