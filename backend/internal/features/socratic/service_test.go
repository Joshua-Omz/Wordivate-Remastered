// Purpose: Unit test suite for Socratic domain service, state machine, and rules enforcement.
// Architecture: backend/internal/features/socratic/service_test.go
package socratic_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/socratic"
)

// mockRepository implements SocraticRepository in-memory for unit testing.
type mockRepository struct {
	mu          sync.Mutex
	sessions    map[uuid.UUID]*socratic.Session
	messages    map[uuid.UUID][]socratic.Message
	snapshots   map[uuid.UUID]*socratic.SnapshotRecord
	failCreate  bool
	failSeq     bool
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		sessions:  make(map[uuid.UUID]*socratic.Session),
		messages:  make(map[uuid.UUID][]socratic.Message),
		snapshots: make(map[uuid.UUID]*socratic.SnapshotRecord),
	}
}

func (m *mockRepository) GetSessionByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*socratic.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[id]
	if !exists || s.UserID != userID {
		return nil, socratic.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockRepository) UpdateSessionStatus(ctx context.Context, id uuid.UUID, userID uuid.UUID, status socratic.SessionStatus) (*socratic.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[id]
	if !exists || s.UserID != userID {
		return nil, socratic.ErrSessionNotFound
	}
	s.Status = status
	return s, nil
}

func (m *mockRepository) UpdateSessionLastActive(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[id]
	if !exists || s.UserID != userID {
		return socratic.ErrSessionNotFound
	}
	s.LastActiveAt = time.Now()
	return nil
}

func (m *mockRepository) CreateMessage(ctx context.Context, sessionID uuid.UUID, seq int32, role socratic.Role, content string, tokenCount *int32) (*socratic.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failCreate {
		return nil, errors.New("db insert failure")
	}

	msg := socratic.Message{
		ID:         uuid.New(),
		SessionID:  sessionID,
		Seq:        seq,
		Role:       role,
		Content:    content,
		TokenCount: tokenCount,
		CreatedAt:  time.Now(),
	}
	m.messages[sessionID] = append(m.messages[sessionID], msg)
	return &msg, nil
}

func (m *mockRepository) GetLastMessageSeq(ctx context.Context, sessionID uuid.UUID) (int32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failSeq {
		return 0, errors.New("db query seq failure")
	}

	msgs := m.messages[sessionID]
	if len(msgs) == 0 {
		return 0, nil
	}
	return msgs[len(msgs)-1].Seq, nil
}

func (m *mockRepository) ListMessagesBySession(ctx context.Context, sessionID uuid.UUID) ([]socratic.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msgs := m.messages[sessionID]
	res := make([]socratic.Message, len(msgs))
	copy(res, msgs)
	return res, nil
}

func (m *mockRepository) GetSnapshotByID(ctx context.Context, id uuid.UUID) (*socratic.SnapshotRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[id]
	if !exists {
		return nil, socratic.ErrSessionNotFound
	}
	return snap, nil
}

func (m *mockRepository) AbandonInactiveSessions(ctx context.Context, cutoff time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	for _, s := range m.sessions {
		if s.Status == socratic.SessionStatusActive && s.LastActiveAt.Before(cutoff) {
			s.Status = socratic.SessionStatusAbandoned
			count++
		}
	}
	return count, nil
}

func (m *mockRepository) GetActiveSessionByWord(ctx context.Context, userID uuid.UUID, headwordKey string) (*socratic.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sessions {
		if s.UserID == userID && s.HeadwordKey == headwordKey && s.Status == socratic.SessionStatusActive {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) AbandonSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*socratic.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[id]
	if !exists || s.UserID != userID {
		return nil, socratic.ErrSessionNotFound
	}
	if s.Status != socratic.SessionStatusActive {
		return nil, socratic.ErrSessionNotActive
	}
	s.Status = socratic.SessionStatusAbandoned
	now := time.Now()
	s.EndedAt = &now
	s.LastActiveAt = now
	return s, nil
}

func (m *mockRepository) ResumeSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*socratic.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[id]
	if !exists || s.UserID != userID {
		return nil, socratic.ErrSessionNotFound
	}
	if s.Status != socratic.SessionStatusAbandoned {
		return nil, socratic.ErrSessionNotAbandoned
	}
	s.Status = socratic.SessionStatusActive
	s.EndedAt = nil
	s.LastActiveAt = time.Now()
	return s, nil
}


