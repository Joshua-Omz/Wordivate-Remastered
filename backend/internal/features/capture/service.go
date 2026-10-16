// Purpose: Domain service for word capture, lexical normalization, and learning session initialization.
// Architecture: backend/internal/features/capture/service.go
package capture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

var (
	// ErrEmptyWord indicates a blank or whitespace-only word input.
	ErrEmptyWord = errors.New("word text cannot be empty")

	// ErrWordTooLong indicates a word exceeding the 100-character architectural limit.
	ErrWordTooLong = errors.New("word text cannot exceed 100 characters")

	// ErrInvalidWordFormat indicates unprintable or illegal control characters in the word string.
	ErrInvalidWordFormat = errors.New("word contains invalid characters")
)

/// Service defines lexical lookup and session capture operations.
type Service interface {
	NormalizeHeadword(raw string) (string, error)
	LookupWord(ctx context.Context, word string) (*LookupResponse, error)
	StartSession(ctx context.Context, userID uuid.UUID, req StartSessionRequest) (*StartSessionResponse, error)
}

type service struct {
	repo       CaptureRepository
	dictClient DictionaryClient
}

/// NewService instantiates a production-ready capture service.
/// Parameters:
/// - repo: domain persistence interface
/// - dictClient: abstract lexical provider
func NewService(repo CaptureRepository, dictClient DictionaryClient) Service {
	return &service{
		repo:       repo,
		dictClient: dictClient,
	}
}

/// NormalizeHeadword sanitizes, trims, and validates an unfamiliar term.
func (s *service) NormalizeHeadword(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrEmptyWord
	}

	if len(trimmed) > 100 {
		return "", ErrWordTooLong
	}

	// Reject control characters or newlines
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", ErrInvalidWordFormat
		}
	}

	return strings.ToLower(trimmed), nil
}

/// LookupWord queries the lexical provider, persists an immutable snapshot, and returns senses.
func (s *service) LookupWord(ctx context.Context, word string) (*LookupResponse, error) {
	headword, err := s.NormalizeHeadword(word)
	if err != nil {
		return nil, err
	}

	// 1. Fetch definition from external provider
	defResult, err := s.dictClient.FetchDefinition(ctx, headword)
	if err != nil {
		return nil, fmt.Errorf("fetching definition: %w", err)
	}

	// 2. Ensure payload serialization is deterministic for SHA-256 hashing
	var payloadBytes []byte
	if len(defResult.RawJSON) > 0 {
		payloadBytes = defResult.RawJSON
	} else {
		payloadBytes, err = json.Marshal(defResult.Meanings)
		if err != nil {
			return nil, fmt.Errorf("serializing snapshot payload: %w", err)
		}
	}

	// 3. Compute immutable cryptographic hash
	contentHash := ComputeContentHash(payloadBytes)

	// 4. Persist or retrieve definition snapshot
	snapshot, err := s.repo.GetOrCreateSnapshot(
		ctx,
		headword,
		defResult.Source,
		defResult.SourceRef,
		payloadBytes,
		contentHash,
	)
	if err != nil {
		return nil, fmt.Errorf("persisting definition snapshot: %w", err)
	}

	// 5. Flatten senses with composite sense keys (e.g. "adjective.0")
	senses := make([]SenseItem, 0)
	for _, m := range defResult.Meanings {
		for i, d := range m.Definitions {
			senseKey := fmt.Sprintf("%s.%d", strings.ToLower(m.PartOfSpeech), i)
			senses = append(senses, SenseItem{
				SenseKey:     senseKey,
				PartOfSpeech: m.PartOfSpeech,
				Definition:   d.Definition,
				Example:      d.Example,
				Synonyms:     d.Synonyms,
			})
		}
	}

	return &LookupResponse{
		WordText:    word,
		HeadwordKey: headword,
		Phonetic:    defResult.Phonetic,
		SnapshotID:  snapshot.ID,
		Source:      snapshot.Source,
		Senses:      senses,
	}, nil
}

/// StartSession initializes an active Socratic learning session with duplicate detection.
func (s *service) StartSession(ctx context.Context, userID uuid.UUID, req StartSessionRequest) (*StartSessionResponse, error) {
	headword, err := s.NormalizeHeadword(req.WordText)
	if err != nil {
		return nil, err
	}

	// 1. Duplicate Detection (SRS FR-004): Check if user already committed this word
	existingRecords, err := s.repo.GetExistingEntriesByWord(ctx, userID, headword)
	if err != nil {
		return nil, fmt.Errorf("checking existing vocabulary entries: %w", err)
	}

	existingSummaries := make([]ExistingEntrySummary, 0, len(existingRecords))
	for _, r := range existingRecords {
		existingSummaries = append(existingSummaries, ExistingEntrySummary{
			ID:                 r.ID,
			SenseKey:           r.SenseKey,
			OfficialDefinition: r.OfficialDefinition,
			PersonalDefinition: r.PersonalDefinition,
			CreatedAt:          r.CreatedAt,
		})
	}

	// 2. Active Session Check (SRS 7.4 Partial Index one_active_session_per_word)
	activeSession, err := s.repo.GetActiveSessionForWord(ctx, userID, headword)
	if err == nil && activeSession != nil {
		// Idempotently resume existing active session
		return &StartSessionResponse{
			SessionID:       activeSession.ID,
			WordText:        activeSession.WordText,
			HeadwordKey:     activeSession.HeadwordKey,
			Status:          activeSession.Status,
			SnapshotID:      activeSession.SnapshotID,
			IsResumed:       true,
			ExistingEntries: existingSummaries,
			PromptVersion:   activeSession.PromptVersion,
		}, nil
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("checking active session: %w", err)
	}

	// 3. Ensure definition snapshot is provisioned
	lookup, err := s.LookupWord(ctx, headword)
	if err != nil {
		return nil, fmt.Errorf("grounding word definition: %w", err)
	}

	// 4. Create new learning session in 'active' status
	session, err := s.repo.CreateSession(
		ctx,
		userID,
		strings.TrimSpace(req.WordText),
		headword,
		strings.TrimSpace(req.ContextNote),
		lookup.SnapshotID,
		"v1.0",
		"default",
	)
	if err != nil {
		return nil, fmt.Errorf("creating learning session: %w", err)
	}

	return &StartSessionResponse{
		SessionID:       session.ID,
		WordText:        session.WordText,
		HeadwordKey:     session.HeadwordKey,
		Status:          session.Status,
		SnapshotID:      session.SnapshotID,
		IsResumed:       false,
		ExistingEntries: existingSummaries,
		PromptVersion:   session.PromptVersion,
	}, nil
}
