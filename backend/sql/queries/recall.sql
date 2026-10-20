-- Purpose: Spaced repetition (SRS) queries for due words, schedule updates, and review audits.
-- Architecture: backend/sql/queries/recall.sql

-- name: GetDueRecallEntries :many
SELECT 
    rs.entry_id,
    rs.user_id,
    rs.interval_days,
    rs.ease_factor,
    rs.repetition_count,
    rs.next_review_at,
    rs.last_reviewed_at,
    ve.word_text,
    ve.headword_key,
    ve.part_of_speech,
    ve.official_definition,
    ve.personal_definition,
    ve.example_sentence,
    ve.contextual_notes,
    ve.understanding_flag
FROM recall_schedules rs
JOIN vocab_entries ve ON rs.entry_id = ve.id
WHERE rs.user_id = $1
  AND rs.next_review_at <= $2
ORDER BY rs.next_review_at ASC
LIMIT $3;

-- name: GetRecallSchedule :one
SELECT * FROM recall_schedules
WHERE entry_id = $1 AND user_id = $2;

-- name: UpsertRecallSchedule :one
INSERT INTO recall_schedules (
    entry_id,
    user_id,
    interval_days,
    ease_factor,
    repetition_count,
    next_review_at,
    last_reviewed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (entry_id) DO UPDATE SET
    interval_days = EXCLUDED.interval_days,
    ease_factor = EXCLUDED.ease_factor,
    repetition_count = EXCLUDED.repetition_count,
    next_review_at = EXCLUDED.next_review_at,
    last_reviewed_at = EXCLUDED.last_reviewed_at
RETURNING *;

-- name: CreateRecallReview :one
INSERT INTO recall_reviews (
    entry_id,
    user_id,
    rating,
    reviewed_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: EnsureSchedulesForUser :exec
INSERT INTO recall_schedules (
    entry_id,
    user_id,
    interval_days,
    ease_factor,
    repetition_count,
    next_review_at
)
SELECT 
    ve.id,
    ve.user_id,
    1,
    2.5,
    0,
    now()
FROM vocab_entries ve
LEFT JOIN recall_schedules rs ON ve.id = rs.entry_id
WHERE ve.user_id = $1 AND rs.entry_id IS NULL
ON CONFLICT (entry_id) DO NOTHING;

-- name: GetReviewHistoryForEntry :many
SELECT * FROM recall_reviews
WHERE entry_id = $1 AND user_id = $2
ORDER BY reviewed_at DESC
LIMIT $3;
