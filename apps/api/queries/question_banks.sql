-- name: CreateQuestionBank :one
INSERT INTO question_banks (id, owner_id, name, description)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetQuestionBank :one
SELECT * FROM question_banks WHERE id = $1;

-- name: ListQuestionBanksByOwner :many
SELECT * FROM question_banks
WHERE owner_id = $1
ORDER BY created_at DESC;

-- name: UpdateQuestionBank :one
UPDATE question_banks
SET name        = $2,
    description = $3
WHERE id = $1
RETURNING *;

-- name: DeleteQuestionBank :exec
DELETE FROM question_banks WHERE id = $1;

-- GetDefaultBank returns the owner's implicit "default" bank, if one exists.
-- Returns no rows when the user has not created a question inline yet.
-- name: GetDefaultBank :one
SELECT * FROM question_banks
WHERE owner_id = $1 AND is_default;

-- CreateDefaultBank inserts the owner's default bank. The partial unique index
-- question_banks_one_default_per_owner guarantees at most one per owner, so a
-- concurrent insert raises a unique violation rather than creating a duplicate.
-- name: CreateDefaultBank :one
INSERT INTO question_banks (id, owner_id, name, is_default)
VALUES ($1, $2, $3, true)
RETURNING *;
