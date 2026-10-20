# Wordivate Backend Development Roadmap & Sprint Distribution

This document defines the sequential sprint plan for implementing the Wordivate backend in Go. It translates the Software Requirements Specification (SRS) and architecture rules into five focused, testable, and pedagogically structured engineering milestones.

---

## Sprint Overview

```
+-------------------------------------------------------------------------+
| Sprint 1: Schema Overhaul & Type-Safe Database Core (sqlc)              |
| -> SRS Section 7.4 Schema, PostgreSQL Extensions, sqlc Queries          |
+------------------------------------+------------------------------------+
                                     |
                                     v
+-------------------------------------------------------------------------+
| Sprint 2: Authentication & Multi-Tenant Security Gateway                |
| -> User Registration, Login, JWT Middleware, Context Isolation          |
+------------------------------------+------------------------------------+
                                     |
                                     v
+-------------------------------------------------------------------------+
| Sprint 3: Word Capture & Lexical Definition Service                     |
| -> Normalization, Duplicate Checks, Dictionary Snapshots & Hashes       |
+------------------------------------+------------------------------------+
                                     |
                                     v
+-------------------------------------------------------------------------+
| Sprint 4: Socratic Conversation Engine & Real-Time Streaming            |
| -> Session State Machine, LLM Interface, Prompt Versioning, SSE Stream  |
+------------------------------------+------------------------------------+
                                     |
                                     v
+-------------------------------------------------------------------------+
| Sprint 5: Transactional Commit, Idempotency, & Vocabulary Library       |
| -> Atomic Commit Tx, Idempotency-Key, Personal Library, Full-Text Search|
+------------------------------------+------------------------------------+
                                     |
                                     v
+-------------------------------------------------------------------------+
| Sprint 6: Adaptive Recall & Spaced Repetition (SRS) Engine              |
| -> SM-2 Algorithm, Due Words Queue, Review Evaluations, Audit History  |
+-------------------------------------------------------------------------+
```

---

## Detailed Sprint Specifications

### Sprint 1: Schema Overhaul & Type-Safe Database Core (`sqlc`) [COMPLETED]
* **Documentation:** [`backend/docs/database/schema_and_sqlc.md`](./backend/docs/database/schema_and_sqlc.md)
* **Objective:** Replace legacy flashcard schema (`words` with ease factor) with the SRS-compliant relational architecture.
* **SRS Anchors:** Section 7.1 – 7.4 (Entities, Relationships, Proposed PostgreSQL Schema).
* **Deliverables:**
  1. `backend/sql/schema/001_init.sql`:
     - Extensions: `pgcrypto`, `pg_trgm`, `citext`.
     - Tables: `users`, `definition_snapshots`, `learning_sessions`, `messages`, `vocab_entries`, `entry_revisions`, `commit_assessments`, `idempotency_keys`.
     - Indexes: `one_active_session_per_word` unique partial index, full-text search GIN index on `search_vector`, trigram index on `word_text`.
  2. `backend/sql/queries/*.sql`:
     - Queries for sessions, messages, snapshots, vocab entries, revisions, and idempotency.
  3. `backend/internal/database/sqlc/`:
     - Regenerated Go query interfaces and structs using `sqlc generate` (11 generated files).
* **Mentorship & Learning Focus:**
  - PostgreSQL DDL, composite uniqueness, partial indexes, and generated `tsvector` columns.
  - How `sqlc` compiles raw SQL into type-safe Go structs without runtime reflection.
* **Acceptance Criteria:** `sqlc generate` succeeds with zero errors, and a fresh PostgreSQL database boots cleanly with all tables and constraints. [VERIFIED]

---

### Sprint 2: Authentication & Multi-Tenant Security Gateway
* **Objective:** Implement user registration, credential verification, and tenant-scoped request context.
* **SRS Anchors:** Section 2 (Roles), Section 3.5 (Account FR-040), Section 4.3 (Security NFR-S1, NFR-S2).
* **Deliverables:**
  1. `backend/internal/features/auth/`:
     - `service.go`: Password hashing with bcrypt, JWT issuing and validation.
     - `handler.go`: `POST /v1/auth/register`, `POST /v1/auth/login`.
  2. `backend/internal/middleware/auth.go`:
     - Middleware that intercepts `Authorization: Bearer <token>`, validates the JWT signature, extracts `user_id`, and injects it into Go's request `context.Context`.
