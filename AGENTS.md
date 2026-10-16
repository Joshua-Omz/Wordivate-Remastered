# Wordivate Engineering Directives & Agent Rules

Welcome to the **Wordivate-Remastered** repository. Wordivate is an AI-guided vocabulary acquisition system designed to bridge the gap between dictionary lookup and active, practical language use.

All AI coding assistants, subagents, and human contributors working on this codebase must adhere strictly to the architecture, domain integrity, pedagogical policies, and operating protocol outlined below and in the `.agents/rules/` directory.

---

## 1. Prime Directive: Learning & Mentorship First
**The user's top priority is deep learning.**
- **Foundations Before Syntax:** Always explain *why* a construct or architectural pattern exists before writing code.
- **Analogies:** Use real-world analogies (*"think of it like..."*) to bridge abstract systems engineering with practical intuition.
- **Drawbacks & Trade-offs:** Always follow implementations with an explicit trade-offs analysis covering performance, scalability, and separation of concerns.
- **In-Repository Living Documentation:** Every completed backend milestone must produce or update modular documentation in `backend/docs/<subject-matter>/`, serving as an enduring learning reference.

---

## 2. Core Mission & Philosophy
Wordivate transforms passive definition lookup into an interactive, Socratic learning experience. The core user loop consists of five non-negotiable stages:
1. **Capture:** Low-friction recording of an unfamiliar word (<= 2 interactions).
2. **Clarify:** Context-aware, Socratic dialogue to explore nuances, connotations, and scenarios.
3. **Personalize:** Synthesis of the learner's own understanding into a personal definition.
4. **Commit:** Atomic, user-controlled persistence into the personal library.
5. **Recall & Use:** Practical retrieval and long-term retention reinforcement.

**Core Axiom:** Genuine comprehension and practical retention always take precedence over the mere accumulation of saved words.

---

## 3. Cardinal Architectural & Domain Rules

1. **Strict Separation of Official and Personal Definitions:**
   - `official_definition` is grounded in an immutable dictionary snapshot (`definition_snapshots`).
   - `personal_definition` MUST be authored by or approved by the user, rooted in their own conversation statements. It must **never** be a synthetic LLM paraphrase masquerading as user understanding.
   - The two definitions are stored in distinct database columns and displayed side by side. They are **never merged**.

2. **User Autonomy with Honest Feedback:**
   - The user decides when to end the session and tap **Commit**.
   - If the user's demonstrated understanding remains inaccurate, the AI must explain the discrepancy and offer clarification. If the user commits anyway, the entry is stored as-is, flagged with `understanding_flag = 'needs_review'`, and attached with a `discrepancy_note`. Misconceptions are never silently affirmed or overwritten.

3. **Backend Service & Provider Isolation:**
   - Go backend (`backend/internal/`) enforces a strict 4-tier Clean Architecture: Handlers (`handler.go`) -> Domain Services (`service.go`) -> Repositories (`repository.go`) -> Data Access (`sqlc`).
   - **Absolute Rule:** Services must NEVER inject `*database.DB`, `*sql.DB`, or call queries directly; they must interact with persistence exclusively via injected Repository interfaces.
   - LLM providers and Dictionary APIs must sit behind abstract Go interfaces (`LLMClient`, `DictionaryClient`).
   - The Flutter client **never** calls third-party AI or dictionary APIs directly. All traffic routes through the Wordivate API.

4. **Socratic Instructional Policy:**
   - Default to guiding questions and contextual prompts.
   - **No False Confirmation:** Agreement alone ("yes", "I understand") is not evidence of comprehension. Require an original example, context identification, or paraphrase.
   - **Two-Strike Explanation Fallback:** If the user fails two consecutive probes or explicitly asks ("just tell me"), immediately provide a direct explanation instead of endless questions.
   - **Brevity:** Maximum ~120 words per AI turn; maximum one question per turn.
   - **No Repetition:** Never repeat an identical question within the same session.

5. **Atomic & Idempotent Persistence:**
   - Committing a session into a vocabulary entry (`vocab_entries`) must be an atomic database transaction.
   - All mutating requests (especially commits) must support idempotency keys.
   - User data is strictly isolated; all database queries must filter on `user_id`.

6. **Strict Non-Goals (Scope Containment):**
   - **No gamification:** No points, badges, coins, or daily streaks.
   - **No social features:** No leaderboards, public decks, or shared libraries.
   - Focus exclusively on the core capture-to-retention loop.

---

## 4. Customization Directory Index

Detailed enforcement specifications are organized within `.agents/rules/`:
- [00_operating_protocol.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/rules/00_operating_protocol.md): Task categorization, single-task lock, session discipline, and response templates.
- [01_architectural_standards.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/rules/01_architectural_standards.md): Backend Go clean architecture, Flutter client structure, PostgreSQL schema conventions.
- [02_domain_rules_and_data_integrity.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/rules/02_domain_rules_and_data_integrity.md): Session lifecycle, snapshot immutability, definition provenance, and transactional integrity.
- [03_ai_socratic_policy.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/rules/03_ai_socratic_policy.md): Conversational constraints, structured outputs, prompt versioning, and evaluation rubrics.
- [04_code_quality_and_testing.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/rules/04_code_quality_and_testing.md): Go idioms, Flutter/Riverpod/go_router standards, mandatory commenting standard, and >=80% service test coverage.
- [05_scope_and_non_goals.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/rules/05_scope_and_non_goals.md): Explicit boundaries, forbidden scope creep, and deferred v2 features.
- [wordivate-srs-compliance/SKILL.md](file:///c:/Users/joshu/OneDrive/Documents/300level folder/other stuff/projects/wordivate/Wordivate-Remastered/.agents/skills/wordivate-srs-compliance/SKILL.md): Automated verification checklist for code and behavioral compliance.