// -------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------

func TestPostUserMessage_Success(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(0)
	svc := socratic.NewService(repo, llm)

	userID := uuid.New()
	sessionID := uuid.New()
	snapID := uuid.New()

	repo.snapshots[snapID] = &socratic.SnapshotRecord{
		ID:          snapID,
		HeadwordKey: "ephemeral",
		Payload:     []byte(`[{"part_of_speech":"adjective","definitions":[{"definition":"Lasting for a very short time."}]}]`),
	}

	repo.sessions[sessionID] = &socratic.Session{
		ID:            sessionID,
		UserID:        userID,
		WordText:      "ephemeral",
		HeadwordKey:   "ephemeral",
		Status:        socratic.SessionStatusActive,
		SnapshotID:    snapID,
		PromptVersion: "v1.0",
		LastActiveAt:  time.Now(),
	}

	eventChan, err := svc.PostUserMessage(context.Background(), userID, sessionID, "I saw this word used describing cherry blossoms.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var tokens []string
	var doneEvent *socratic.StreamEvent

	for ev := range eventChan {
		if ev.Type == socratic.StreamEventToken {
			tokens = append(tokens, ev.Token)
		} else if ev.Type == socratic.StreamEventDone {
			doneEvent = &ev
		}
	}

	if len(tokens) == 0 {
		t.Fatalf("expected streamed tokens, got none")
	}

	if doneEvent == nil || !doneEvent.Done {
		t.Fatalf("expected final Done event, got nil")
	}

	// Verify persistence: user message (seq 1) and assistant message (seq 2)
	msgs, err := repo.ListMessagesBySession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("error listing messages: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages persisted, got %d", len(msgs))
	}

	if msgs[0].Role != socratic.RoleUser || msgs[0].Seq != 1 {
		t.Errorf("expected msg[0] user with seq 1, got %v with seq %d", msgs[0].Role, msgs[0].Seq)
	}

	if msgs[1].Role != socratic.RoleAssistant || msgs[1].Seq != 2 {
		t.Errorf("expected msg[1] assistant with seq 2, got %v with seq %d", msgs[1].Role, msgs[1].Seq)
	}
}

func TestPostUserMessage_EmptyContent(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	_, err := svc.PostUserMessage(context.Background(), uuid.New(), uuid.New(), "   ")
	if !errors.Is(err, socratic.ErrEmptyMessage) {
		t.Errorf("expected ErrEmptyMessage, got %v", err)
	}
}

func TestPostUserMessage_ContentTooLong(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	longContent := strings.Repeat("word ", 500) // > 2000 chars
	_, err := svc.PostUserMessage(context.Background(), uuid.New(), uuid.New(), longContent)
	if !errors.Is(err, socratic.ErrMessageTooLong) {
		t.Errorf("expected ErrMessageTooLong, got %v", err)
	}
}

func TestPostUserMessage_SessionNotFound(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	_, err := svc.PostUserMessage(context.Background(), uuid.New(), uuid.New(), "hello")
	if !errors.Is(err, socratic.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestPostUserMessage_SessionNotActive(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "serendipity",
		Status:   socratic.SessionStatusCommitted,
	}

	_, err := svc.PostUserMessage(context.Background(), userID, sessionID, "can I continue?")
	if !errors.Is(err, socratic.ErrSessionNotActive) {
		t.Errorf("expected ErrSessionNotActive, got %v", err)
	}
}

func TestPostUserMessage_TwoStrikeFallback_Explicit(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(0)
	svc := socratic.NewService(repo, llm)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:          sessionID,
		UserID:      userID,
		WordText:    "ephemeral",
		Status:      socratic.SessionStatusActive,
		ContextNote: "",
	}

	eventChan, err := svc.PostUserMessage(context.Background(), userID, sessionID, "I don't know, just tell me please")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	for ev := range eventChan {
		if ev.Type == socratic.StreamEventToken {
			sb.WriteString(ev.Token)
		}
	}

	response := sb.String()
	// Should contain direct explanation language
	if !strings.Contains(strings.ToLower(response), "crystal clear") && !strings.Contains(strings.ToLower(response), "fundamentally means") {
		t.Errorf("expected direct explanation fallback response, got: %s", response)
	}
}

