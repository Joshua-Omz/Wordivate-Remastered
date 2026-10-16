-- Purpose: Chat messages queries for Socratic tutoring sessions.
-- Architecture: backend/sql/queries/messages.sql

-- name: CreateMessage :one
INSERT INTO messages (
    session_id,
    seq,
    role,
    content,
    token_count
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: ListMessagesBySession :many
SELECT * FROM messages
WHERE session_id = $1
ORDER BY seq ASC;

-- name: GetLastMessageSeq :one
SELECT COALESCE(MAX(seq), 0)::INT AS last_seq
FROM messages
WHERE session_id = $1;
