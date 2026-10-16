-- Purpose: Vocabulary library persistence and retrieval queries.
-- Architecture: backend/sql/queries/vocab_entries.sql

-- name: CreateVocabEntry :one
INSERT INTO vocab_entries (
    user_id,
    word_text,
    headword_key,
    sense_key,
    sense_label,
    part_of_speech,
    snapshot_id,
    official_definition,
    personal_definition,
    personal_def_origin,
    example_sentence,
    contextual_notes,
    understanding_flag,
    discrepancy_note,
    origin_session_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING *;

-- name: GetVocabEntryByID :one
SELECT * FROM vocab_entries
WHERE id = $1 AND user_id = $2;

-- name: GetVocabEntryByWordAndSense :one
SELECT * FROM vocab_entries
WHERE user_id = $1 AND headword_key = $2 AND sense_key = $3;

-- name: GetVocabEntriesByWord :many
SELECT * FROM vocab_entries
WHERE user_id = $1 AND headword_key = $2
ORDER BY created_at DESC;

-- name: ListVocabEntries :many
SELECT * FROM vocab_entries
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: SearchVocabEntries :many
SELECT * FROM vocab_entries
WHERE user_id = $1
  AND search_vector @@ websearch_to_tsquery('english', $2)
ORDER BY ts_rank(search_vector, websearch_to_tsquery('english', $2)) DESC
LIMIT $3 OFFSET $4;

-- name: UpdateVocabEntryPersonal :one
UPDATE vocab_entries
SET personal_definition = $3,
    example_sentence = $4,
    contextual_notes = $5,
    understanding_flag = $6,
    discrepancy_note = $7,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteVocabEntry :exec
DELETE FROM vocab_entries
WHERE id = $1 AND user_id = $2;