func TestPostUserMessage_TwoStrikeFallback_ConsecutiveStruggles(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(0)
	svc := socratic.NewService(repo, llm)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
		Status:   socratic.SessionStatusActive,
	}

	// First struggle turn
	_, _ = svc.PostUserMessage(context.Background(), userID, sessionID, "idk")
	// Second struggle turn
	eventChan, err := svc.PostUserMessage(context.Background(), userID, sessionID, "no idea")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	for ev := range eventChan {
		if ev.Type == socratic.StreamEventToken {
			sb.WriteString(ev.Token)
		}
	}

	response := sb.String()
	if !strings.Contains(strings.ToLower(response), "crystal clear") && !strings.Contains(strings.ToLower(response), "fundamentally means") {
		t.Errorf("expected two-strike fallback triggered by consecutive struggles, got: %s", response)
	}
}

func TestPostUserMessage_ContextCancellation(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(50 * time.Millisecond) // slow tokens
	svc := socratic.NewService(repo, llm)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
		Status:   socratic.SessionStatusActive,
	}

	ctx, cancel := context.WithCancel(context.Background())

	eventChan, err := svc.PostUserMessage(ctx, userID, sessionID, "Tell me about this word")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cancel after reading first event
	<-eventChan
	cancel()

	// Drain remaining events or wait for channel closure
	for range eventChan {
	}
}

func TestGetSessionHistory_Success(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "lucid",
		Status:   socratic.SessionStatusActive,
	}

	_, _ = repo.CreateMessage(context.Background(), sessionID, 1, socratic.RoleUser, "What is lucid?", nil)

	history, err := svc.GetSessionHistory(context.Background(), userID, sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if history.Session.ID != sessionID {
		t.Errorf("expected session id %s, got %s", sessionID, history.Session.ID)
	}

	if len(history.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(history.Messages))
	}
}

