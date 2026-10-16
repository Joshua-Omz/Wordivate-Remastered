---
name: wordivate-srs-compliance
description: >-
  Audits, verifies, and guides development in Wordivate-Remastered against the
  Software Requirements Specification (SRS) and product README. Use whenever implementing
  or refactoring features, writing database migrations, designing API endpoints,
  or testing AI conversation policies.
---

# Wordivate SRS Compliance & Verification Skill

Use this skill to ensure all code, architecture, and behavior align with the core Wordivate requirements.

## Step-by-Step Verification Checklist

### 1. Domain & Data Verification
- [ ] Are `official_definition` and `personal_definition` stored in separate fields and never merged?
- [ ] Does the `personal_definition` originate from user text, without synthetic AI hallucinations?
- [ ] Is `definition_snapshots` populated with an immutable lexical payload and content hash?
- [ ] Is the commit operation wrapped in an atomic database transaction?
- [ ] Are mutating endpoints protected by idempotency checks (`Idempotency-Key` header)?
- [ ] Are all database operations strictly scoped to the authenticated `user_id`?

### 2. Socratic AI Policy Verification
- [ ] Does the conversational prompt default to Socratic questioning?
- [ ] Is there an explicit rule against false confirmation (not accepting "yes" as proof of understanding)?
- [ ] Does the two-strike rule provide a direct explanation if the user is stuck or says "just tell me"?
- [ ] Is each AI turn bounded by ~120 words and limited to a single question?
- [ ] Are questions prevented from repeating within a single session?
- [ ] Is the prompt template versioned and tracked in `learning_sessions.prompt_version`?

### 3. Architecture & Code Quality Verification
- [ ] Does the backend maintain clean layering (Handlers -> Services -> Repositories)?
- [ ] Are LLM and dictionary providers hidden behind Go interfaces?
- [ ] Does the client avoid direct calls to third-party AI or dictionary APIs?
- [ ] Are non-goals respected (no gamification, streaks, points, or social feeds)?
- [ ] Does service-layer code maintain >= 80% unit test coverage?
- [ ] Do interactive Flutter elements maintain at least 44x44px touch targets?
