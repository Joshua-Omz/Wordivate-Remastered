# Domain Rules & Data Integrity

## 1. The Core 5-Stage Vocabulary Lifecycle

Every vocabulary item in Wordivate transitions through five distinct phases:

```
[1. Capture]     -> Fast, frictionless capture of unfamiliar word
      |
[2. Clarify]     -> Socratic, context-aware AI learning session
      |
[3. Personalize] -> User expresses understanding in their own words
      |
[4. Commit]      -> Transactional persistence to user's library
      |
[5. Recall/Use]  -> Contextual review, retrieval practice, and usage reinforcement
```

---

## 2. Inviolable Rule: Separation of Official vs. Personal Definitions

### 2.1 Lexical Grounding (`official_definition`)
- Official definitions must be anchored in authoritative lexical sources recorded in the `definition_snapshots` table.
- Each snapshot captures:
  - `headword_key`: normalized lowercase word.
  - `source`: identifier of the dictionary source.
  - `payload`: raw, structured lexical data (parts of speech, senses, etymology).
  - `content_hash`: SHA-256 hash ensuring payload immutability.
- Once created, a `definition_snapshot` is **immutable**. Subsequent updates to third-party dictionaries will never alter historical snapshots referenced by existing vocabulary entries.

### 2.2 User Provenance (`personal_definition`)
- The personal definition represents how the learner conceptualizes the word.
- **Provenance Requirement (FR-022, AI-011):** The personal definition MUST be authored by the user or synthesized directly from statements made by the user in the learning conversation.
- **No Hallucinated Paraphrases:** The system must NEVER invent an arbitrary AI-generated paraphrase and commit it as the user's personal understanding.
- If the user commits without having articulated an explanation, the application must prompt the user to compose one before finalizing.

### 2.3 Strict Separation (FR-023)
- `vocab_entries.official_definition` and `vocab_entries.personal_definition` must be stored in independent database columns.
- The UI must display them side by side. They must **never** be merged or flattened into a single text block.

---

## 3. Session State Machine & Concurrency

### 3.1 State Transitions (SES-001 - SES-003)
A learning session (`learning_sessions`) progresses through the following states:

```
        +-------------------------+
        |         active          |<--------------------+
        +---+---------+-------+---+                     |
            |         |       |                         |
(Commit tap)|         |       | (Unrecoverable error)   | (Resume)
            v         |       v                         |
      +------------+  |  +----------+                   |
      | committing |  |  |  failed  |                   |
      +-----+------+  |  +----------+                   |
            |         |                                 |
            v         | (30 min inactivity)             |
      +-----------+   v                                 |
      | committed | +-----------+                       |
      +-----------+ | abandoned +-----------------------+
                    +-----------+
```

1. **`active`:** Ongoing learning conversation. Maximum of **one active session per user per word**.
2. **`committing`:** Commit draft generated; undergoing user confirmation.
3. **`committed`:** Library entry successfully written; session finalized.
4. **`abandoned`:** No activity for > 30 minutes. Resumable within 30 days. Purged after 7 days if < 2 user messages.
5. **`failed`:** Unrecoverable setup or provider failure.

### 3.2 Duplicate Prevention & Normalization (FR-003, FR-004)
- Headwords must be normalized: trim leading/trailing whitespace, fold case (`"  Ephemeral "` -> `"ephemeral"`).
- If a word is captured that already exists in the user's library:
  - The UI must inform the user and offer options: (1) View existing entry, (2) Study a distinct sense/meaning, or (3) Relearn.
  - The system must **never** silently overwrite or duplicate existing library records without user confirmation.

---

## 4. Transactional Commit & Idempotency

### 4.1 Atomic Commit (FR-026)
A commit operation MUST execute inside a single PostgreSQL database transaction:
1. Transition session state from `active` / `committing` -> `committed`.
2. Insert or update the `vocab_entries` record.
3. Record the initial audit revision in `entry_revisions` (`rev_no = 1`).
4. Record the AI assessment in `commit_assessments`.

If any step fails, the entire transaction rolls back cleanly. Partial commits are strictly forbidden.

### 4.2 Idempotency Guarantee (FR-027)
- Clients must include a unique `Idempotency-Key` header with commit requests.
- The backend records keys in `idempotency_keys` (`PRIMARY KEY (user_id, key)`).
- Repeated identical commit requests must return the previously generated response without re-executing writes or generating duplicate entries.

---

## 5. Misconception Handling & Autonomy (FR-024)

- If a user's articulated understanding conflicts with the authoritative dictionary definition, the AI must explicitly point out the discrepancy during clarification and commit drafting.
- **Learner Autonomy Preserved:** The user retains the final right to commit.
- If the user commits with an uncorrected inaccuracy:
  - The record is persisted with `vocab_entries.understanding_flag = 'needs_review'`.
  - The identified discrepancy is persisted in `vocab_entries.discrepancy_note`.
  - The user's words are saved intact; the system must never censor or rewrite the user's text silently.
