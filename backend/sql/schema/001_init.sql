-- Purpose: Core relational schema for Wordivate matching Software Requirements Specification (SRS) Section 7.4.
-- Architecture: backend/sql/schema/001_init.sql (PostgreSQL DDL)
-- Dependencies: pgcrypto (UUIDs), pg_trgm (fuzzy/trigram search), citext (case-insensitive emails)

-- ============================================================================
-- 1. EXTENSIONS
-- ============================================================================

-- pgcrypto provides gen_random_uuid() for cryptographically secure UUID v4 primary keys.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- pg_trgm enables trigram matching for fast, fuzzy searching on headwords.
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- citext provides case-insensitive string matching for email addresses without lowercase() wrappers.
CREATE EXTENSION IF NOT EXISTS "citext";

-- ============================================================================
-- 2. IDENTITY & ACCOUNTS
-- ============================================================================

-- users holds learner accounts and authentication credentials.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT NOT NULL UNIQUE,
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- ============================================================================
-- 3. LEXICAL DEFINITIONS (IMMUTABLE SNAPSHOTS)
-- ============================================================================

-- definition_snapshots stores shared, immutable dictionary payloads.
-- Once recorded with a content_hash, a snapshot never changes, preserving lexical integrity
-- even if third-party dictionary definitions evolve in the future (SRS DEF-002, DEF-007).
CREATE TABLE definition_snapshots (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    headword_key TEXT NOT NULL,
    source       TEXT NOT NULL,
    source_ref   TEXT,
    payload      JSONB NOT NULL,
    content_hash BYTEA NOT NULL,
    retrieved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, headword_key, content_hash)
);

-- ============================================================================
-- 4. LEARNING SESSIONS & CHAT HISTORY
-- ============================================================================

-- learning_sessions manages the lifecycle of interactive Socratic tutoring sessions.
-- Lifecycle states: active -> committing -> committed | abandoned | failed (SRS 6.2).
CREATE TABLE learning_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_id        UUID, -- Linked to vocab_entries once committed (foreign key added below)
    word_text       TEXT NOT NULL,
    headword_key    TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('active', 'committing', 'committed', 'abandoned', 'failed')),
    context_note    TEXT,
    snapshot_id     UUID REFERENCES definition_snapshots(id),
    prompt_version  TEXT NOT NULL,
    model_id        TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ
);

-- Enforce exactly one active session per user per normalized word (SRS SES-001).
CREATE UNIQUE INDEX one_active_session_per_word
    ON learning_sessions (user_id, headword_key) WHERE status = 'active';

-- Fast lookup for user's recent sessions for resume and history queries.
CREATE INDEX sessions_user_recent
    ON learning_sessions (user_id, last_active_at DESC);

-- messages stores append-only turns of the Socratic dialogue.
-- Messages are strictly ordered by sequence number (seq) to prevent race conditions in SSE streaming.
CREATE TABLE messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  UUID NOT NULL REFERENCES learning_sessions(id) ON DELETE CASCADE,
    seq         INT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content     TEXT NOT NULL,
    token_count INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, seq)
);

-- ============================================================================
-- 5. VOCABULARY LIBRARY (COMMITTED KNOWLEDGE)
-- ============================================================================

-- vocab_entries represents persistent, user-specific vocabulary items.
-- Core Invariant: official_definition and personal_definition are stored separately and NEVER merged (FR-023).
CREATE TABLE vocab_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    word_text           TEXT NOT NULL,
    headword_key        TEXT NOT NULL,
    sense_key           TEXT NOT NULL DEFAULT '',
    sense_label         TEXT,
    part_of_speech      TEXT,
    snapshot_id         UUID NOT NULL REFERENCES definition_snapshots(id),
    official_definition TEXT NOT NULL,
    personal_definition TEXT NOT NULL,
    personal_def_origin TEXT NOT NULL CHECK (personal_def_origin IN ('user_written', 'user_approved_ai_edit')),
    example_sentence    TEXT,
    contextual_notes    TEXT,
    understanding_flag  TEXT NOT NULL DEFAULT 'ok' CHECK (understanding_flag IN ('ok', 'needs_review')),
    discrepancy_note    TEXT,
    origin_session_id   UUID NOT NULL REFERENCES learning_sessions(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Generated full-text search vector with weighted columns (A: word, B: personal def, C: notes).
    search_vector       TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', word_text), 'A') ||
        setweight(to_tsvector('english', personal_definition), 'B') ||
        setweight(to_tsvector('english', coalesce(contextual_notes, '')), 'C')
    ) STORED,
    UNIQUE (user_id, headword_key, sense_key)
);

-- Fast lookup of user's library ordered by creation date.
CREATE INDEX entries_user_created ON vocab_entries (user_id, created_at DESC);

-- GIN index on search_vector for high-performance sub-500ms full-text search (NFR-P3).
CREATE INDEX entries_search ON vocab_entries USING GIN (search_vector);

-- Trigram index on word_text for substring and fuzzy matching.
CREATE INDEX entries_word_trgm ON vocab_entries USING GIN (word_text gin_trgm_ops);

-- Circular foreign key: sessions can link to the entry they created.
ALTER TABLE learning_sessions
    ADD CONSTRAINT sessions_entry_fk
    FOREIGN KEY (entry_id) REFERENCES vocab_entries(id) ON DELETE SET NULL;

-- ============================================================================
-- 6. AUDIT HISTORY & AI COMMIT ASSESSMENTS
-- ============================================================================

-- entry_revisions retains post-commit edit history; revision 1 holds original committed values (FR-034).
CREATE TABLE entry_revisions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id   UUID NOT NULL REFERENCES vocab_entries(id) ON DELETE CASCADE,
    rev_no     INT NOT NULL,
    changes    JSONB NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entry_id, rev_no)
);

-- commit_assessments stores the AI tutor's verdict and structured reasoning at commit time.
CREATE TABLE commit_assessments (
    session_id       UUID PRIMARY KEY REFERENCES learning_sessions(id) ON DELETE CASCADE,
    verdict          TEXT NOT NULL CHECK (verdict IN ('sufficient', 'partial', 'inaccurate')),
    discrepancy_note TEXT,
    raw_output       JSONB NOT NULL,
    prompt_version   TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- 7. IDEMPOTENCY SAFETY
-- ============================================================================

-- idempotency_keys guarantees that repeated mutations (especially commits) produce no duplicates (FR-027).
CREATE TABLE idempotency_keys (
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key          TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    response     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);
