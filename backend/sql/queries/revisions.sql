-- Purpose: Vocabulary entry audit revisions queries.
-- Architecture: backend/sql/queries/revisions.sql

-- name: CreateEntryRevision :one
INSERT INTO entry_revisions (
    entry_id,
    rev_no,
    changes
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: ListEntryRevisions :many
SELECT * FROM entry_revisions
WHERE entry_id = $1
ORDER BY rev_no ASC;

-- name: GetNextRevisionNo :one
SELECT COALESCE(MAX(rev_no), 0)::INT + 1 AS next_rev_no
FROM entry_revisions
WHERE entry_id = $1;
