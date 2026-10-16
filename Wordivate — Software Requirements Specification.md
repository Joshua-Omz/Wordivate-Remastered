# Wordivate — Software Requirements Specification

Version 0.1 (draft) · Author: Joshua · Status: for review

Legend: **\[C\]** = confirmed in the product description. **\[A\]** = assumption. **\[P\]** = proposed solution. **\[Q\]** = open question.

## 1. Product Scope

### 1.1 Purpose

Wordivate is a personal vocabulary app that turns word lookup into an AI-guided learning session, so users understand a word well enough to use it naturally. \[C\]

### 1.2 Goals

- G1: Capture a word with minimal friction. \[C\]
- G2: Guide the user to genuine understanding through an adaptive, Socratic chat. \[C\]
- G3: Build a personal definition from the user's own understanding, kept distinct from the authoritative definition. \[C\]
- G4: Persist committed words, definitions, conversations and examples in a personal library with search and filtering. \[C\]
- G5: Keep the interface focused on word, conversation and resulting knowledge. \[C\]

### 1.3 Assumptions

- A1: Mobile-first Flutter client with a Go backend and PostgreSQL. \[A\]
- A2: English is the only supported word and conversation language in v1. \[A\]
- A3: Users must have an account to keep a persistent library. \[A\]
- A4: A network connection is required for AI sessions; the library is readable offline only if caching is added later. \[A\]
- A5: One LLM provider is used behind an abstraction layer. \[A\]

### 1.4 Non-Goals (v1)

Social features, leaderboards, gamification, streaks, shared libraries, spaced-repetition scheduling, multi-language learning, voice input, and unrelated productivity features. Recall and usage features are acknowledged as future work (see Section 12). \[C\]

## 2. User Roles and Permissions

| Role | Description | Permissions |
| --- | --- | --- |
| Guest \[A\] | Unauthenticated visitor | View landing/sign-in only |
| Learner | Authenticated user | Create, read, update (notes/examples), delete own sessions and words; search own library; export own data \[A\] |
| Administrator \[A\] | Operator | Manage system config, prompt versions, usage limits; no access to learner conversation content except under audited support access |

Learners can only access their own data. \[C: user-specific library\]

## 3. Functional Requirements

Priority: M = must, S = should, C = could.

### 3.1 Capture

- **FR-001 (M)** The home screen provides a single input to enter a word and start a session. *Acceptance:* a session can be started in at most two interactions after the app opens (type word, confirm).
- **FR-002 (M)** Capture requires only the word. Optional context (where encountered, source sentence) may be added but never blocks start. *Acceptance:* no other field is mandatory.
- **FR-003 (M)** The word is normalized (trim, case-fold for matching, reject empty or multi-word input beyond a configured limit). *Acceptance:* "  Ephemeral " and "ephemeral" resolve to the same word key. Phrases are handled per Q3.
- **FR-004 (M)** On capture, the system checks for an existing committed entry and applies the duplicate rule (Section 10).

### 3.2 Learning Session

- **FR-010 (M)** Starting a session creates a Session record in status `active` and fetches definition data (Section 9) before or alongside the first AI turn. *Acceptance:* session exists with word and timestamp.
- **FR-011 (M)** The chat displays the word persistently and a visible Commit action. *Acceptance:* both are visible on the chat screen at all times.
- **FR-012 (M)** The AI opens with an adaptive turn (not a fixed script) and conducts a Socratic conversation guided by the instructional policy in Section 6.
- **FR-013 (M)** The AI recognizes when the user needs a direct explanation and gives one instead of another question. *Acceptance:* if the user says "just tell me" or fails two consecutive probes, the next AI turn contains an explanation.
- **FR-014 (M)** The AI does not repeat an identical question already asked in the session. *Acceptance:* automated check on question similarity across turns.
- **FR-015 (M)** The AI asks the user to write an original sentence where appropriate and evaluates it, explaining whether it conveys the intended meaning. *Acceptance:* evaluator output includes verdict and reasoning.
- **FR-016 (S)** The AI signals when the learning objective appears met and offers to commit, without forcing it. *Acceptance:* offer appears at most once per session unless the user declines and later shows new understanding.
- **FR-017 (M)** All user and assistant messages are persisted as they occur. *Acceptance:* force-closing the app does not lose acknowledged messages.
- **FR-018 (M)** Users may abandon a session; it is retained as `abandoned` and can be resumed (Section 10).

