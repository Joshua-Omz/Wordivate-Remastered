// Purpose: Comprehensive unit tests for transactional vocabulary commit, idempotency, and library exploration.
// Architecture: backend/internal/features/library/service_test.go
package library_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/library"
)

type mockRepository struct {
	mu           sync.Mutex
	sessions     map[uuid.UUID]*library.SessionRecord
	messages     map[uuid.UUID][]library.HistoryMessage
	snapshots    map[uuid.UUID]*library.SnapshotRecord
	entries      map[uuid.UUID]*library.VocabEntry
	revisions    map[uuid.UUID][]library.EntryRevision
	assessments  map[uuid.UUID]*library.CommitAssessment
	idempotency  map[string]*library.IdempotencyData
	failCommitTx bool
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		sessions:    make(map[uuid.UUID]*library.SessionRecord),
		messages:    make(map[uuid.UUID][]library.HistoryMessage),
		snapshots:   make(map[uuid.UUID]*library.SnapshotRecord),
		entries:     make(map[uuid.UUID]*library.VocabEntry),
		revisions:   make(map[uuid.UUID][]library.EntryRevision),
		assessments: make(map[uuid.UUID]*library.CommitAssessment),
		idempotency: make(map[string]*library.IdempotencyData),
	}
}

func (m *mockRepository) GetSessionForCommit(ctx context.Context, sessionID, userID uuid.UUID) (*library.SessionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[sessionID]
	if !exists || s.UserID != userID {
		return nil, library.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID) ([]library.HistoryMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msgs := m.messages[sessionID]
	res := make([]library.HistoryMessage, len(msgs))
	copy(res, msgs)
	return res, nil
}

func (m *mockRepository) GetSnapshotByID(ctx context.Context, snapshotID uuid.UUID) (*library.SnapshotRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, exists := m.snapshots[snapshotID]
	if !exists {
		return nil, errors.New("snapshot not found")
	}
	return snap, nil
}

func (m *mockRepository) UpdateSessionStatus(ctx context.Context, sessionID, userID uuid.UUID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sessions[sessionID]
	if !exists || s.UserID != userID {
		return library.ErrSessionNotFound
	}
	s.Status = status
	return nil
}

func (m *mockRepository) GetIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*library.IdempotencyData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapKey := userID.String() + ":" + key
	data, exists := m.idempotency[mapKey]
	if !exists {
		return nil, nil
	}
	return data, nil
}

func (m *mockRepository) CreateIdempotencyKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapKey := userID.String() + ":" + key
	m.idempotency[mapKey] = &library.IdempotencyData{
		UserID:      userID,
		Key:         key,
		RequestHash: requestHash,
		CreatedAt:   time.Now(),
	}
	return nil
}

func (m *mockRepository) SaveIdempotencyResponse(ctx context.Context, userID uuid.UUID, key string, response []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapKey := userID.String() + ":" + key
	if item, exists := m.idempotency[mapKey]; exists {
		item.Response = response
	}
	return nil
}

