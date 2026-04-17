-- name: CreateWord :one
INSERT INTO words (user_id, term, definition)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetDueWords :many
SELECT * FROM words
WHERE user_id = $1 AND next_review <= NOW()
ORDER BY next_review ASC;

-- name: UpdateWordSRS :one
UPDATE words
SET next_review = $2,
    interval    = $3,
    ease_factor = $4
WHERE id = $1
RETURNING *;
