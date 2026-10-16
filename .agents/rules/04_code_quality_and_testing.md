# Code Quality, Testing, & Commenting Standards

## 1. Backend Coding Standards (Go)

### 1.1 Idiomatic Error Handling
- Never ignore errors: always check `if err != nil`.
- Wrap errors with diagnostic context using `%w`:
  ```go
  if err := r.db.Exec(ctx, query, args...); err != nil {
      return fmt.Errorf("committing vocab entry for user %s: %w", userID, err)
  }
  ```
- **No Panics:** Handlers and services must NEVER call `panic()`. Always return typed or wrapped errors to upper layers. The Chi `Recoverer` middleware exists solely as an ultimate safety net for catastrophic faults.

### 1.2 Context Propagation & Lifecycle
- Every blocking or I/O function (database calls, HTTP requests, LLM streaming) MUST accept `ctx context.Context` as its first parameter.
- Honor cancellation signals (`ctx.Done()`) to prevent orphaned queries and goroutine leaks.

### 1.3 Concurrency & Channels
- Always protect shared in-memory state with `sync.RWMutex` or encapsulate within actor-like goroutine channels.
- Ensure every started goroutine has a clearly defined termination path (e.g., listening on `ctx.Done()` or explicit close channels).

---

## 2. Client Coding Standards (Flutter / Dart)

### 2.1 Language, Linting, & Formatting
- **Null Safety:** Strict null-safe Dart conforming to `flutter_lints`.
- **Formatting:** Standard `dart format` style with 80-column line wrapping and trailing commas for readable diffs.
- **Dependencies:** Do not add a new package to `pubspec.yaml` without stating why and obtaining explicit approval.

### 2.2 Architecture & State Management
- **Feature-First Organization:** Every feature lives in `client/lib/features/<feature>/` separated into `data/`, `domain/`, and `presentation/`. No business logic inside widgets.
- **Riverpod for State:** Use Riverpod for all application state. `setState` is forbidden outside purely local, ephemeral widget state (e.g., text controller focus, animation ticks).
- **Navigation:** Use `go_router` exclusively; routes must be declared in one centralized router location.
- **Immutability:** State models must be immutable, supporting functional updates via `copyWith`.
- **Naming Conventions:**
  - Types: `UpperCamelCase`
  - Members & variables: `lowerCamelCase`
  - Files: `snake_case.dart`
  - Riverpod providers: Must end with `Provider` (e.g., `socraticSessionProvider`).
- **Error Handling:** Never swallow exceptions. Surface failures explicitly through typed domain results or `AsyncValue.error`.

### 2.3 Widget Hygiene & Accessibility (WCAG 2.2 AA)
- Prefer small, `const`-constructible widgets. Extract private widget classes instead of writing helper methods that return widgets (helper methods break element subtree caching).
- **Minimum Touch Targets:** Interactive controls must measure at least **44 x 44 logical pixels**.
- **Screen Reader Announcements:** When streaming AI responses, do not announce every token. Announce the full completed message once streaming ends.

---

## 3. Mandatory Commenting Standard

Every file and function must follow this standard. Comments exist to make code review faster and explain *intent*:

1. **File Header Comment:** Every file must start with a top-level comment explaining its architectural role:
   ```dart
   // Purpose: Manages Socratic chat state and SSE message streaming.
   // Architecture: client/lib/features/socratic_session/presentation/providers
   ```
2. **Doc Comments (`///`):** Every class, public method, and function must have a doc comment detailing:
   - What it does.
   - Its parameters and return values.
   - Any exceptions or errors it throws.
3. **Inline Intent Comments (`//`):** Any non-obvious logic, workarounds, or performance optimizations must have an inline comment explaining *why* it is written that way.
4. **No Trivial Comments:** Comments must explain *reasoning*, not restate what the syntax already says (e.g., avoid `// increments counter` above `counter++`).
5. **Completion Gate:** A code modification is incomplete if any new or modified symbol lacks these comments.

---

## 4. Testing Requirements (SRS Section 11)

### 4.1 Unit & Integration Testing
- **Minimum 80% Service Coverage (NFR-M4):** All domain service logic in `backend/internal/features/` must maintain at least 80% unit test coverage.
- **Multi-Tenant Isolation Tests:** Integration tests must verify that user A cannot view, modify, or delete sessions or vocabulary entries belonging to user B (HTTP 403 / 404).
- **Failure Injection Testing:** Automated tests must simulate:
  - Database failure during commit (verifying atomicity rollback).
  - LLM provider timeouts (verifying retry UI without data loss).
  - Malformed AI structured output (verifying fallback to user input).

### 4.2 AI Behavior & Regression Verification (TST-AI-1 - TST-AI-6)
Before deploying any prompt version or model upgrade, run the automated evaluation suite against standardized learner personas:
1. **Confident-Correct:** Validates that the AI confirms genuine comprehension efficiently without dragging out the conversation.
2. **Confident-Wrong:** Validates that misconceptions are actively corrected with reasoning, not falsely affirmed.
3. **Vague / Struggling:** Validates that the AI triggers the two-strike direct explanation fallback.
4. **Impatient ("Just tell me"):** Validates that the AI immediately abandons Socratic questioning and explains directly.
5. **Prompt-Injection / Adversarial:** Validates that the model treats user input strictly as data and refuses role breaks.