func (m *mockRepository) CommitSessionTx(ctx context.Context, params library.CommitTxParams) (*library.VocabEntry, *library.CommitAssessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failCommitTx {
		return nil, nil, errors.New("simulated transaction failure")
	}

	entryID := uuid.New()
	now := time.Now()

	entry := &library.VocabEntry{
		ID:                 entryID,
		UserID:             params.UserID,
		WordText:           params.WordText,
		HeadwordKey:        params.HeadwordKey,
		SenseKey:           params.SenseKey,
		SenseLabel:         params.SenseLabel,
		PartOfSpeech:       params.PartOfSpeech,
		SnapshotID:         params.SnapshotID,
		OfficialDefinition: params.OfficialDefinition,
		PersonalDefinition: params.PersonalDefinition,
		PersonalDefOrigin:  params.PersonalDefOrigin,
		ExampleSentence:    params.ExampleSentence,
		ContextualNotes:    params.ContextualNotes,
		UnderstandingFlag:  params.UnderstandingFlag,
		DiscrepancyNote:    params.DiscrepancyNote,
		OriginSessionID:    params.SessionID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	rev1 := library.EntryRevision{
		ID:      uuid.New(),
		EntryID: entryID,
		RevNo:   1,
		Changes: map[string]interface{}{
			"personal_definition": params.PersonalDefinition,
			"revision_type":       "initial_commit",
		},
		ChangedAt: now,
	}

	assessment := &library.CommitAssessment{
		SessionID:       params.SessionID,
		Verdict:         params.AssessmentVerdict,
		DiscrepancyNote: params.DiscrepancyNote,
		RawOutput:       params.AssessmentRaw,
		PromptVersion:   params.PromptVersion,
		CreatedAt:       now,
	}

	// Persist mock records atomically
	m.entries[entryID] = entry
	m.revisions[entryID] = []library.EntryRevision{rev1}
	m.assessments[params.SessionID] = assessment

	if s, exists := m.sessions[params.SessionID]; exists {
		s.Status = "committed"
		s.EndedAt = &now
	}

	return entry, assessment, nil
}

func (m *mockRepository) GetEntryByID(ctx context.Context, id, userID uuid.UUID) (*library.VocabEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, exists := m.entries[id]
	if !exists || e.UserID != userID {
		return nil, library.ErrEntryNotFound
	}
	return e, nil
}

func (m *mockRepository) GetEntryRevisions(ctx context.Context, entryID uuid.UUID) ([]library.EntryRevision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	revs := m.revisions[entryID]
	res := make([]library.EntryRevision, len(revs))
	copy(res, revs)
	return res, nil
}

func (m *mockRepository) ListEntries(ctx context.Context, userID uuid.UUID, filter library.ListEntriesFilter) ([]library.VocabEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]library.VocabEntry, 0)
	for _, e := range m.entries {
		if e.UserID != userID {
			continue
		}
		if filter.PartOfSpeech != "" && e.PartOfSpeech != filter.PartOfSpeech {
			continue
		}
		if filter.UnderstandingFlag != "" && string(e.UnderstandingFlag) != filter.UnderstandingFlag {
			continue
		}
		result = append(result, *e)
	}
	return result, nil
}

func (m *mockRepository) SearchEntries(ctx context.Context, userID uuid.UUID, query string, limit, offset int32) ([]library.VocabEntry, error) {
	return m.ListEntries(ctx, userID, library.ListEntriesFilter{})
}

func (m *mockRepository) UpdateEntryPersonalTx(ctx context.Context, userID, entryID uuid.UUID, req library.UpdateEntryRequest) (*library.VocabEntry, *library.EntryRevision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, exists := m.entries[entryID]
	if !exists || e.UserID != userID {
		return nil, nil, library.ErrEntryNotFound
	}

	if req.PersonalDefinition != nil {
		e.PersonalDefinition = *req.PersonalDefinition
	}
	if req.ExampleSentence != nil {
		e.ExampleSentence = *req.ExampleSentence
	}
	if req.ContextualNotes != nil {
		e.ContextualNotes = *req.ContextualNotes
	}
	if req.UnderstandingFlag != nil {
		e.UnderstandingFlag = *req.UnderstandingFlag
	}
	if req.DiscrepancyNote != nil {
		e.DiscrepancyNote = req.DiscrepancyNote
	}
	e.UpdatedAt = time.Now()

	nextRevNo := int32(len(m.revisions[entryID]) + 1)
	rev := library.EntryRevision{
		ID:        uuid.New(),
		EntryID:   entryID,
		RevNo:     nextRevNo,
		Changes:   map[string]interface{}{"updated": true},
		ChangedAt: time.Now(),
	}
	m.revisions[entryID] = append(m.revisions[entryID], rev)

	return e, &rev, nil
}

func (m *mockRepository) DeleteEntry(ctx context.Context, id, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, exists := m.entries[id]
	if !exists || e.UserID != userID {
		return library.ErrEntryNotFound
	}
	delete(m.entries, id)
	delete(m.revisions, id)
	return nil
}

