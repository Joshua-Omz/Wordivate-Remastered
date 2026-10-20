// Purpose: Domain entities, DTOs, and request/response models for vocabulary library and transactional commit.
// Architecture: backend/internal/features/library/models.go
package library

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// UnderstandingFlag designates whether the committed personal understanding aligns with lexical reality (SRS FR-024).
type UnderstandingFlag string

const (
	UnderstandingFlagOK          UnderstandingFlag = "ok"
	UnderstandingFlagNeedsReview UnderstandingFlag = "needs_review"
)

// PersonalDefOrigin tracks the authorial provenance of the personal definition (SRS FR-023).
type PersonalDefOrigin string

const (
	PersonalDefOriginUserWritten          PersonalDefOrigin = "user_written"
	PersonalDefOriginUserApprovedAIEdit   PersonalDefOrigin = "user_approved_ai_edit"
)

// Common library and commit domain errors.
var (
	ErrEntryNotFound              = errors.New("vocabulary entry not found")
	ErrSessionNotFound            = errors.New("learning session not found")
	ErrSessionNotActive           = errors.New("learning session is not in active or committing state")
	ErrMissingPersonalDefinition  = errors.New("personal definition is required and cannot be blank")
	ErrInvalidOrigin              = errors.New("personal definition origin must be 'user_written' or 'user_approved_ai_edit'")
	ErrInvalidUnderstandingFlag   = errors.New("understanding flag must be 'ok' or 'needs_review'")
	ErrAlreadyCommitted           = errors.New("learning session has already been committed")
	ErrAssessmentUnavailable      = errors.New("ai tutor commit assessment is temporarily unavailable")
)

/// VocabEntry represents persistent, user-specific vocabulary knowledge stored in the library.
type VocabEntry struct {
	ID                 uuid.UUID         `json:"id"`
	UserID             uuid.UUID         `json:"user_id"`
	WordText           string            `json:"word_text"`
	HeadwordKey        string            `json:"headword_key"`
	SenseKey           string            `json:"sense_key"`
	SenseLabel         string            `json:"sense_label,omitempty"`
	PartOfSpeech       string            `json:"part_of_speech,omitempty"`
	SnapshotID         uuid.UUID         `json:"snapshot_id"`
	OfficialDefinition string            `json:"official_definition"`
	PersonalDefinition string            `json:"personal_definition"`
	PersonalDefOrigin  PersonalDefOrigin `json:"personal_def_origin"`
	ExampleSentence    string            `json:"example_sentence,omitempty"`
	ContextualNotes    string            `json:"contextual_notes,omitempty"`
	UnderstandingFlag  UnderstandingFlag `json:"understanding_flag"`
	DiscrepancyNote    *string           `json:"discrepancy_note,omitempty"`
	OriginSessionID    uuid.UUID         `json:"origin_session_id"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

/// EntryRevision records an audit revision of a vocabulary entry (SRS FR-034).
type EntryRevision struct {
	ID        uuid.UUID              `json:"id"`
	EntryID   uuid.UUID              `json:"entry_id"`
	RevNo     int32                  `json:"rev_no"`
	Changes   map[string]interface{} `json:"changes"`
	ChangedAt time.Time              `json:"changed_at"`
}

/// CommitAssessment captures the AI tutor's verdict and structured reasoning at commit time.
type CommitAssessment struct {
	SessionID       uuid.UUID              `json:"session_id"`
	Verdict         string                 `json:"verdict"` // sufficient, partial, inaccurate
	DiscrepancyNote *string                `json:"discrepancy_note,omitempty"`
	RawOutput       map[string]interface{} `json:"raw_output"`
	PromptVersion   string                 `json:"prompt_version"`
	CreatedAt       time.Time              `json:"created_at"`
}

/// CommitDraftResponse returns structured AI synthesis derived strictly from user conversation text.
type CommitDraftResponse struct {
	Verdict                     string  `json:"verdict"` // sufficient, partial, inaccurate
	Reasoning                   string  `json:"reasoning"`
	CandidatePersonalDefinition string  `json:"candidate_personal_definition"`
	CandidateExampleSentence    string  `json:"candidate_example_sentence,omitempty"`
	ContextualNotes             string  `json:"contextual_notes,omitempty"`
	DiscrepancyNote             *string `json:"discrepancy_note,omitempty"`
	PromptVersion               string  `json:"prompt_version"`
}

/// CommitSessionRequest encapsulates user payload to atomically persist a vocabulary entry.
type CommitSessionRequest struct {
	PersonalDefinition string            `json:"personal_definition"`
	PersonalDefOrigin  PersonalDefOrigin `json:"personal_def_origin"`
	ExampleSentence    string            `json:"example_sentence,omitempty"`
	ContextualNotes    string            `json:"contextual_notes,omitempty"`
	UnderstandingFlag  UnderstandingFlag `json:"understanding_flag"`
	DiscrepancyNote    *string           `json:"discrepancy_note,omitempty"`
	SenseKey           string            `json:"sense_key,omitempty"`
	SenseLabel         string            `json:"sense_label,omitempty"`
	PartOfSpeech       string            `json:"part_of_speech,omitempty"`
}

/// CommitSessionResponse returns the newly committed entry and assessment metadata.
type CommitSessionResponse struct {
	Entry       *VocabEntry       `json:"entry"`
	Assessment  *CommitAssessment `json:"assessment,omitempty"`
	IsDuplicate bool              `json:"is_duplicate"`
}

/// UpdateEntryRequest contains mutable user fields for editing an existing vocabulary entry.
type UpdateEntryRequest struct {
	PersonalDefinition *string            `json:"personal_definition,omitempty"`
	ExampleSentence    *string            `json:"example_sentence,omitempty"`
	ContextualNotes    *string            `json:"contextual_notes,omitempty"`
	UnderstandingFlag  *UnderstandingFlag `json:"understanding_flag,omitempty"`
	DiscrepancyNote    *string            `json:"discrepancy_note,omitempty"`
}

/// ListEntriesFilter defines multi-dimensional search and filtering options for the personal library.
type ListEntriesFilter struct {
	Query             string
	PartOfSpeech      string
	UnderstandingFlag string
	StartDate         *time.Time
	EndDate           *time.Time
	Limit             int32
	Offset            int32
}

/// OriginSessionInfo holds summary metadata about the session that spawned the library entry.
type OriginSessionInfo struct {
	SessionID uuid.UUID `json:"session_id"`
	WordText  string    `json:"word_text"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

/// HistoryMessage represents a conversational turn attached to an entry detail view.
type HistoryMessage struct {
	Seq       int32     `json:"seq"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

/// EntryDetailResponse provides side-by-side official/personal definitions and origin chat history.
type EntryDetailResponse struct {
	Entry               *VocabEntry       `json:"entry"`
	Revisions           []EntryRevision   `json:"revisions"`
	OriginSession       OriginSessionInfo `json:"origin_session"`
	ConversationHistory []HistoryMessage  `json:"conversation_history"`
}
