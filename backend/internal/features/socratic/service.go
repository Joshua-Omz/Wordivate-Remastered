// Purpose: Domain service orchestrating Socratic state transitions, prompt assembly, and streaming turns.
// Architecture: backend/internal/features/socratic/service.go
package socratic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/socratic/prompts"
)

/// Service specifies the business logic interface for Socratic tutoring sessions.
type Service interface {
	PostUserMessage(ctx context.Context, userID, sessionID uuid.UUID, content string) (<-chan StreamEvent, error)
	GetSessionHistory(ctx context.Context, userID, sessionID uuid.UUID) (*SessionHistoryResponse, error)
	AbandonSession(ctx context.Context, userID, sessionID uuid.UUID) (*Session, error)
	ResumeSession(ctx context.Context, userID, sessionID uuid.UUID) (*Session, error)
	ReapInactiveSessions(ctx context.Context, timeout time.Duration) (int64, error)
}

type socraticService struct {
	repo      SocraticRepository
	llmClient LLMClient
}

/// NewService instantiates a new Socratic domain service with injected repository and LLM client.
func NewService(repo SocraticRepository, llmClient LLMClient) Service {
	return &socraticService{
		repo:      repo,
		llmClient: llmClient,
	}
}

/// PostUserMessage processes a learner turn, records it durably, and streams the assistant reply.
func (s *socraticService) PostUserMessage(ctx context.Context, userID, sessionID uuid.UUID, content string) (<-chan StreamEvent, error) {
	// 1. Input sanitization and length validation
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, ErrEmptyMessage
	}
	if len(trimmed) > 2000 {
		return nil, ErrMessageTooLong
	}

	// 2. Fetch session and enforce tenant isolation
	session, err := s.repo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	// 3. State machine validation (SRS 6.2)
	// Only active sessions may receive interactive conversational turns
	if session.Status != SessionStatusActive {
		return nil, ErrSessionNotActive
	}

	// 4. Retrieve monotonic sequential counter
	lastSeq, err := s.repo.GetLastMessageSeq(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("retrieving message sequence: %w", err)
	}

	userSeq := lastSeq + 1
	assistantSeq := userSeq + 1

	// 5. Append-only persistence of user message
	_, err = s.repo.CreateMessage(ctx, sessionID, userSeq, RoleUser, trimmed, nil)
	if err != nil {
		return nil, fmt.Errorf("persisting user message: %w", err)
	}

	// 6. Retrieve full conversation history for context assembly
	history, err := s.repo.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("retrieving session message history: %w", err)
	}

	// 7. Retrieve immutable definition snapshot for lexical grounding (DEF-004)
	officialDef := "Definition unavailable."
	if session.SnapshotID != uuid.Nil {
		snapshot, err := s.repo.GetSnapshotByID(ctx, session.SnapshotID)
		if err == nil && snapshot != nil {
			officialDef = formatSnapshotDefinition(snapshot.Payload)
		}
	}

	// 8. Evaluate Socratic rules: check two-strike direct explanation fallback (FR-013)
	directExplanation := ShouldTriggerDirectExplanation(trimmed, history)

	// 9. Assemble hydrated system prompt from versioned template (NFR-M2)
	promptVersion := session.PromptVersion
	if promptVersion == "" {
		promptVersion = "v1.0"
	}

	systemPrompt, err := prompts.Render(promptVersion, prompts.TemplateContext{
		WordText:           session.WordText,
		OfficialDefinition: officialDef,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering Socratic prompt: %w", err)
	}

	// 10. Call LLM streaming client
	llmChan, err := s.llmClient.StreamChat(ctx, ChatStreamRequest{
		SystemPrompt:       systemPrompt,
		WordText:           session.WordText,
		OfficialDefinition: officialDef,
		Messages:           history,
		DirectExplanation:  directExplanation,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
	}

	// 11. Pipeline LLM tokens through client-facing output channel with background persistence
	outChan := make(chan StreamEvent, 32)

	go func() {
		defer close(outChan)

		var fullResponse strings.Builder
		tokenCount := 0

		for event := range llmChan {
			if event.Type == StreamEventToken {
				fullResponse.WriteString(event.Token)
				tokenCount++
				event.Seq = assistantSeq
			}

			select {
			case <-ctx.Done():
				return
			case outChan <- event:
			}
		}

		// Save completed assistant message durably with independent timeout
		asstContent := fullResponse.String()
		if asstContent != "" {
			saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			tc := int32(tokenCount)
			_, _ = s.repo.CreateMessage(saveCtx, sessionID, assistantSeq, RoleAssistant, asstContent, &tc)
			_ = s.repo.UpdateSessionLastActive(saveCtx, sessionID, userID)
		}

		// Emit final completion event
		select {
		case <-ctx.Done():
			return
		case outChan <- StreamEvent{
			Type:       StreamEventDone,
			Seq:        assistantSeq,
			Done:       true,
			TokenCount: tokenCount,
		}:
		}
	}()

	return outChan, nil
}

/// GetSessionHistory retrieves a session and all its messages ordered by sequence.
func (s *socraticService) GetSessionHistory(ctx context.Context, userID, sessionID uuid.UUID) (*SessionHistoryResponse, error) {
	session, err := s.repo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	messages, err := s.repo.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetching session messages: %w", err)
	}

	return &SessionHistoryResponse{
		Session:  session,
		Messages: messages,
	}, nil
}

/// ReapInactiveSessions scans for active sessions with no activity for >timeout and marks them abandoned.
func (s *socraticService) ReapInactiveSessions(ctx context.Context, timeout time.Duration) (int64, error) {
	cutoff := time.Now().Add(-timeout)
	return s.repo.AbandonInactiveSessions(ctx, cutoff)
}

/// AbandonSession explicitly marks an active learning session as abandoned.
func (s *socraticService) AbandonSession(ctx context.Context, userID, sessionID uuid.UUID) (*Session, error) {
	session, err := s.repo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	if session.Status != SessionStatusActive {
		return nil, ErrSessionNotActive
	}

	return s.repo.AbandonSession(ctx, sessionID, userID)
}

/// ResumeSession re-activates an abandoned learning session if no active session exists for the headword.
func (s *socraticService) ResumeSession(ctx context.Context, userID, sessionID uuid.UUID) (*Session, error) {
	session, err := s.repo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	if session.Status != SessionStatusAbandoned {
		return nil, ErrSessionNotAbandoned
	}

	// Collision guard: Ensure no other active session exists for this user and headword
	active, err := s.repo.GetActiveSessionByWord(ctx, userID, session.HeadwordKey)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrActiveSessionExists
	}

	return s.repo.ResumeSession(ctx, sessionID, userID)
}


func formatSnapshotDefinition(payload []byte) string {
	type rawDefinition struct {
		Definition string `json:"definition"`
	}
	type rawMeaning struct {
		PartOfSpeech string          `json:"part_of_speech"`
		Definitions  []rawDefinition `json:"definitions"`
	}

	var meanings []rawMeaning
	if err := json.Unmarshal(payload, &meanings); err == nil && len(meanings) > 0 {
		var sb strings.Builder
		for _, m := range meanings {
			sb.WriteString(fmt.Sprintf("[%s]:\n", m.PartOfSpeech))
			for _, d := range m.Definitions {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Definition))
			}
		}
		return sb.String()
	}

	return string(payload)
}