### 3.3 Commit

- **FR-020 (M)** The Commit action is available at any time; the user decides when to end. \[C\]
- **FR-021 (M)** On Commit the system generates a draft learning record containing: word, official definition, personal definition, example sentence (if any), contextual notes, conversation reference, metadata. \[C\]
- **FR-022 (M)** The personal definition is derived from the user's own statements in the session. If the user never expressed an understanding, the system prompts the user to write one rather than generating one. *Acceptance:* every stored personal definition traces to user-authored text; AI may help edit only with user approval.
- **FR-023 (M)** The official definition and personal definition are stored as separate fields and never merged. \[C\]
- **FR-024 (M)** Before saving, the system compares the personal definition to the official definition. If it appears inaccurate, the AI explains the discrepancy and offers correction or practice. \[C\] The user may still commit; the record is then flagged `understanding_flag = needs_review` and the discrepancy note is shown. \[P\]
- **FR-025 (M)** The user can review and edit the personal definition, example and notes in a confirmation step before the record is saved. \[P\]
- **FR-026 (M)** Commit is atomic: either the full record and session status update persist, or neither does. *Acceptance:* injected failure mid-commit leaves no partial record.
- **FR-027 (M)** Commit is idempotent: repeating the request does not create duplicates.

### 3.4 Library

- **FR-030 (M)** A persistent list of the user's committed words, default order newest first.
- **FR-031 (M)** Word detail view shows official and personal definitions side by side, example sentences, contextual notes and the full learning conversation. \[C\]
- **FR-032 (M)** Search by word text, and by text in personal definition and notes. *Acceptance:* results returned for partial matches.
- **FR-033 (S)** Filter by creation date range, understanding flag, and part of speech. \[A\]
- **FR-034 (S)** Edit personal definition, example and notes after commit; the original committed text is retained in history. \[P\]
- **FR-035 (M)** Delete a word and its conversation permanently on request.
- **FR-036 (S)** Resume learning on a committed word by starting a follow-up session linked to the entry. \[Q4\]

### 3.5 Account

- **FR-040 (M)** Sign up, sign in, sign out, and delete account with all data. \[A\]
- **FR-041 (S)** Export the user's library as JSON or CSV. \[A\]

## 4. Non-Functional Requirements

### 4.1 Performance

- **NFR-P1** Capture-to-chat-visible in under 1 s at p95 (excluding AI latency).
- **NFR-P2** First AI token within 3 s at p95; show a typing/streaming indicator immediately. \[A\]
- **NFR-P3** Library list and search respond within 500 ms at p95 for 5,000 words per user.
- **NFR-P4** Commit completes within 5 s at p95 including draft generation.

### 4.2 Reliability

- **NFR-R1** Message persistence is durable; no acknowledged message lost.
- **NFR-R2** Backend availability target 99.5% monthly. \[A\]
- **NFR-R3** AI and dictionary provider failures degrade gracefully (Section 6.4).
- **NFR-R4** Daily database backups with tested restore. \[A\]

### 4.3 Security

- **NFR-S1** TLS for all traffic; encryption at rest for the database.
- **NFR-S2** Authorization enforced server-side on every resource; users cannot access others' data.
- **NFR-S3** LLM and dictionary API keys held server-side only; the client never calls providers directly.
- **NFR-S4** Rate limiting per user on session start and message send.
- **NFR-S5** Prompt-injection resilience: user text is treated as data; the model cannot alter stored records except through defined server-side actions.

### 4.4 Privacy

- **NFR-PR1** Learning conversations are private to the user.
- **NFR-PR2** Data sent to the LLM provider is limited to the word, definition data and conversation; no account identifiers. \[P\]
- **NFR-PR3** The privacy notice discloses third-party processing; provider no-training terms are confirmed before launch. \[Q\]
- **NFR-PR4** Account deletion removes all user data within 30 days including backups rotation. \[A\]

### 4.5 Accessibility

- **NFR-A1** WCAG 2.2 AA targets: contrast, scalable text, screen-reader labels, focus order, touch targets of at least 44 px.
- **NFR-A2** The chat is fully usable with a screen reader; streaming text announces completed messages rather than every token.