* **Mentorship & Learning Focus:**
  - Idiomatic middleware chaining in Go using `http.Handler` and `chi`.
  - Type-safe context injection using custom unexported context key types (preventing key collisions).
  - Multi-tenant security: Why every database query must parameterize `WHERE user_id = $1`.
* **Acceptance Criteria:** Integration tests verify registration, login, token issuance, and refusal of unauthorized requests (HTTP 401).

---

### Sprint 3: Word Capture & Lexical Definition Service [COMPLETED]
* **Documentation:** [`backend/docs/capture/word_capture_and_lexical_service.md`](./backend/docs/capture/word_capture_and_lexical_service.md)
* **Objective:** Build the word entry point and external lexical grounding service with snapshot immutability.
* **SRS Anchors:** Section 3.1 (Capture FR-001 – FR-004), Section 9 (Definition Source DEF-001 – DEF-007).
* **Deliverables:**
  1. `backend/internal/features/capture/`:
     - Normalization logic: trim whitespace, case-fold, validate headword length limits.
     - Duplicate detection: check if headword already exists in `vocab_entries` for this user and return appropriate guidance.
  2. `backend/internal/features/capture/dictionary.go`:
     - `DictionaryClient` interface: abstracting third-party lexical APIs.
     - Fetcher & normalizer: retrieves definitions, senses, and parts of speech.
     - Snapshot engine: computes SHA-256 content hash and stores immutable payload in `definition_snapshots`.
  3. `POST /v1/sessions` and `GET /v1/words/lookup` endpoints: creates or resumes a session record in status `active` with attached `snapshot_id`.
* **Mentorship & Learning Focus:**
  - Interface segregation in Go: designing interfaces for external services to allow effortless testing and mocking.
  - Cryptographic hashing in Go (`crypto/sha256`) for immutable snapshot verification.
  - Handling HTTP client timeouts and graceful degradation when external APIs fail (ERR-003).
* **Acceptance Criteria:** Capturing "  Ephemeral " normalizes to "ephemeral", retrieves dictionary senses, persists an immutable snapshot, and creates an `active` session within 1.0 s (NFR-P1). [VERIFIED]

---

### Sprint 4: Socratic Conversation Engine & Real-Time Streaming [COMPLETED]
* **Documentation:** [`backend/docs/socratic/socratic_engine_and_sse.md`](./backend/docs/socratic/socratic_engine_and_sse.md)
* **Objective:** Implement the adaptive tutor engine, session state machine, and real-time streaming transport.
* **SRS Anchors:** Section 3.2 (Learning Session FR-010 – FR-018), Section 6 (AI Policy AI-001 – AI-008), Section 8.2.
* **Deliverables:**
  1. `backend/internal/features/socratic/`:
     - `LLMClient` interface: methods for streaming chat completions and structured outputs.
     - Session state machine: enforcing `active`, `committing`, `committed`, `abandoned`, `failed`.
     - Append-only message persistence: tracking turns with sequential integers (`seq`).
     - Prompt assembler: injects versioned prompt template (`prompt_version`), definition snapshot grounding, and conversation history.
     - Rule enforcers: checking max turn length (~120 words), single question limit, non-repetition, and 2-strike direct explanation fallback.
  2. `POST /v1/sessions/{id}/messages`: Server-Sent Events (SSE) streaming endpoint that writes assistant tokens in real time.
  3. Session inactivity tracker: marks sessions inactive for >30 minutes as `abandoned`.
* **Mentorship & Learning Focus:**
  - Go concurrency patterns: goroutines, channels, and context cancellation (`ctx.Done()`).
  - Server-Sent Events (SSE) implementation using Go's `http.Flusher` without external dependencies.
  - Structured prompt engineering and state machine modeling in Go.
* **Acceptance Criteria:** User message is persisted durably, assistant reply streams via SSE, and tutor enforces Socratic behavior without false confirmations. [VERIFIED]

---

