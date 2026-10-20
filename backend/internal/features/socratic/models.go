// Purpose: Domain entities, state types, and event models for Socratic learning sessions.
// Architecture: backend/internal/features/socratic/models.go
package socratic

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the finite state machine states for learning sessions (SRS 6.2).
type SessionStatus string

const (
	SessionStatusActive     SessionStatus = "active"
	SessionStatusCommitting SessionStatus = "committing"
	SessionStatusCommitted  SessionStatus = "committed"
	SessionStatusAbandoned  SessionStatus = "abandoned"
	SessionStatusFailed     SessionStatus = "failed"
)

// Role designates the speaker in a Socratic conversation turn.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Common Socratic domain errors.
var (
	ErrSessionNotFound     = errors.New("learning session not found")
	ErrSessionNotActive    = errors.New("learning session is not active")
	ErrSessionNotAbandoned = errors.New("learning session is not abandoned")
	ErrActiveSessionExists = errors.New("an active session already exists for this word")
	ErrEmptyMessage        = errors.New("message content cannot be empty")
	ErrMessageTooLong      = errors.New("message content exceeds maximum limit of 2000 characters")
	ErrLLMUnavailable      = errors.New("ai tutor service is temporarily unavailable")
)

/// Session encapsulates the lifecycle state and context of a Socratic exploration.
type Session struct {
	ID            uuid.UUID     `json:"id"`
	UserID        uuid.UUID     `json:"user_id"`
	WordText      string        `json:"word_text"`
	HeadwordKey   string        `json:"headword_key"`
	Status        SessionStatus `json:"status"`
	ContextNote   string        `json:"context_note,omitempty"`
	SnapshotID    uuid.UUID     `json:"snapshot_id,omitempty"`
	PromptVersion string        `json:"prompt_version"`
	ModelID       string        `json:"model_id,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	LastActiveAt  time.Time     `json:"last_active_at"`
	EndedAt       *time.Time    `json:"ended_at,omitempty"`
}

/// Message represents an immutable turn in the Socratic dialogue.
type Message struct {
	ID         uuid.UUID  `json:"id"`
	SessionID  uuid.UUID  `json:"session_id"`
	Seq        int32      `json:"seq"`
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	TokenCount *int32     `json:"token_count,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

/// StreamEventType categorizes Server-Sent Events (SSE) messages emitted to the client.
type StreamEventType string

const (
	StreamEventToken StreamEventType = "token"
	StreamEventDone  StreamEventType = "done"
	StreamEventError StreamEventType = "error"
)

/// StreamEvent is the payload serialized over the SSE connection for real-time streaming.
type StreamEvent struct {
	Type       StreamEventType `json:"type"`
	Token      string          `json:"token,omitempty"`
	Seq        int32           `json:"seq,omitempty"`
	Done       bool            `json:"done,omitempty"`
	TokenCount int             `json:"token_count,omitempty"`
	Error      string          `json:"error,omitempty"`
}

/// SendMessageRequest encapsulates user input when sending a conversational message.
type SendMessageRequest struct {
	Content string `json:"content"`
}

/// SessionHistoryResponse contains a session and its ordered messages.
type SessionHistoryResponse struct {
	Session  *Session   `json:"session"`
	Messages []Message `json:"messages"`
}
