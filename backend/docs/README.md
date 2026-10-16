# Wordivate Backend Documentation & Learning Hub

Welcome to the internal engineering documentation and learning hub for the Wordivate backend. 

In accordance with our **AI-Assisted Development Operating Rules** and **Mentorship Mandate**, all implementations in this repository are accompanied by deep, subject-matter documentation. These documents explain not only *what* code was written, but the *architectural rationale*, *Go mechanics*, and *systems principles* behind it.

---

## Subject-Matter Directory Index

| Subject Area | Directory Path | Scope & Key Concepts Covered |
| :--- | :--- | :--- |
| **Database & Schema** | [`database/`](./database/) — [Schema & SQLC Deep-Dive](./database/schema_and_sqlc.md) | PostgreSQL schema DDL, SQLC type generation, composite indexing, transactions (`sql.Tx`), and connection pooling. |
| **Authentication & Security** | [`auth/`](./auth/) — [Authentication & Multi-Tenancy](./auth/authentication_and_multitenancy.md) | Multi-tenancy, bcrypt password hashing, JWT claims, context propagation, and authorization middleware. |
| **Lexical Grounding & Capture** | [`lexical/`](./lexical/) | Word normalization, duplicate detection, third-party dictionary clients, SHA-256 payload hashing, and immutable snapshots. |
| **Socratic Engine & Streaming** | [`socratic/`](./socratic/) | Adaptive tutor prompts, session state machines, Server-Sent Events (SSE), concurrency, channels, and error recovery. |
| **Commit & Vocabulary Library** | [`commit_library/`](./commit_library/) | Atomic transactional commits, idempotency guarantees, full-text search (`tsvector`), and revision history. |

---

## Standard Structure for Documentation Files
Every document created in this directory must include:
1. **Concept & Systems 'Why':** The fundamental architectural or engineering problem being solved.
2. **Go Implementation Deep-Dive:** Core Go language mechanics (pointers, interfaces, concurrency, context, errors).
3. **Architecture & Flow:** Visual diagrams (Mermaid) or data lifecycle representations.
4. **Verification & Testing:** Concrete CLI commands (`curl`, test commands, SQL verification) to test and validate behavior.
