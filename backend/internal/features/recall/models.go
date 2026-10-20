// Purpose: Domain models, request/response DTOs, and error types for Adaptive Recall (Sprint 6).
// Architecture: backend/internal/features/recall/models.go
package recall

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidRating indicates a score outside the 0-5 SuperMemo scale.
	ErrInvalidRating = errors.New("rating must be an integer between 0 and 5")

	// ErrInvalidWordID indicates a malformed or non-UUID word identifier.
	ErrInvalidWordID = errors.New("word_id must be a valid UUID")

	// ErrEntryNotFound indicates the vocabulary entry does not exist or does not belong to the user.
	ErrEntryNotFound = errors.New("vocabulary entry not found or unauthorized")

	// ErrUnauthorizedUser indicates an unauthenticated or invalid user identity.
	ErrUnauthorizedUser = errors.New("unauthorized user")
)

// DueWordItem represents a vocabulary entry due for spaced retrieval practice.
type DueWordItem struct {
	ID                 uuid.UUID  `json:"id"`
	Term               string     `json:"term"`
	Definition         string     `json:"definition"` // Alias for personal_definition (backwards compatibility)
	OfficialDefinition string     `json:"official_definition"`
	PersonalDefinition string     `json:"personal_definition"`
	PartOfSpeech       string     `json:"part_of_speech,omitempty"`
	ExampleSentence    string     `json:"example_sentence,omitempty"`
	ContextualNotes    string     `json:"contextual_notes,omitempty"`
	UnderstandingFlag  string     `json:"understanding_flag"`
	Interval           int        `json:"interval"` // Alias for interval_days (backwards compatibility)
	IntervalDays       int        `json:"interval_days"`
	EaseFactor         float64    `json:"ease_factor"`
	RepetitionCount    int        `json:"repetition_count"`
	NextReview         time.Time  `json:"next_review"`
	LastReviewed       *time.Time `json:"last_reviewed,omitempty"`
}

// DueWordsResponse encapsulates the list and count of words ready for recall.
type DueWordsResponse struct {
	Words []DueWordItem `json:"words"`
	Count int           `json:"count"`
}

// ReviewSubmission represents the user's recall feedback submission for an SRS item.
type ReviewSubmission struct {
	WordID string `json:"word_id"`
	Rating int    `json:"rating"` // 0-5 SuperMemo rating scale
}

// RecallSchedule represents the persistent state of an entry's spaced repetition interval.
type RecallSchedule struct {
	EntryID         uuid.UUID  `json:"entry_id"`
	UserID          uuid.UUID  `json:"user_id"`
	IntervalDays    int        `json:"interval_days"`
	EaseFactor      float64    `json:"ease_factor"`
	RepetitionCount int        `json:"repetition_count"`
	NextReviewAt    time.Time  `json:"next_review_at"`
	LastReviewedAt  *time.Time `json:"last_reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ReviewResult encapsulates the updated scheduling outcome after processing a review.
type ReviewResult struct {
	EntryID         uuid.UUID `json:"entry_id"`
	RepetitionCount int       `json:"repetition_count"`
	IntervalDays    int       `json:"interval_days"`
	EaseFactor      float64   `json:"ease_factor"`
	NextReviewAt    time.Time `json:"next_review_at"`
	ReviewedAt      time.Time `json:"reviewed_at"`
}
