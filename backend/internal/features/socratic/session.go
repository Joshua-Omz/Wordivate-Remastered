package socratic

import (
	"context"
	"time"

	"github.com/Joshua-Omz/wordivate/backend/internal/database"
)

// SessionManager coordinates the multi-turn Socratic questioning loop.
type SessionManager struct {
	db  *database.DB
	llm LLMClient
}

// NewSessionManager creates a new Socratic session manager.
func NewSessionManager(db *database.DB, llm LLMClient) *SessionManager {
	return &SessionManager{
		db:  db,
		llm: llm,
	}
}

// StartSession initiates a Socratic dialogue for a word.
func (sm *SessionManager) StartSession(ctx context.Context, wordID, term, definition string) (*WSMessage, error) {
	question, err := sm.llm.GenerateSocraticQuestion(ctx, term, definition)
	if err != nil {
		return nil, err
	}

	return &WSMessage{
		Type:      MessageTypeQuestion,
		Content:   question,
		WordID:    wordID,
		Timestamp: time.Now(),
	}, nil
}

// ProcessAnswer evaluates the learner's response.
func (sm *SessionManager) ProcessAnswer(ctx context.Context, wordID, term, definition, answer string) (*WSMessage, error) {
	eval, err := sm.llm.EvaluateResponse(ctx, term, definition, answer)
	if err != nil {
		return nil, err
	}

	return &WSMessage{
		Type:      MessageTypeFeedback,
		Content:   eval.Critique,
		WordID:    wordID,
		Timestamp: time.Now(),
	}, nil
}
