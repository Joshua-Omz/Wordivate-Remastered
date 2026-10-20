// Purpose: Domain service for transactional vocabulary commit, idempotency validation, and library exploration.
// Architecture: backend/internal/features/library/service.go
package library

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

/// Service defines the domain business logic for vocabulary commits and library management.
type Service interface {
	GenerateCommitDraft(ctx context.Context, userID, sessionID uuid.UUID) (*CommitDraftResponse, error)
	CommitSession(ctx context.Context, userID, sessionID uuid.UUID, idempotencyKey string, req CommitSessionRequest) (*CommitSessionResponse, error)
	ListEntries(ctx context.Context, userID uuid.UUID, filter ListEntriesFilter) ([]VocabEntry, error)
	GetEntryDetail(ctx context.Context, userID, entryID uuid.UUID) (*EntryDetailResponse, error)
	UpdateEntry(ctx context.Context, userID, entryID uuid.UUID, req UpdateEntryRequest) (*VocabEntry, error)
	DeleteEntry(ctx context.Context, userID, entryID uuid.UUID) error
}

type libraryService struct {
	repo     LibraryRepository
	aiClient CommitAssessmentClient
}

/// NewService instantiates a library domain service with injected repository and assessment client.
func NewService(repo LibraryRepository, aiClient CommitAssessmentClient) Service {
	return &libraryService{
		repo:     repo,
		aiClient: aiClient,
	}
}

/// GenerateCommitDraft inspects session dialogue and generates structured synthesis (SRS AI-010).
func (s *libraryService) GenerateCommitDraft(ctx context.Context, userID, sessionID uuid.UUID) (*CommitDraftResponse, error) {
	session, err := s.repo.GetSessionForCommit(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	if session.Status == "committed" {
		return nil, ErrAlreadyCommitted
	}
	if session.Status != "active" && session.Status != "committing" {
		return nil, ErrSessionNotActive
	}

	// Update session state machine to 'committing' (SRS 6.2)
	_ = s.repo.UpdateSessionStatus(ctx, sessionID, userID, "committing")

	messages, err := s.repo.GetSessionMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetching session dialogue: %w", err)
	}

	officialDef := "Definition unavailable."
	if session.SnapshotID != uuid.Nil {
		snapshot, err := s.repo.GetSnapshotByID(ctx, session.SnapshotID)
		if err == nil && snapshot != nil {
			officialDef = formatSnapshotDefinition(snapshot.Payload)
		}
	}

	// Call structured assessment AI client
	assessment, err := s.aiClient.GenerateDraft(ctx, AssessmentInput{
		WordText:           session.WordText,
		OfficialDefinition: officialDef,
		Messages:           messages,
		ContextNote:        session.ContextNote,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssessmentUnavailable, err)
	}

	return &CommitDraftResponse{
		Verdict:                     assessment.Verdict,
		Reasoning:                   assessment.Reasoning,
		CandidatePersonalDefinition: assessment.CandidatePersonalDefinition,
		CandidateExampleSentence:    assessment.CandidateExampleSentence,
		ContextualNotes:             assessment.ContextualNotes,
		DiscrepancyNote:             assessment.DiscrepancyNote,
		PromptVersion:               assessment.PromptVersion,
	}, nil
}