func TestReapInactiveSessions(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	activeStale := uuid.New()
	activeRecent := uuid.New()

	repo.sessions[activeStale] = &socratic.Session{
		ID:           activeStale,
		UserID:       uuid.New(),
		Status:       socratic.SessionStatusActive,
		LastActiveAt: time.Now().Add(-45 * time.Minute), // stale
	}

	repo.sessions[activeRecent] = &socratic.Session{
		ID:           activeRecent,
		UserID:       uuid.New(),
		Status:       socratic.SessionStatusActive,
		LastActiveAt: time.Now().Add(-5 * time.Minute), // active
	}

	reaped, err := svc.ReapInactiveSessions(context.Background(), 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reaped != 1 {
		t.Errorf("expected 1 session reaped, got %d", reaped)
	}

	if repo.sessions[activeStale].Status != socratic.SessionStatusAbandoned {
		t.Errorf("expected stale session to be abandoned, got %s", repo.sessions[activeStale].Status)
	}

	if repo.sessions[activeRecent].Status != socratic.SessionStatusActive {
		t.Errorf("expected recent session to stay active, got %s", repo.sessions[activeRecent].Status)
	}
}

func TestRules_BrevityAndQuestions(t *testing.T) {
	text := "This is a brief thought about vocabulary. Can you write a sentence with it?"
	words := socratic.CountWords(text)
	questions := socratic.CountQuestions(text)

	if words != 14 {
		t.Errorf("expected 14 words, got %d", words)
	}

	if questions != 1 {
		t.Errorf("expected 1 question, got %d", questions)
	}

	cleaned := socratic.CleanToken("token\r\n")
	if cleaned != "token" {
		t.Errorf("expected 'token', got %q", cleaned)
	}
}

func TestPostUserMessage_LLMError(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(0)
	llm.SimulateError = errors.New("upstream model failure")
	svc := socratic.NewService(repo, llm)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
		Status:   socratic.SessionStatusActive,
	}

	_, err := svc.PostUserMessage(context.Background(), userID, sessionID, "hello")
	if !errors.Is(err, socratic.ErrLLMUnavailable) {
		t.Errorf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestPostUserMessage_AdaptiveSentenceBranch(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(0)
	svc := socratic.NewService(repo, llm)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
		Status:   socratic.SessionStatusActive,
	}

	// User writes a sentence using the target word
	eventChan, err := svc.PostUserMessage(context.Background(), userID, sessionID, "Fame in the digital era can be quite ephemeral.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sb strings.Builder
	for ev := range eventChan {
		if ev.Type == socratic.StreamEventToken {
			sb.WriteString(ev.Token)
		}
	}

	res := sb.String()
	if !strings.Contains(strings.ToLower(res), "effectively") {
		t.Errorf("expected sentence feedback praise, got: %s", res)
	}
}

func TestGetSessionHistory_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))

	_, err := svc.GetSessionHistory(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, socratic.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestService_AbandonSession(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))
	userID := uuid.New()
	sessionID := uuid.New()

	t.Run("Successfully abandons active session", func(t *testing.T) {
		repo.sessions[sessionID] = &socratic.Session{
			ID:          sessionID,
			UserID:      userID,
			WordText:    "ephemeral",
			HeadwordKey: "ephemeral",
			Status:      socratic.SessionStatusActive,
		}

		updated, err := svc.AbandonSession(context.Background(), userID, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != socratic.SessionStatusAbandoned {
			t.Errorf("got status %v, want abandoned", updated.Status)
		}
		if updated.EndedAt == nil {
			t.Errorf("expected ended_at timestamp to be populated")
		}
	})

	t.Run("Rejects non-existent session", func(t *testing.T) {
		_, err := svc.AbandonSession(context.Background(), userID, uuid.New())
		if !errors.Is(err, socratic.ErrSessionNotFound) {
			t.Errorf("expected ErrSessionNotFound, got %v", err)
		}
	})

	t.Run("Rejects already abandoned session", func(t *testing.T) {
		_, err := svc.AbandonSession(context.Background(), userID, sessionID)
		if !errors.Is(err, socratic.ErrSessionNotActive) {
			t.Errorf("expected ErrSessionNotActive, got %v", err)
		}
	})
}

func TestService_ResumeSession(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))
	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:          sessionID,
		UserID:      userID,
		WordText:    "ephemeral",
		HeadwordKey: "ephemeral",
		Status:      socratic.SessionStatusAbandoned,
	}

	t.Run("Successfully resumes abandoned session", func(t *testing.T) {
		updated, err := svc.ResumeSession(context.Background(), userID, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != socratic.SessionStatusActive {
			t.Errorf("got status %v, want active", updated.Status)
		}
		if updated.EndedAt != nil {
			t.Errorf("expected ended_at to be nil on resume")
		}
	})

	t.Run("Rejects resume on active session", func(t *testing.T) {
		_, err := svc.ResumeSession(context.Background(), userID, sessionID)
		if !errors.Is(err, socratic.ErrSessionNotAbandoned) {
			t.Errorf("expected ErrSessionNotAbandoned, got %v", err)
		}
	})

	t.Run("Rejects resume if another active session exists for word", func(t *testing.T) {
		// First abandon the session again
		_, err := svc.AbandonSession(context.Background(), userID, sessionID)
		if err != nil {
			t.Fatalf("failed to abandon: %v", err)
		}

		// Create another active session for "ephemeral"
		activeID := uuid.New()
		repo.sessions[activeID] = &socratic.Session{
			ID:          activeID,
			UserID:      userID,
			WordText:    "ephemeral",
			HeadwordKey: "ephemeral",
			Status:      socratic.SessionStatusActive,
		}

		_, err = svc.ResumeSession(context.Background(), userID, sessionID)
		if !errors.Is(err, socratic.ErrActiveSessionExists) {
			t.Errorf("expected ErrActiveSessionExists, got %v", err)
		}
	})
}


