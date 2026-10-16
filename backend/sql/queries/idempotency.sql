-- Purpose: Idempotency key tracking queries for safe retries of mutating requests.
-- Architecture: backend/sql/queries/idempotency.sql

-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys
WHERE user_id = $1 AND key = $2;

-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (
    user_id,
    key,
    request_hash
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: UpdateIdempotencyResponse :one
UPDATE idempotency_keys
SET response = $3
WHERE user_id = $1 AND key = $2
RETURNING *;
