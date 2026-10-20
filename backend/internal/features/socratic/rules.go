// Purpose: Socratic rule enforcers for turns, brevity, non-repetition, and 2-strike explanation fallback.
// Architecture: backend/internal/features/socratic/rules.go
package socratic

import (
	"strings"
	"unicode"
)

// Direct explanation triggers expressing confusion or requesting direct answers.
var directExplanationTriggers = []string{
	"just tell me",
	"i don't know",
	"idk",
	"explain it",
	"explain please",
	"what is it",
	"what does it mean",
	"can you explain",
	"give me the answer",
	"tell me what it means",
	"no idea",
}

// Passive agreement indicators that must never be treated as demonstrated comprehension (AI-002).
var passiveAgreementPhrases = []string{
	"yes",
	"yeah",
	"yep",
	"ok",
	"okay",
	"makes sense",
	"i get it",
	"i understand",
	"got it",
	"sure",
	"alright",
}

/// ShouldTriggerDirectExplanation checks if the user's message history demands switching
/// immediately from Socratic questioning to direct explanation (FR-013).
func ShouldTriggerDirectExplanation(latestUserMessage string, history []Message) bool {
	normalized := strings.ToLower(strings.TrimSpace(latestUserMessage))

	// Check 1: Explicit user request ("just tell me", "I don't know")
	for _, trigger := range directExplanationTriggers {
		if strings.Contains(normalized, trigger) {
			return true
		}
	}

	// Check 2: Two consecutive user turns demonstrating struggle or passive agreement without substance
	consecutiveStruggles := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == RoleUser {
			text := strings.ToLower(strings.TrimSpace(history[i].Content))
			if isStrugglingOrPassive(text) {
				consecutiveStruggles++
				if consecutiveStruggles >= 2 {
					return true
				}
			} else {
				// Reset count if user produced a substantive response
				break
			}
		}
	}

	return false
}

func isStrugglingOrPassive(text string) bool {
	if text == "" {
		return true
	}
	for _, trigger := range directExplanationTriggers {
		if strings.Contains(text, trigger) {
			return true
		}
	}
	for _, passive := range passiveAgreementPhrases {
		if text == passive || strings.HasPrefix(text, passive+" ") || strings.HasSuffix(text, " "+passive) {
			return true
		}
	}
	// Very short responses (< 15 characters) with no punctuation or vocabulary depth
	if len(text) < 15 && len(strings.Fields(text)) <= 3 {
		return true
	}
	return false
}

/// CountWords computes the approximate word count in a text block for brevity checks (AI-006).
func CountWords(text string) int {
	return len(strings.Fields(text))
}

/// CountQuestions returns the number of question marks in the text to verify the single-question rule.
func CountQuestions(text string) int {
	count := 0
	for _, r := range text {
		if r == '?' {
			count++
		}
	}
	return count
}

/// CleanToken trims leading/trailing whitespace control characters while preserving semantic spaces.
func CleanToken(s string) string {
	return strings.TrimRightFunc(s, unicode.IsControl)
}
