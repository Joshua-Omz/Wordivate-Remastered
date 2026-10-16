package socratic

import (
	"context"
	"fmt"
)

// LLMClient defines the interface for AI prompt completion.
type LLMClient interface {
	GenerateSocraticQuestion(ctx context.Context, term, definition string) (string, error)
	EvaluateResponse(ctx context.Context, term, definition, userResponse string) (*EvaluationResult, error)
}

type mockLLMClient struct{}

// NewMockLLMClient returns a mock client for development/testing without live API keys.
func NewMockLLMClient() LLMClient {
	return &mockLLMClient{}
}

func (m *mockLLMClient) GenerateSocraticQuestion(ctx context.Context, term, definition string) (string, error) {
	return fmt.Sprintf("Can you craft a sentence using '%s' in the context of personal development or philosophy?", term), nil
}

func (m *mockLLMClient) EvaluateResponse(ctx context.Context, term, definition, userResponse string) (*EvaluationResult, error) {
	return &EvaluationResult{
		Score:      4,
		Critique:   fmt.Sprintf("Great application of '%s'! You captured its nuances accurately.", term),
		NextPrompt: "Now try contrasting it with its antonym in a single sentence.",
		IsComplete: true,
	}, nil
}