### Sprint 5: Transactional Commit, Idempotency, & Vocabulary Library [COMPLETED]
* **Documentation:** [`backend/docs/commit_library/commit_and_vocabulary_library.md`](./backend/docs/commit_library/commit_and_vocabulary_library.md)
* **Objective:** Implement the atomic commit process, personal definition provenance validation, and the persistent vocabulary library.
* **SRS Anchors:** Section 3.3 (Commit FR-020 – FR-027), Section 3.4 (Library FR-030 – FR-036), Section 7.4.
* **Deliverables:**
  1. `POST /v1/sessions/{id}/commit-draft`:
     - Generates structured JSON assessment (`sufficient | partial | inaccurate`), discrepancy note, and draft personal definition derived strictly from user text (AI-011).
  2. `POST /v1/sessions/{id}/commit`:
     - Wrapped in a single PostgreSQL transaction (`sql.Tx`): updates session status, writes `vocab_entries`, writes `entry_revisions` (rev 1), writes `commit_assessments`.
     - Validates and stores `Idempotency-Key` to prevent duplicate writes on repeat requests.
  3. `backend/internal/features/library/`:
     - `GET /v1/words`: list with date range, part of speech, and understanding flag filtering.
     - Full-text search using PostgreSQL `search_vector` (`to_tsquery` and GIN index).
     - `GET /v1/words/{id}`: detail view showing official vs personal definitions side by side with conversation history.
     - `PATCH /v1/words/{id}`: edit personal fields, appending changes to `entry_revisions`.
     - `DELETE /v1/words/{id}`: permanent cascade deletion.
* **Mentorship & Learning Focus:**
  - Managing PostgreSQL transactions (`BEGIN`, `COMMIT`, `ROLLBACK`) safely in Go using `defer`.
  - Idempotent API design in distributed systems.
  - PostgreSQL full-text search (`tsvector`, `tsquery`, `setweight`) in Go.
* **Acceptance Criteria:** Committing a word executes atomically; repeated commit requests return the identical record without duplication; library search returns weighted results in < 500 ms (NFR-P3). [VERIFIED]

---

### Sprint 6: Adaptive Recall & Spaced Repetition (SRS) Engine [COMPLETED]
* **Documentation:** [`backend/docs/recall/adaptive_recall_engine.md`](./backend/docs/recall/adaptive_recall_engine.md)
* **Objective:** Implement the spaced repetition engine (SM-2 variant), multi-tenant queue, review evaluations, and Clean Architecture decoupling.
* **SRS Anchors:** Section 3.6 (Recall & Use FR-050 – FR-054), Section 7.4.
* **Deliverables:**
  1. `backend/sql/schema/002_recall.sql`:
     - Relational tables: `recall_schedules` (1:1 with `vocab_entries`) and `recall_reviews` (audit log).
     - Indexes: `recall_user_due (user_id, next_review_at ASC)` and `recall_reviews_entry`.
  2. `backend/sql/queries/recall.sql`:
     - Queries for due words retrieval, schedule upserts, review event logging, and lazy schedule seeding.
  3. `backend/internal/features/recall/`:
     - `models.go`: Domain models for due words, review submissions, and review results.
     - `repository.go`: Decoupled Clean Architecture persistence interface.
     - `service.go`: SM-2 algorithm implementation with ease factor clamping ($EF \ge 1.30$), interval expansion, and multi-tenant security verification.
     - `handler.go`: HTTP endpoints `GET /v1/recall/due` and `POST /v1/recall/reviews`.
     - `service_test.go` and `handler_test.go`: Unit test suite (>90% service coverage, 100% handler coverage).
  4. Visual Architecture Flow:
     - [`backend/docs/diagrams/recall_spaced_repetition_flow.svg`](./backend/docs/diagrams/recall_spaced_repetition_flow.svg)
* **Mentorship & Learning Focus:**
  - Cognitive science of spaced retrieval practice (Ebbinghaus forgetting curve).
  - SuperMemo-2 mathematical modeling and floating-point rounding safety in Go.
  - Eliminating architectural violations via the Repository Pattern in Clean Architecture.
  - Zero-worker lazy on-demand seeding vs background worker queues.
* **Acceptance Criteria:** Due words queue returns entries ordered by urgency; review submissions calculate next review via SM-2 and update schedule atomically; whole test suite passes with zero regressions. [VERIFIED]

---

## Execution Governance
All sprints will be executed in sequential lock under the **AI-Assisted Development Operating Rules**:
1. Each sprint task begins with an Intent Declaration (Category, Goal, Scope, Out of Scope).
2. All code conforms to the Mandatory Commenting Standard and idiomatic Go conventions.
3. Every step includes pedagogical explanations (foundations before syntax, analogies, trade-offs).