### 4.6 Maintainability

- **NFR-M1** Layered backend (handler, service, repository) with the LLM and dictionary behind interfaces so providers can be swapped. \[P\]
- **NFR-M2** Prompts are versioned artifacts stored in source control; each session records the prompt version used.
- **NFR-M3** Schema changes via migrations; structured logging and request tracing.
- **NFR-M4** Minimum 80% unit test coverage on service-layer logic. \[A\]

## 5. User Stories and Primary Flows

### 5.1 User Stories

- US1: As a learner, I want to type a word quickly so I can start learning before I forget it.
- US2: As a learner, I want the AI to ask me questions so I understand the word instead of just reading a definition.
- US3: As a learner, I want a direct explanation when I am stuck so I am not frustrated by endless questions.
- US4: As a learner, I want to write my own sentence and get honest feedback.
- US5: As a learner, I want to decide when to commit a word.
- US6: As a learner, I want my personal definition saved next to the official one so I can see how my understanding compares.
- US7: As a learner, I want to search my library and reread how I learned a word.
- US8: As a learner, I want to resume an interrupted session without losing progress.

### 5.2 Primary Flow: Capture to Commit

1. Learner opens app and enters a word.
2. System normalizes it, checks for duplicates, creates the session, retrieves definition data.
3. AI opens the conversation; learner and AI exchange turns.
4. Learner taps Commit.
5. System drafts record; AI flags any inaccuracy in the personal definition.
6. Learner reviews and edits, then confirms.
7. System saves atomically and shows the word in the library.

### 5.3 Primary Flow: Retrieve

1. Learner opens library, searches or filters.
2. Learner opens a word and sees definitions side by side, examples, notes and conversation.

## 6. AI Conversation Behavior

### 6.1 Instructional Policy \[P\]

The model receives a system prompt defining: role (vocabulary tutor), Socratic default, ground-truth definition data supplied by the server, the behaviors listed below, and constraints.

- **AI-001** Use conversation history to infer what the user already knows and choose the next move (probe, explain, example, sentence task, contrast with a similar word, correct).
- **AI-002** Never declare understanding based only on agreement; require evidence such as paraphrase, appropriate-context identification, or an original sentence. \[C\]
- **AI-003** Correct errors constructively with reasoning. \[C\]
- **AI-004** Distinguish senses when context requires and ask which sense the user encountered. \[C\]
- **AI-005** Address register, connotation and limitations where relevant. \[C\]
- **AI-006** Keep turns concise; default maximum of about 120 words per turn and one question per turn. \[A\]
- **AI-007** Stay on the target word; politely redirect off-topic requests.
- **AI-008** Factual claims about meaning must be consistent with the supplied definition data; where the model adds nuance beyond it, it must not contradict it.

### 6.2 Session Lifecycle

States: `active` → `committing` → `committed`; `active` → `abandoned`; `abandoned` → `active` (resume); `active` → `failed` (unrecoverable setup error). \[P\]

- **SES-001** Only one `active` session per user per word.
- **SES-002** Sessions inactive for 30 minutes are marked `abandoned` but remain resumable for 30 days. \[A\]
- **SES-003** Abandoned sessions with fewer than two user messages are purged after 7 days. \[A\]

### 6.3 Structured Outputs

- **AI-010** Commit-time generation uses a structured schema (JSON) for: understanding assessment (`sufficient | partial | inaccurate`), discrepancy note, candidate example sentence (from user text only), contextual notes. The server validates the schema before use.
- **AI-011** The personal-definition draft is composed only from user-authored statements; the server verifies each draft sentence is grounded in a user message (or the user edits it manually). \[P\]

### 6.4 Error Handling

- **ERR-001** AI request timeout (default 30 s) or provider error: show an inline retry action; the user message remains saved. No automatic silent resend more than twice.
- **ERR-002** Malformed structured output: retry once with a repair prompt; otherwise fall back to a manual commit form where the user writes the personal definition.
- **ERR-003** Dictionary source failure: the session may proceed with a clearly labeled "definition unavailable" state; Commit is blocked until an official definition is attached or the user explicitly accepts a pending-verification flag \[Q5\].
- **ERR-004** Rate limit exceeded: show a clear message with retry time.
- **ERR-005** Content-policy refusal from the model: show a neutral message and allow the user to continue or abandon.

