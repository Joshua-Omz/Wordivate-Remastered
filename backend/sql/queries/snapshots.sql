-- Purpose: Lexical definition snapshots queries (immutable dictionary data).
-- Architecture: backend/sql/queries/snapshots.sql

-- name: CreateDefinitionSnapshot :one
INSERT INTO definition_snapshots (
    headword_key,
    source,
    source_ref,
    payload,
    content_hash
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (source, headword_key, content_hash) 
DO UPDATE SET retrieved_at = now()
RETURNING *;

-- name: GetDefinitionSnapshotByID :one
SELECT * FROM definition_snapshots
WHERE id = $1;

-- name: GetDefinitionSnapshotByHash :one
SELECT * FROM definition_snapshots
WHERE source = $1 AND headword_key = $2 AND content_hash = $3;
