// Purpose: AI tutor interface and mock client for structured commit assessments and personal definition synthesis.
// Architecture: backend/internal/features/library/ai_client.go
package library

import (
	"context"
	"fmt"
	"strings"
)

/// AssessmentInput carries the grounding definition and full session dialogue needed for commit evaluation.
type AssessmentInput struct {
	WordText           string
	OfficialDefinition string
	Messages           []HistoryMessage
	ContextNote        string
}

/// AssessmentOutput contains the structured verdict, reasoning, and draft personal fields (SRS AI-010).
type AssessmentOutput struct {
	Verdict                     string  `json:"verdict"` // sufficient | partial | inaccurate
	Reasoning                   string  `json:"reasoning"`
	CandidatePersonalDefinition string  `json:"candidate_personal_definition"`
	CandidateExampleSentence    string  `json:"candidate_example_sentence,omitempty"`
	ContextualNotes             string  `json:"contextual_notes,omitempty"`
	DiscrepancyNote             *string `json:"discrepancy_note,omitempty"`
	PromptVersion               string  `json:"prompt_version"`
}

/// CommitAssessmentClient defines the contract for structured AI commit assessments.
type CommitAssessmentClient interface {
	GenerateDraft(ctx context.Context, input AssessmentInput) (*AssessmentOutput, error)
}

/// MockAssessmentClient provides deterministic commit drafting for testing and local development.
type MockAssessmentClient struct {
	CustomVerdict string
	SimulateError error
}

/// NewMockAssessmentClient constructs a deterministic assessment client.
func NewMockAssessmentClient() *MockAssessmentClient {
	return &MockAssessmentClient{}
}

/// GenerateDraft synthesizes a personal definition rooted in user messages (SRS AI-011).
func (m *MockAssessmentClient) GenerateDraft(ctx context.Context, input AssessmentInput) (*AssessmentOutput, error) {
	if m.SimulateError != nil {
		return nil, m.SimulateError
	}

	// Extract user statements from conversation history
	var userStatements []string
	var userExampleSentence string

	for _, msg := range input.Messages {
		if strings.EqualFold(msg.Role, "user") {
			content := strings.TrimSpace(msg.Content)
			if content != "" {
				userStatements = append(userStatements, content)
				// Look for an original sentence containing the target word
				if strings.Contains(strings.ToLower(content), strings.ToLower(input.WordText)) && len(content) > 15 {
					userExampleSentence = content
				}
			}
		}
	}

	// Synthesize personal definition derived strictly from user conversation statements (AI-011)
	candidateDef := fmt.Sprintf("A quality or state of being fleeting, short-lived, or quickly dissolving like mist.")
	if len(userStatements) > 0 {
		candidateDef = fmt.Sprintf("Understanding based on reflection: %s", userStatements[len(userStatements)-1])
	}

	verdict := "sufficient"
	if m.CustomVerdict != "" {
		verdict = m.CustomVerdict
	}

	var discrepancy *string
	reasoning := fmt.Sprintf("The learner demonstrated functional comprehension of '%s' by describing its temporal and fleeting nature in context.", input.WordText)

	if verdict == "inaccurate" {
		note := fmt.Sprintf("The user interpreted '%s' as lasting or eternal, which contradicts the official definition.", input.WordText)
		discrepancy = &note
		reasoning = "Demonstrated usage conflicted with lexical standards."
	}

	return &AssessmentOutput{
		Verdict:                     verdict,
		Reasoning:                   reasoning,
		CandidatePersonalDefinition: candidateDef,
		CandidateExampleSentence:    userExampleSentence,
		ContextualNotes:             input.ContextNote,
		DiscrepancyNote:             discrepancy,
		PromptVersion:               "v1.0",
	}, nil
}
