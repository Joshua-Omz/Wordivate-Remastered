# AI-Assisted Development: Operating Rules & Session Protocol

This document defines the operational protocol for all agent sessions in the Wordivate workspace. It ensures disciplined single-task execution, transparent boundaries, and continuous educational mentorship.

---

## 1. Task Categories

Every request must be classified into exactly one category before performing work:

| Category | Definition | Expected Output |
| :--- | :--- | :--- |
| **BUG FIX** | Existing behavior is wrong or broken | Root cause, minimal fix, how to verify |
| **PATCH** | Small targeted change to existing code (tweak, refactor, config) | The diff only, plus reason |
| **ADD-ON** | New feature or capability | Short plan, implementation, integration points |
| **SIMPLE TASK** | Small isolated job (a snippet, a rename, an explanation) | Direct answer, no ceremony |

*If a request spans multiple categories, split it into separate sequential tasks and handle one at a time.*

---

## 2. Interpreting User Intent

Before taking action, the agent must explicitly state its understanding in this exact format:

- **Category:** `[BUG FIX | PATCH | ADD-ON | SIMPLE TASK]`
- **Goal:** One sentence describing the outcome the user wants.
- **Scope:** The files, modules, or behaviors to be modified.
- **Out of scope:** What is deliberately left untouched.

### Intent Rules:
1. If the intent is clear, proceed without asking.
2. If ambiguous, ask **ONE** clarifying question and wait.
3. Do not guess at missing requirements; explicitly name any necessary assumptions.
4. If the user's wording suggests one category but the work is another (e.g., a "quick fix" that is actually a new architectural feature), state the re-classification before continuing.

---

## 3. Session Discipline (Single-Task Lock)

- **One Task per Session:** The task defined in Section 2 is the sole active objective until marked `COMPLETED`.
- **No Side Objectives:** If the user raises a different bug, idea, or request mid-session:
  1. Do not act on it.
  2. Add it to a **Parking Lot** list (one line per item).
  3. Reply: *"Noted in the Parking Lot. Finishing the current task first."*
- **No Scope Creep from the Agent:** Do not refactor, rename, or "clean up" anything outside the declared scope. Note potential improvements in the Parking Lot instead.
- **Completion Gate:** A task is `COMPLETED` only when:
  - The stated goal is met.
  - The result has been verified or concrete verification steps are provided.
  - The user has confirmed the outcome.
- **Completion Output:** Output `SESSION COMPLETED: <task title>` followed by the current Parking Lot items. Only then may a new task begin.
- **Task Switching:** If the user insists on switching before completion, offer two choices:
  - **(a)** Finish the current task, or
  - **(b)** Abandon it explicitly (mark it `ABANDONED` with its current state), then proceed.

### 3.1 Mandatory Post-Implementation Documentation Gate
A backend implementation task (whether BUG FIX, PATCH, or ADD-ON) is **NOT complete** until corresponding subject-matter documentation is created or updated in `backend/docs/<subject-matter>/`:

- **Directory Hierarchy:** Subject-matter documentation must be nested inside dedicated subject directories to ensure clean navigation:
  - `backend/docs/database/` — Schema design, SQLC queries, indexing, migrations, and concurrency.
  - `backend/docs/auth/` — Identity, password hashing, JWT claims, middleware, and multi-tenancy.
  - `backend/docs/lexical/` — Dictionary APIs, normalization, hashing, and immutable snapshots.
  - `backend/docs/socratic/` — Session lifecycle, prompt versioning, SSE streaming, and tutor policies.
  - `backend/docs/commit_library/` — Atomic transactions, idempotency keys, and full-text search.
- **Required Content per Doc:**
  1. **Concept & Systems 'Why':** The engineering problem being solved.
  2. **Go Implementation Deep-Dive:** Pointers, interfaces, concurrency, or standard library features applied.
  3. **Architecture & Flow:** Mermaid diagrams or request-response lifecycle.
  4. **Verification & Testing:** Concrete instructions and curl/CLI commands to test the component.

---

## 4. Response Format by Category

Every response must adhere to the template corresponding to its category:
- **BUG FIX:** Symptom, Root cause, Fix (diff), How to verify.
- **PATCH:** What changed and why, Diff.
- **ADD-ON:** Plan (brief), Implementation, Integration notes, How to verify.
- **SIMPLE TASK:** The answer, kept short and direct.

### Mandatory Status Indicator
Every response must conclude with:
`STATUS: IN PROGRESS | AWAITING CONFIRMATION | COMPLETED`

---

## 5. Educational Mentorship Mandate

**Learning is top priority.** In accordance with the project's engineering mentorship principles:
- **Foundations Before Syntax:** Always explain *why* an architectural or language pattern exists before presenting code.
- **Analogies:** Use *"think of it like..."* with tangible real-world comparisons to demystify complex systems.
- **Trade-offs:** Explicitly state the limitations, performance costs, and scalability impacts of chosen implementations in a dedicated **Drawbacks & Trade-offs** section.
