-- Purpose: Relational schema for Adaptive Recall & Spaced Repetition (SRS) Engine (Sprint 6).
-- Architecture: backend/sql/schema/002_recall.sql (PostgreSQL DDL)
-- Dependencies: vocab_entries, users

-- ============================================================================
-- 1. RECALL SCHEDULES
-- ============================================================================

-- recall_schedules tracks the current spaced repetition state per vocabulary entry.
-- Follows SM-2 interval expansion, ease factor clamping, and repetition counts.
CREATE TABLE recall_schedules (
    entry_id         UUID PRIMARY KEY REFERENCES vocab_entries(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    interval_days    INT NOT NULL DEFAULT 1,
    ease_factor      FLOAT8 NOT NULL DEFAULT 2.5,
    repetition_count INT NOT NULL DEFAULT 0,
    next_review_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_reviewed_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast lookup for due items per user ordered by urgency (SRS RET-001).
CREATE INDEX recall_user_due
    ON recall_schedules (user_id, next_review_at ASC);

-- ============================================================================
-- 2. RECALL REVIEWS (AUDIT LOG)
-- ============================================================================

-- recall_reviews stores immutable history of learner recall evaluations.
CREATE TABLE recall_reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id    UUID NOT NULL REFERENCES vocab_entries(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating      INT NOT NULL CHECK (rating BETWEEN 0 AND 5),
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast lookup of past review history for a vocabulary item.
CREATE INDEX recall_reviews_entry
    ON recall_reviews (entry_id, reviewed_at DESC);
