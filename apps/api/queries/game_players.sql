-- name: AddPlayer :one
INSERT INTO game_players (id, game_id, display_name, session_token)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPlayer :one
SELECT * FROM game_players WHERE id = $1;

-- name: GetPlayerBySessionToken :one
SELECT * FROM game_players WHERE session_token = $1;

-- name: ListPlayersInGame :many
SELECT * FROM game_players
WHERE game_id = $1
ORDER BY joined_at ASC;

-- name: ListActivePlayersInGame :many
SELECT * FROM game_players
WHERE game_id = $1
  AND left_at IS NULL
ORDER BY joined_at ASC;

-- name: AddScoreToPlayer :one
UPDATE game_players
SET score = score + $2
WHERE id = $1
RETURNING *;

-- name: SetPlayerScore :one
UPDATE game_players
SET score = $2
WHERE id = $1
RETURNING *;

-- name: MarkPlayerLeft :exec
UPDATE game_players
SET left_at = now()
WHERE id = $1
  AND left_at IS NULL;

-- name: ClearPlayerLeft :exec
UPDATE game_players
SET left_at = NULL
WHERE id = $1;

-- name: LeaderboardForGame :many
SELECT id, display_name, score
FROM game_players
WHERE game_id = $1
ORDER BY score DESC, joined_at ASC;