## 7. Data Model

### 7.1 Entities

- **User**: the account; owns all data.
- **LearningSession**: one chat about a word; ends committed, abandoned or failed.
- **Message**: an append-only chat turn within a session.
- **DefinitionSnapshot**: immutable dictionary data (source, payload, content hash), shared across users.
- **VocabEntry**: the committed library item, one per user, word and sense; holds the official and personal definitions side by side.
- **EntryRevision**: edit history after commit; revision 1 holds the original committed values.
- **CommitAssessment**: the AI's validated verdict on understanding at commit.
- **IdempotencyKey**: makes commit and other writes safe to repeat.

### 7.2 Relationships

A user has many sessions and entries. A session has many messages and at most one commit assessment, and commits to at most one entry (its origin); a later relearning session can link to an existing entry. Sessions and entries both point to an immutable definition snapshot, and an entry has many revisions. Idempotency keys belong to a user and are omitted from the diagram.

&#91;embedded content: data model · 7 tables\]

Only committed sessions create library entries, so abandoned sessions never appear in the library.

### 7.3 Persistence Requirements

- Messages append-only; ordered by sequence number.
- Definition snapshots are immutable once attached to a committed record so the record remains stable if the source changes.
- Deletion cascades from user to all owned data.

### 7.4 Proposed Schema (PostgreSQL) \[P\]

Sessions and entries are separate tables: an entry is unique per user, word and sense, which handles duplicates and multiple meanings. Dictionary payloads are snapshotted so a later dictionary change never rewrites what the learner studied.

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS citext;

-- Identity
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email         CITEXT NOT NULL UNIQUE,
  password_hash TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMPTZ
);

-- Shared, immutable dictionary data
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

-- The chat events
CREATE TABLE learning_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  entry_id        UUID,
  word_text       TEXT NOT NULL,
  headword_key    TEXT NOT NULL,
  status          TEXT NOT NULL CHECK (status IN
                  ('active','committing','committed','abandoned','failed')),
  context_note    TEXT,
  snapshot_id     UUID REFERENCES definition_snapshots(id),
  prompt_version  TEXT NOT NULL,
  model_id        TEXT,
  started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_active_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at        TIMESTAMPTZ
);
CREATE UNIQUE INDEX one_active_session_per_word
  ON learning_sessions (user_id, headword_key) WHERE status = 'active';
CREATE INDEX sessions_user_recent
  ON learning_sessions (user_id, last_active_at DESC);

CREATE TABLE messages (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id  UUID NOT NULL REFERENCES learning_sessions(id) ON DELETE CASCADE,
  seq         INT  NOT NULL,
  role        TEXT NOT NULL CHECK (role IN ('user','assistant')),
  content     TEXT NOT NULL,
  token_count INT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (session_id, seq)
);

-- The library
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
  personal_def_origin TEXT NOT NULL CHECK (personal_def_origin IN
                      ('user_written','user_approved_ai_edit')),
  example_sentence    TEXT,
  contextual_notes    TEXT,
  understanding_flag  TEXT NOT NULL DEFAULT 'ok'
                      CHECK (understanding_flag IN ('ok','needs_review')),
  discrepancy_note    TEXT,
  origin_session_id   UUID NOT NULL REFERENCES learning_sessions(id),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  search_vector       TSVECTOR GENERATED ALWAYS AS (
      setweight(to_tsvector('english', word_text), 'A') ||
      setweight(to_tsvector('english', personal_definition), 'B') ||
      setweight(to_tsvector('english', coalesce(contextual_notes,'')), 'C')
  ) STORED,
  UNIQUE (user_id, headword_key, sense_key)
);
CREATE INDEX entries_user_created ON vocab_entries (user_id, created_at DESC);
CREATE INDEX entries_search ON vocab_entries USING GIN (search_vector);
CREATE INDEX entries_word_trgm ON vocab_entries USING GIN (word_text gin_trgm_ops);

ALTER TABLE learning_sessions
  ADD CONSTRAINT sessions_entry_fk
  FOREIGN KEY (entry_id) REFERENCES vocab_entries(id) ON DELETE SET NULL;

