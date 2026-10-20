// Purpose: LLM provider interface and mock streaming engine for Socratic dialogue.
// Architecture: backend/internal/features/socratic/llm_client.go
package socratic

import (
	"context"
	"fmt"
	"strings"
	"time"
)

/// ChatStreamRequest encapsulates input parameters for generating a Socratic assistant turn.
type ChatStreamRequest struct {
	SystemPrompt       string
	WordText           string
	OfficialDefinition string
	Messages           []Message
	DirectExplanation  bool
}

/// LLMClient abstracts third-party AI model providers (Gemini, OpenAI, Claude) behind a streaming interface.
type LLMClient interface {
	StreamChat(ctx context.Context, req ChatStreamRequest) (<-chan StreamEvent, error)
}

/// MockLLMClient provides a deterministic in-memory AI tutor for testing and offline development.
type MockLLMClient struct {
	ChunkDelay     time.Duration
	CustomResponse string
	SimulateError  error
}

/// NewMockLLMClient constructs a mock LLM client with realistic or zero token delay.
func NewMockLLMClient(delay time.Duration) *MockLLMClient {
	return &MockLLMClient{
		ChunkDelay: delay,
	}
}

/// StreamChat generates an adaptive response and streams tokens across a read-only Go channel.
func (m *MockLLMClient) StreamChat(ctx context.Context, req ChatStreamRequest) (<-chan StreamEvent, error) {
	if m.SimulateError != nil {
		return nil, m.SimulateError
	}

	outChan := make(chan StreamEvent, 16)

	// Determine response text based on Socratic directives
	responseText := m.determineResponse(req)

	go func() {
		defer close(outChan)

		// Split response into word-sized tokens to simulate realistic AI streaming
		tokens := tokenizeResponse(responseText)

		for _, token := range tokens {
			select {
			case <-ctx.Done():
				// Client disconnected or context canceled; halt streaming
				return
			default:
				if m.ChunkDelay > 0 {
					time.Sleep(m.ChunkDelay)
				}

				select {
				case <-ctx.Done():
					return
				case outChan <- StreamEvent{
					Type:  StreamEventToken,
					Token: token,
				}:
				}
			}
		}
	}()

	return outChan, nil
}

func (m *MockLLMClient) determineResponse(req ChatStreamRequest) string {
	if m.CustomResponse != "" {
		return m.CustomResponse
	}

	// 1. Two-Strike direct explanation fallback
	if req.DirectExplanation {
		return fmt.Sprintf("Let's make this crystal clear. '%s' fundamentally means: %s. For instance, think of how seasons change or how morning fog disappears by noon—both are classic examples. How might you describe something temporary in your daily routine using '%s'?", req.WordText, req.OfficialDefinition, req.WordText)
	}

	// 2. Analyze user's latest message to adaptively probe
	var latestUserMsg string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == RoleUser {
			latestUserMsg = req.Messages[i].Content
			break
		}
	}

	lowerMsg := strings.ToLower(latestUserMsg)

	// Check if user crafted a sentence containing the word
	if strings.Contains(lowerMsg, strings.ToLower(req.WordText)) {
		return fmt.Sprintf("You used '%s' effectively there! The sentence demonstrates the nuance well. What other context—like art, technology, or human relationships—could you apply it to?", req.WordText)
	}

	// Check if user gave a passive or generic response
	if isStrugglingOrPassive(lowerMsg) {
		return fmt.Sprintf("I hear you. To really anchor '%s', can you think of a situation where you felt something fleeting or temporary?", req.WordText)
	}

	// Default adaptive probing question
	return fmt.Sprintf("That is an intriguing angle on '%s'. How would you distinguish it from a close synonym, like 'fleeting' or 'transient'?", req.WordText)
}

func tokenizeResponse(text string) []string {
	words := strings.Fields(text)
	tokens := make([]string, 0, len(words))
	for i, w := range words {
		if i < len(words)-1 {
			tokens = append(tokens, w+" ")
		} else {
			tokens = append(tokens, w)
		}
	}
	return tokens
}