/// CommitSession atomically commits a session into a persistent vocabulary entry (SRS FR-020).
func (s *libraryService) CommitSession(ctx context.Context, userID, sessionID uuid.UUID, idempotencyKey string, req CommitSessionRequest) (*CommitSessionResponse, error) {
	// 1. Validate required personal definition (Cardinal Rule 1)
	trimmedDef := strings.TrimSpace(req.PersonalDefinition)
	if trimmedDef == "" {
		return nil, ErrMissingPersonalDefinition
	}

	// 2. Validate authorial origin (SRS FR-023)
	if req.PersonalDefOrigin != PersonalDefOriginUserWritten && req.PersonalDefOrigin != PersonalDefOriginUserApprovedAIEdit {
		return nil, ErrInvalidOrigin
	}

	// 3. Validate understanding flag (SRS FR-024)
	flag := req.UnderstandingFlag
	if flag == "" {
		flag = UnderstandingFlagOK
	}
	if flag != UnderstandingFlagOK && flag != UnderstandingFlagNeedsReview {
		return nil, ErrInvalidUnderstandingFlag
	}

	// 4. Idempotency Key validation and cached replay (SRS FR-027)
	cleanKey := strings.TrimSpace(idempotencyKey)
	var requestHash []byte
	if cleanKey != "" {
		reqBytes, err := json.Marshal(req)
		if err == nil {
			hash := sha256.Sum256(reqBytes)
			requestHash = hash[:]
		}

		existingKey, err := s.repo.GetIdempotencyKey(ctx, userID, cleanKey)
		if err == nil && existingKey != nil {
			if len(existingKey.Response) > 0 {
				var cachedResp CommitSessionResponse
				if err := json.Unmarshal(existingKey.Response, &cachedResp); err == nil {
					cachedResp.IsDuplicate = true
					return &cachedResp, nil
				}
			}
			return nil, errors.New("concurrent mutation with identical idempotency key in progress")
		}

		_ = s.repo.CreateIdempotencyKey(ctx, userID, cleanKey, requestHash)
	}

	// 5. Verify session state
	session, err := s.repo.GetSessionForCommit(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	if session.Status == "committed" {
		return nil, ErrAlreadyCommitted
	}
	if session.Status != "active" && session.Status != "committing" {
		return nil, ErrSessionNotActive
	}

	// 6. Retrieve grounded lexical definition
	officialDef := "Lexical definition unavailable."
	if session.SnapshotID != uuid.Nil {
		snapshot, err := s.repo.GetSnapshotByID(ctx, session.SnapshotID)
		if err == nil && snapshot != nil {
			officialDef = formatSnapshotDefinition(snapshot.Payload)
		}
	}

	// 7. Execute atomic transaction (session status, entry, rev 1, assessment)
	txParams := CommitTxParams{
		UserID:             userID,
		SessionID:          sessionID,
		WordText:           session.WordText,
		HeadwordKey:        session.HeadwordKey,
		SenseKey:           req.SenseKey,
		SenseLabel:         req.SenseLabel,
		PartOfSpeech:       req.PartOfSpeech,
		SnapshotID:         session.SnapshotID,
		OfficialDefinition: officialDef,
		PersonalDefinition: trimmedDef,
		PersonalDefOrigin:  req.PersonalDefOrigin,
		ExampleSentence:    strings.TrimSpace(req.ExampleSentence),
		ContextualNotes:    strings.TrimSpace(req.ContextualNotes),
		UnderstandingFlag:  flag,
		DiscrepancyNote:    req.DiscrepancyNote,
		AssessmentVerdict:  "sufficient",
		AssessmentReason:   "Committed through verified Socratic dialogue.",
		AssessmentRaw: map[string]interface{}{
			"verdict":            "sufficient",
			"personal_def":       trimmedDef,
			"understanding_flag": string(flag),
		},
		PromptVersion: session.PromptVersion,
	}

	entry, assessment, err := s.repo.CommitSessionTx(ctx, txParams)
	if err != nil {
		return nil, fmt.Errorf("executing atomic commit transaction: %w", err)
	}

	resp := &CommitSessionResponse{
		Entry:       entry,
		Assessment:  assessment,
		IsDuplicate: false,
	}

	// 8. Store response payload against idempotency key for safe future retries
	if cleanKey != "" {
		respJSON, err := json.Marshal(resp)
		if err == nil {
			_ = s.repo.SaveIdempotencyResponse(ctx, userID, cleanKey, respJSON)
		}
	}

	return resp, nil
}

/// ListEntries queries the library with optional full-text search, POS, and date filtering.
func (s *libraryService) ListEntries(ctx context.Context, userID uuid.UUID, filter ListEntriesFilter) ([]VocabEntry, error) {
	trimmedQuery := strings.TrimSpace(filter.Query)
	if trimmedQuery != "" {
		return s.repo.SearchEntries(ctx, userID, trimmedQuery, filter.Limit, filter.Offset)
	}
	return s.repo.ListEntries(ctx, userID, filter)
}

/// GetEntryDetail returns side-by-side official/personal definitions, revisions, and origin chat turns.
func (s *libraryService) GetEntryDetail(ctx context.Context, userID, entryID uuid.UUID) (*EntryDetailResponse, error) {
	entry, err := s.repo.GetEntryByID(ctx, entryID, userID)
	if err != nil {
		return nil, err
	}

	revisions, err := s.repo.GetEntryRevisions(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("fetching entry revisions: %w", err)
	}

	var originSession OriginSessionInfo
	var conversationHistory []HistoryMessage

	if entry.OriginSessionID != uuid.Nil {
		sessRecord, err := s.repo.GetSessionForCommit(ctx, entry.OriginSessionID, userID)
		if err == nil && sessRecord != nil {
			originSession = OriginSessionInfo{
				SessionID: sessRecord.ID,
				WordText:  sessRecord.WordText,
				StartedAt: sessRecord.StartedAt,
			}
			if sessRecord.EndedAt != nil {
				originSession.EndedAt = *sessRecord.EndedAt
			}
		}

		msgs, err := s.repo.GetSessionMessages(ctx, entry.OriginSessionID)
		if err == nil {
			conversationHistory = msgs
		}
	}

	return &EntryDetailResponse{
		Entry:               entry,
		Revisions:           revisions,
		OriginSession:       originSession,
		ConversationHistory: conversationHistory,
	}, nil
}

/// UpdateEntry applies post-commit edits to personal fields, recording a new audit revision.
func (s *libraryService) UpdateEntry(ctx context.Context, userID, entryID uuid.UUID, req UpdateEntryRequest) (*VocabEntry, error) {
	if req.PersonalDefinition != nil {
		trimmed := strings.TrimSpace(*req.PersonalDefinition)
		if trimmed == "" {
			return nil, ErrMissingPersonalDefinition
		}
		req.PersonalDefinition = &trimmed
	}

	if req.UnderstandingFlag != nil {
		flag := *req.UnderstandingFlag
		if flag != UnderstandingFlagOK && flag != UnderstandingFlagNeedsReview {
			return nil, ErrInvalidUnderstandingFlag
		}
	}

	entry, _, err := s.repo.UpdateEntryPersonalTx(ctx, userID, entryID, req)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

/// DeleteEntry cascade-deletes a vocabulary entry and its audit revisions.
func (s *libraryService) DeleteEntry(ctx context.Context, userID, entryID uuid.UUID) error {
	// Verify entry exists and belongs to user
	_, err := s.repo.GetEntryByID(ctx, entryID, userID)
	if err != nil {
		return err
	}
	return s.repo.DeleteEntry(ctx, entryID, userID)
}

func formatSnapshotDefinition(payload []byte) string {
	type rawDefinition struct {
		Definition string `json:"definition"`
	}
	type rawMeaning struct {
		PartOfSpeech string          `json:"part_of_speech"`
		Definitions  []rawDefinition `json:"definitions"`
	}

	var meanings []rawMeaning
	if err := json.Unmarshal(payload, &meanings); err == nil && len(meanings) > 0 {
		var sb strings.Builder
		for _, m := range meanings {
			sb.WriteString(fmt.Sprintf("[%s]:\n", m.PartOfSpeech))
			for _, d := range m.Definitions {
				sb.WriteString(fmt.Sprintf("- %s\n", d.Definition))
			}
		}
		return sb.String()
	}

	return string(payload)
}