-- Post-commit edit history (rev 1 = original committed values)
CREATE TABLE entry_revisions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entry_id   UUID NOT NULL REFERENCES vocab_entries(id) ON DELETE CASCADE,
  rev_no     INT  NOT NULL,
  changes    JSONB NOT NULL,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (entry_id, rev_no)
);

-- Commit-time AI assessment, kept for learning-outcome evaluation
CREATE TABLE commit_assessments (
  session_id       UUID PRIMARY KEY REFERENCES learning_sessions(id) ON DELETE CASCADE,
  verdict          TEXT NOT NULL CHECK (verdict IN ('sufficient','partial','inaccurate')),
  discrepancy_note TEXT,
  raw_output       JSONB NOT NULL,
  prompt_version   TEXT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotent commits and other mutating calls
CREATE TABLE idempotency_keys (
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key          TEXT NOT NULL,
  request_hash BYTEA NOT NULL,
  response     JSONB,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, key)
);
```

## 8. API and Service Boundaries \[P\]

### 8.1 Services

- **Auth service** — identity and tokens.
- **Session service** — session lifecycle, message persistence, orchestration.
- **Conversation engine** — builds prompts, calls the LLM provider, validates structured outputs, enforces policy checks. Isolated behind an interface.
- **Definition service** — fetches, normalizes and caches dictionary data; produces DefinitionSnapshots.
- **Library service** — commit, read, search, edit, delete.

### 8.2 Endpoints (REST, JSON)

| Method | Path | Purpose |
| --- | --- | --- |
| POST | /v1/sessions | Start session `{word, context?}` |
| GET | /v1/sessions/{id} | Session with messages |
| POST | /v1/sessions/{id}/messages | Send user message; streams assistant reply (SSE) |
| POST | /v1/sessions/{id}/resume | Resume abandoned session |
| POST | /v1/sessions/{id}/commit-draft | Generate draft record |
| POST | /v1/sessions/{id}/commit | Confirm and save (idempotency key header) |
| POST | /v1/sessions/{id}/abandon | Abandon |
| GET | /v1/words | List/search/filter library |
| GET | /v1/words/{id} | Detail |
| PATCH | /v1/words/{id} | Edit personal fields |
| DELETE | /v1/words/{id} | Delete |
| GET | /v1/definitions?word= | Preview definition data |
| GET | /v1/export | Export library |

- The client never talks to the LLM or dictionary providers directly.
- All endpoints require auth except sign-in routes; all queries are scoped by user id.

## 9. Definition Source Requirements

- **DEF-001 (M)** Official definitions come from a designated dictionary source, not from unconstrained model output. \[C\]
- **DEF-002 (M)** Each definition stores its source name, retrieval time and source reference. \[P\]
- **DEF-003 (M)** The source must provide multiple senses with part of speech where available; the record preserves relevant senses. \[C\]
- **DEF-004 (M)** The model receives the retrieved definition data as grounding and is instructed not to contradict it.
- **DEF-005 (S)** If the model believes the retrieved data is wrong or incomplete, it reports a mismatch to the server for logging rather than overriding it silently.
- **DEF-006 (M)** Words not found: the user is offered spelling suggestions, may proceed with an "unverified" definition state, and the AI must say it cannot confirm the definition. Commit follows ERR-003 rules.
- **DEF-007 (S)** Definition data is cached by headword with a refresh policy; committed records use immutable snapshots.
- **DEF-008 (M)** Licensing and terms of the chosen dictionary API are verified for storage and display of definitions before adoption. \[Q1\]

## 10. Edge Cases

| Case | Required behavior |
| --- | --- |
| Incomplete conversation (user commits early) | Allow commit. Ask the user to supply a personal definition; if none, block commit of an empty personal definition. Flag `needs_review` if understanding is not demonstrated. |
| Incorrect personal definition | AI explains the discrepancy and offers a retry before commit; if the user commits anyway, store as-is with `needs_review` and discrepancy note; never rewrite the user's text silently. |
| Multiple meanings | Ask which sense applies; record `sense_label`. A word may have more than one record, one per sense. \[A\] |
| Failed AI request | See ERR-001 to ERR-005; user input never lost. |
| Duplicate word | If already committed: offer to open the existing entry, start a new sense session, or review/refresh. Do not silently create a duplicate. If an active session exists, resume it. |
| Interrupted session | Persist after every message; on return, the session resumes at the last message with a short recap. |
| Offline during session | Message send is blocked with a clear notice; drafts of unsent text are kept locally. \[A\] |
| Misspelled word | Suggest corrections before creating a session. |
| Very long or abusive input | Reject or truncate per limits; no storage of rejected content beyond logs. |
| Concurrent edits (two devices) | Last-write-wins with revision history on library edits; commit uses idempotency. |
| Commit pressed twice | Idempotent; second call returns the existing record. |

## 11. Testing Requirements

### 11.1 Conventional Testing

- Unit tests for services, validation, session state machine and normalization.
- Integration tests for each endpoint including authorization (cross-user access must fail).
- Failure-injection tests for provider timeouts, malformed AI output, and mid-commit failures.
- Accessibility tests: screen reader walkthroughs of capture, chat, commit, library.
- Performance tests against NFR-P targets.

### 11.2 AI Behavior Verification

- **TST-AI-1** A versioned evaluation set of scripted learner personas (confident-correct, confident-wrong, vague, impatient, off-topic, adversarial) run against each prompt version.
- **TST-AI-2** Automated checks: one question per turn, length limit, no repeated questions, explanation provided after repeated failures, no contradiction of supplied definition.
- **TST-AI-3** Rubric-based review (LLM-as-judge plus periodic human review) for: Socratic quality, constructive correction, avoidance of false confirmation.
- **TST-AI-4** Regression gate: a new prompt or model version must meet or beat thresholds on the evaluation set before release.
- **TST-AI-5** Grounding test: personal-definition drafts contain no content absent from user messages.
- **TST-AI-6** Prompt-injection test cases (user attempts to override the tutor role or alter stored records).

### 11.3 Learning-Outcome Evaluation

Measures aligned with the success criteria in the product description: \[C\]

- Accuracy of personal definitions versus official definitions (rubric scored).
- Quality of user-written sentences at commit.
- Delayed recall: user explains or uses a word correctly after a delay (for example 7 and 30 days) in a lightweight probe study.
- Proportion of committed words flagged `needs_review`.
- Session efficiency: turns per committed word.

Thresholds for these measures are open. \[Q6\]

## 12. Dependencies, Assumptions, Decisions and Risks

### 12.1 Dependencies

LLM provider API, dictionary data provider, PostgreSQL, authentication mechanism, push/email provider if notifications are added later.

### 12.2 Unresolved Decisions

- **Q1** Which dictionary source, and do its terms allow storing and displaying definitions?
- **Q2** Which LLM provider and model; budget per session and per user?
- **Q3** Are phrases and idioms in scope, or single words only?
- **Q4** Can a committed word be relearned with a new session, and how is that shown in the library?
- **Q5** Should commit be blocked when no official definition is available?
- **Q6** Targets for learning-outcome metrics.
- **Q7** Authentication method (email/password, OAuth, or both).
- **Q8** Platforms in v1: mobile only, web, or both?
- **Q9** Pricing or usage limits per user.
- **Q10** Exact definition of "sufficient understanding" used for the commit assessment.

### 12.3 Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| AI affirms wrong understanding | Misconceptions stored as knowledge | Grounded definitions, AI-002, discrepancy check, `needs_review` flag, evaluation set |
| Model drifts into long lectures or repetitive questions | Poor UX | Turn limits, repetition checks, regression gate |
| Dictionary licensing or availability | Blocks launch or degrades accuracy | Resolve Q1 early; cache; fallback source |
| LLM cost growth | Unsustainable economics | Token budgets, concise turns, caching definitions, usage limits |
| Personal definition drifts into AI paraphrase | Violates core principle | Grounding verification (AI-011), user review step |
| Provider outage | Core loop unavailable | Retry, clear errors, provider abstraction |
| Privacy concerns over conversation data | Trust loss | Minimal data to provider, clear notice, deletion |
| Scope creep into gamification and social features | Dilutes core loop | Explicit non-goals, review gate for scope changes |

### 12.4 Future Scope (not v1)

Recall and usage practice, spaced review, and retention measurement, designed to assess actual learning rather than save counts. \[C\]
