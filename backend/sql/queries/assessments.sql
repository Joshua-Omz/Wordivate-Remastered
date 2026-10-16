-- Purpose: Commit-time AI assessment queries.
-- Architecture: backend/sql/queries/assessments.sql

-- name: CreateCommitAssessment :one
INSERT INTO commit_assessments (
    session_id,
    verdict,
    discrepancy_note,
    raw_output,
    prompt_version
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetCommitAssessmentBySession :one
SELECT * FROM commit_assessments
WHERE session_id = $1;