// -------------------------------------------------------------------------
// Unit Tests
// -------------------------------------------------------------------------

func TestGenerateCommitDraft_Success(t *testing.T) {
	repo := newMockRepository()
	ai := library.NewMockAssessmentClient()
	svc := library.NewService(repo, ai)

	userID := uuid.New()
	sessionID := uuid.New()
	snapID := uuid.New()

	repo.sessions[sessionID] = &library.SessionRecord{
		ID:            sessionID,
		UserID:        userID,
		WordText:      "ephemeral",
		Status:        "active",
		SnapshotID:    snapID,
		PromptVersion: "v1.0",
	}

	repo.messages[sessionID] = []library.HistoryMessage{
		{Seq: 1, Role: "user", Content: "I notice cherry blossoms are ephemeral."},
	}

	repo.snapshots[snapID] = &library.SnapshotRecord{
		ID:          snapID,
		HeadwordKey: "ephemeral",
		Payload:     []byte(`[{"part_of_speech":"adjective","definitions":[{"definition":"Lasting a short time."}]}]`),
	}

	draft, err := svc.GenerateCommitDraft(context.Background(), userID, sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if draft.Verdict != "sufficient" {
		t.Errorf("expected sufficient verdict, got %s", draft.Verdict)
	}
	if draft.CandidateExampleSentence == "" {
		t.Errorf("expected extracted user example sentence, got empty")
	}

	// Verify session transitioned to 'committing' state
	if repo.sessions[sessionID].Status != "committing" {
		t.Errorf("expected status 'committing', got %s", repo.sessions[sessionID].Status)
	}
}

func TestCommitSession_AtomicSuccess(t *testing.T) {
	repo := newMockRepository()
	ai := library.NewMockAssessmentClient()
	svc := library.NewService(repo, ai)

	userID := uuid.New()
	sessionID := uuid.New()
	snapID := uuid.New()

	repo.sessions[sessionID] = &library.SessionRecord{
		ID:            sessionID,
		UserID:        userID,
		WordText:      "ephemeral",
		HeadwordKey:   "ephemeral",
		Status:        "committing",
		SnapshotID:    snapID,
		PromptVersion: "v1.0",
	}

	repo.snapshots[snapID] = &library.SnapshotRecord{
		ID:          snapID,
		HeadwordKey: "ephemeral",
		Payload:     []byte(`[{"part_of_speech":"adjective","definitions":[{"definition":"Lasting a short time."}]}]`),
	}

	req := library.CommitSessionRequest{
		PersonalDefinition: "Something fleeting that vanishes quickly like morning dew.",
		PersonalDefOrigin:  library.PersonalDefOriginUserWritten,
		ExampleSentence:    "Morning dew is ephemeral.",
		ContextualNotes:    "Metaphorical nature usage",
		UnderstandingFlag:  library.UnderstandingFlagOK,
		PartOfSpeech:       "adjective",
	}

	resp, err := svc.CommitSession(context.Background(), userID, sessionID, "idemp-key-101", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Entry.WordText != "ephemeral" {
		t.Errorf("expected entry word 'ephemeral', got %s", resp.Entry.WordText)
	}

	// Verify revision 1 exists
	revs, _ := repo.GetEntryRevisions(context.Background(), resp.Entry.ID)
	if len(revs) != 1 || revs[0].RevNo != 1 {
		t.Errorf("expected 1 initial revision with rev_no 1, got %+v", revs)
	}

	// Verify session is marked committed
	if repo.sessions[sessionID].Status != "committed" {
		t.Errorf("expected session committed, got %s", repo.sessions[sessionID].Status)
	}

	// Test Idempotency Replay: repeating exact call with identical key must return cached duplicate
	repeatResp, err := svc.CommitSession(context.Background(), userID, sessionID, "idemp-key-101", req)
	if err != nil {
		t.Fatalf("unexpected idempotency replay error: %v", err)
	}
	if !repeatResp.IsDuplicate {
		t.Errorf("expected repeat response to be marked IsDuplicate = true")
	}
	if repeatResp.Entry.ID != resp.Entry.ID {
		t.Errorf("expected identical entry ID on replay, got %s vs %s", repeatResp.Entry.ID, resp.Entry.ID)
	}
}

func TestCommitSession_ValidationErrors(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	sessionID := uuid.New()

	// 1. Missing personal definition
	_, err := svc.CommitSession(context.Background(), userID, sessionID, "", library.CommitSessionRequest{
		PersonalDefinition: "  ",
	})
	if !errors.Is(err, library.ErrMissingPersonalDefinition) {
		t.Errorf("expected ErrMissingPersonalDefinition, got %v", err)
	}

	// 2. Invalid personal definition origin
	_, err = svc.CommitSession(context.Background(), userID, sessionID, "", library.CommitSessionRequest{
		PersonalDefinition: "Valid def",
		PersonalDefOrigin:  "invalid_origin",
	})
	if !errors.Is(err, library.ErrInvalidOrigin) {
		t.Errorf("expected ErrInvalidOrigin, got %v", err)
	}

	// 3. Invalid understanding flag
	_, err = svc.CommitSession(context.Background(), userID, sessionID, "", library.CommitSessionRequest{
		PersonalDefinition: "Valid def",
		PersonalDefOrigin:  library.PersonalDefOriginUserWritten,
		UnderstandingFlag:  "invalid_flag",
	})
	if !errors.Is(err, library.ErrInvalidUnderstandingFlag) {
		t.Errorf("expected ErrInvalidUnderstandingFlag, got %v", err)
	}
}

func TestCommitSession_AlreadyCommitted(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &library.SessionRecord{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
		Status:   "committed",
	}

	req := library.CommitSessionRequest{
		PersonalDefinition: "Valid def",
		PersonalDefOrigin:  library.PersonalDefOriginUserWritten,
	}

	_, err := svc.CommitSession(context.Background(), userID, sessionID, "", req)
	if !errors.Is(err, library.ErrAlreadyCommitted) {
		t.Errorf("expected ErrAlreadyCommitted, got %v", err)
	}
}

func TestEntryDetail_SideBySideHistory(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	entryID := uuid.New()
	sessionID := uuid.New()

	repo.entries[entryID] = &library.VocabEntry{
		ID:                 entryID,
		UserID:             userID,
		WordText:           "ephemeral",
		OfficialDefinition: "Lasting for a very short time.",
		PersonalDefinition: "Like a Snapchat message that disappears.",
		OriginSessionID:    sessionID,
	}

	repo.sessions[sessionID] = &library.SessionRecord{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
	}

	repo.messages[sessionID] = []library.HistoryMessage{
		{Seq: 1, Role: "user", Content: "Is ephemeral like social media stories?"},
	}

	repo.revisions[entryID] = []library.EntryRevision{
		{ID: uuid.New(), EntryID: entryID, RevNo: 1},
	}

	detail, err := svc.GetEntryDetail(context.Background(), userID, entryID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Entry.OfficialDefinition != "Lasting for a very short time." {
		t.Errorf("mismatched official definition: %s", detail.Entry.OfficialDefinition)
	}
	if detail.Entry.PersonalDefinition != "Like a Snapchat message that disappears." {
		t.Errorf("mismatched personal definition: %s", detail.Entry.PersonalDefinition)
	}
	if len(detail.ConversationHistory) != 1 {
		t.Errorf("expected 1 conversation history turn, got %d", len(detail.ConversationHistory))
	}
}

func TestUpdateEntry_AppendsRevision(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	entryID := uuid.New()

	repo.entries[entryID] = &library.VocabEntry{
		ID:                 entryID,
		UserID:             userID,
		WordText:           "ephemeral",
		PersonalDefinition: "Original def",
	}
	repo.revisions[entryID] = []library.EntryRevision{
		{ID: uuid.New(), EntryID: entryID, RevNo: 1},
	}

	newDef := "Revised definition with deeper clarity"
	updated, err := svc.UpdateEntry(context.Background(), userID, entryID, library.UpdateEntryRequest{
		PersonalDefinition: &newDef,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.PersonalDefinition != newDef {
		t.Errorf("expected updated personal definition, got %s", updated.PersonalDefinition)
	}

	revs, _ := repo.GetEntryRevisions(context.Background(), entryID)
	if len(revs) != 2 || revs[1].RevNo != 2 {
		t.Errorf("expected revision 2 appended, got %+v", revs)
	}
}

func TestDeleteEntry_Success(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	entryID := uuid.New()

	repo.entries[entryID] = &library.VocabEntry{
		ID:     entryID,
		UserID: userID,
	}

	if err := svc.DeleteEntry(context.Background(), userID, entryID); err != nil {
		t.Fatalf("unexpected error deleting entry: %v", err)
	}

	_, err := repo.GetEntryByID(context.Background(), entryID, userID)
	if !errors.Is(err, library.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound after deletion, got %v", err)
	}
}

func TestListEntries_SearchQuery(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	entryID := uuid.New()

	repo.entries[entryID] = &library.VocabEntry{
		ID:                 entryID,
		UserID:             userID,
		WordText:           "serendipity",
		PersonalDefinition: "Fortunate accident.",
	}

	entries, err := svc.ListEntries(context.Background(), userID, library.ListEntriesFilter{
		Query: "fortunate",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 search result, got %d", len(entries))
	}
}

func TestUpdateEntry_ValidationErrors(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	userID := uuid.New()
	entryID := uuid.New()

	// 1. Empty personal definition
	emptyDef := "   "
	_, err := svc.UpdateEntry(context.Background(), userID, entryID, library.UpdateEntryRequest{
		PersonalDefinition: &emptyDef,
	})
	if !errors.Is(err, library.ErrMissingPersonalDefinition) {
		t.Errorf("expected ErrMissingPersonalDefinition, got %v", err)
	}

	// 2. Invalid understanding flag
	badFlag := library.UnderstandingFlag("corrupted")
	validDef := "Valid definition"
	_, err = svc.UpdateEntry(context.Background(), userID, entryID, library.UpdateEntryRequest{
		PersonalDefinition: &validDef,
		UnderstandingFlag:  &badFlag,
	})
	if !errors.Is(err, library.ErrInvalidUnderstandingFlag) {
		t.Errorf("expected ErrInvalidUnderstandingFlag, got %v", err)
	}
}

func TestGenerateCommitDraft_Errors(t *testing.T) {
	repo := newMockRepository()
	ai := library.NewMockAssessmentClient()
	svc := library.NewService(repo, ai)

	userID := uuid.New()
	sessionID := uuid.New()

	// 1. Session not found
	_, err := svc.GenerateCommitDraft(context.Background(), userID, sessionID)
	if !errors.Is(err, library.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}

	// 2. Already committed
	repo.sessions[sessionID] = &library.SessionRecord{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ephemeral",
		Status:   "committed",
	}
	_, err = svc.GenerateCommitDraft(context.Background(), userID, sessionID)
	if !errors.Is(err, library.ErrAlreadyCommitted) {
		t.Errorf("expected ErrAlreadyCommitted, got %v", err)
	}

	// 3. Not active or committing (e.g., abandoned)
	repo.sessions[sessionID].Status = "abandoned"
	_, err = svc.GenerateCommitDraft(context.Background(), userID, sessionID)
	if !errors.Is(err, library.ErrSessionNotActive) {
		t.Errorf("expected ErrSessionNotActive, got %v", err)
	}

	// 4. AI assessment failure
	repo.sessions[sessionID].Status = "active"
	ai.SimulateError = errors.New("upstream timeout")
	_, err = svc.GenerateCommitDraft(context.Background(), userID, sessionID)
	if !errors.Is(err, library.ErrAssessmentUnavailable) {
		t.Errorf("expected ErrAssessmentUnavailable, got %v", err)
	}
}

func TestGetEntryDetail_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	_, err := svc.GetEntryDetail(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, library.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestDeleteEntry_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())

	err := svc.DeleteEntry(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, library.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

