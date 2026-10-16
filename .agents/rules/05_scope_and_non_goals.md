# Scope Containment & Explicit Non-Goals

## 1. Product Philosophy: Depth Over Clutter

Wordivate is intentionally designed with minimal cognitive load. The UI and feature set must remain ruthlessly focused on:
1. The captured word.
2. The learning conversation.
3. The resulting personal knowledge.

Unnecessary dashboards, gamification gimmicks, and premature social features dilute the learner's focus and corrupt genuine comprehension into artificial score-chasing.

---

## 2. Explicit Non-Goals for Version 1.0 (SRS Section 1.4)

Contributors and subagents must strictly reject or defer feature requests involving:

| Forbidden Feature Area | Rationale & Guidance |
| :--- | :--- |
| **Gamification** | No experience points (XP), leveling, coins, badges, or daily streak counters. Learning is motivated by utility, not synthetic dopamine loops. |
| **Social & Community** | No public leaderboards, user following, shared decks, or public activity feeds. Vocabulary libraries are private to the individual learner. |
| **Multi-Language Learning** | English is the only supported target word and conversation language in v1. Multi-language support is deferred. |
| **Voice / Speech Audio** | No text-to-speech (TTS) synthesis or voice audio input in v1. |
| **Complex Spaced Repetition Scheduling** | SuperMemo / SM-2 spaced repetition algorithms are deferred to post-v1 retention features. Do not over-engineer complex review schedulers in v1. |
| **Unrelated Productivity Tools** | No general note-taking suites, flashcard marketplace, or markdown document editors. |

---

## 3. Scope Gate for Agents

Whenever an agent or developer considers adding a feature:
1. **Check Core Loop:** Does this feature directly serve *Capture*, *Clarify*, *Personalize*, *Commit*, or *Retrieve*? If not, reject it.
2. **Flag Assumptions:** If an ambiguous requirement emerges, label it with `[A]` (Assumption) or `[Q]` (Question) rather than inventing unconfirmed functionality.
3. **Protect Simplicity:** Always prefer the cleanest, most understandable implementation over premature complexity.
