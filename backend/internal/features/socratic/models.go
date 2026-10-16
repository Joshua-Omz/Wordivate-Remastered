package socratic

import "time"

// MessageType indicates the type of payload over the WebSocket connection.
type MessageType string

const (
	MessageTypeQuestion MessageType = "question"
	MessageTypeAnswer   MessageType = "answer"
	MessageTypeFeedback MessageType = "feedback"
	MessageTypeError    MessageType = "error"
)

// WSMessage is the standardized envelope for Socratic WebSocket communication.
type WSMessage struct {
	Type      MessageType `json:"type"`
	Content   string      `json:"content"`
	WordID    string      `json:"word_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// EvaluationResult represents AI feedback on the learner's usage of a word.
type EvaluationResult struct {
	Score       int    `json:"score"` // 0-5
	Critique    string `json:"critique"`
	NextPrompt  string `json:"next_prompt"`
	IsComplete  bool   `json:"is_complete"`
}
