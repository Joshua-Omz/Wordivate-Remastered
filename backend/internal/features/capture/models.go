// Purpose: Data transfer objects (DTOs) and request/response models for word capture and lexical lookup.
// Architecture: backend/internal/features/capture/models.go
package capture

import (
	"time"

	"github.com/google/uuid"
)

/// SenseItem represents a distinct lexical sense extracted from a dictionary snapshot.
type SenseItem struct {
	SenseKey     string   `json:"sense_key"`
	PartOfSpeech string   `json:"part_of_speech"`
	Definition   string   `json:"definition"`
	Example      string   `json:"example,omitempty"`
	Synonyms     []string `json:"synonyms,omitempty"`
}

/// LookupResponse returns the lexical senses and snapshot reference for a queried headword.
type LookupResponse struct {
	WordText    string      `json:"word_text"`
	HeadwordKey string      `json:"headword_key"`
	Phonetic    string      `json:"phonetic,omitempty"`
	SnapshotID  uuid.UUID   `json:"snapshot_id"`
	Source      string      `json:"source"`
	Senses      []SenseItem `json:"senses"`
}

/// StartSessionRequest represents the input parameters to initiate a Socratic learning session.
type StartSessionRequest struct {
	WordText         string `json:"word_text"`
	ContextNote      string `json:"context_note,omitempty"`
	SelectedSenseKey string `json:"selected_sense_key,omitempty"`
}

/// ExistingEntrySummary provides duplicate awareness when capturing a previously saved term (SRS FR-004).
type ExistingEntrySummary struct {
	ID                 uuid.UUID `json:"id"`
	SenseKey           string    `json:"sense_key"`
	OfficialDefinition string    `json:"official_definition"`
	PersonalDefinition string    `json:"personal_definition"`
	CreatedAt          time.Time `json:"created_at"`
}

/// StartSessionResponse returns the initialized or resumed session state.
type StartSessionResponse struct {
	SessionID       uuid.UUID              `json:"session_id"`
	WordText        string                 `json:"word_text"`
	HeadwordKey     string                 `json:"headword_key"`
	Status          string                 `json:"status"`
	SnapshotID      uuid.UUID              `json:"snapshot_id"`
	IsResumed       bool                   `json:"is_resumed"`
	ExistingEntries []ExistingEntrySummary `json:"existing_entries,omitempty"`
	PromptVersion   string                 `json:"prompt_version"`
}
