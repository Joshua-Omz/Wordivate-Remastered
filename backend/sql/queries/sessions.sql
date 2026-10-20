-- Purpose: Socratic learning sessions state queries.
-- Architecture: backend/sql/queries/sessions.sql

-- name: CreateSession :one
INSERT INTO learning_sessions (
    user_id,
    word_text,
    headword_key,
    status,
    context_note,
    snapshot_id,
    prompt_version,
    model_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetActiveSessionByWord :one
SELECT * FROM learning_sessions
WHERE user_id = $1 AND headword_key = $2 AND status = 'active';

-- name: GetSessionByID :one
SELECT * FROM learning_sessions
WHERE id = $1 AND user_id = $2;

-- name: UpdateSessionStatus :one
UPDATE learning_sessions
SET status = $2,
    last_active_at = now(),
    ended_at = CASE WHEN $2 IN ('committed', 'abandoned', 'failed') THEN now() ELSE ended_at END
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: UpdateSessionLastActive :one
UPDATE learning_sessions
SET last_active_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: LinkSessionToEntry :one
UPDATE learning_sessions
SET entry_id = $2,
    status = 'committed',
    ended_at = now(),
    last_active_at = now()
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: ListRecentSessionsByUser :many
SELECT * FROM learning_sessions
WHERE user_id = $1
ORDER BY last_active_at DESC
LIMIT $2;

-- name: AbandonInactiveSessions :execrows
UPDATE learning_sessions
SET status = 'abandoned',
    ended_at = now()
WHERE status = 'active'
  AND last_active_at < $1;

-- name: AbandonSession :one
UPDATE learning_sessions
SET status = 'abandoned',
    ended_at = now(),
    last_active_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'active'
RETURNING *;

-- name: ResumeSession :one
UPDATE learning_sessions
SET status = 'active',
    ended_at = NULL,
    last_active_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'abandoned'
RETURNING *;

