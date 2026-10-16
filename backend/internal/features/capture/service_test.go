// Purpose: Comprehensive unit tests for word normalization, dictionary lookup, snapshot hashing, and session capture.
// Architecture: backend/internal/features/capture/service_test.go
package capture_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/capture"
)

// -------------------------------------------------------------------------
// In-Memory Mock Repository
// -------------------------------------------------------------------------

type mockCaptureRepository struct {
	snapshots       map[uuid.UUID]*capture.DefinitionSnapshot
	snapshotsByHash map[string]*capture.DefinitionSnapshot
	sessions        map[uuid.UUID]*capture.LearningSession
	vocabEntries    map[uuid.UUID][]capture.VocabEntryRecord
}

func newMockCaptureRepository() *mockCaptureRepository {
	return &mockCaptureRepository{
		snapshots:       make(map[uuid.UUID]*capture.DefinitionSnapshot),
		snapshotsByHash: make(map[string]*capture.DefinitionSnapshot),
		sessions:        make(map[uuid.UUID]*capture.LearningSession),
		vocabEntries:    make(map[uuid.UUID][]capture.VocabEntryRecord),
	}
}

func (m *mockCaptureRepository) GetOrCreateSnapshot(ctx context.Context, headwordKey, source string, sourceRef string, payload, contentHash []byte) (*capture.DefinitionSnapshot, error) {
	key := string(contentHash)
	if existing, ok := m.snapshotsByHash[key]; ok {
		return existing, nil
	}

	snap := &capture.DefinitionSnapshot{
		ID:          uuid.New(),
		HeadwordKey: headwordKey,
		Source:      source,
		SourceRef:   sourceRef,
		Payload:     payload,
		ContentHash: contentHash,
		RetrievedAt: time.Now(),
	}
	m.snapshots[snap.ID] = snap
	m.snapshotsByHash[key] = snap
	return snap, nil
}

func (m *mockCaptureRepository) GetSnapshotByID(ctx context.Context, id uuid.UUID) (*capture.DefinitionSnapshot, error) {
	snap, ok := m.snapshots[id]
	if !ok {
		return nil, capture.ErrNotFound
	}
	return snap, nil
}

func (m *mockCaptureRepository) GetActiveSessionForWord(ctx context.Context, userID uuid.UUID, headwordKey string) (*capture.LearningSession, error) {
	for _, s := range m.sessions {
		if s.UserID == userID && s.HeadwordKey == headwordKey && s.Status == "active" {
			return s, nil
		}
	}
	return nil, capture.ErrNotFound
}

func (m *mockCaptureRepository) CreateSession(ctx context.Context, userID uuid.UUID, wordText, headwordKey, contextNote string, snapshotID uuid.UUID, promptVersion, modelID string) (*capture.LearningSession, error) {
	s := &capture.LearningSession{
		ID:            uuid.New(),
		UserID:        userID,
		WordText:      wordText,
		HeadwordKey:   headwordKey,
		Status:        "active",
		ContextNote:   contextNote,
		SnapshotID:    snapshotID,
		PromptVersion: promptVersion,
		ModelID:       modelID,
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
	}
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockCaptureRepository) GetExistingEntriesByWord(ctx context.Context, userID uuid.UUID, headwordKey string) ([]capture.VocabEntryRecord, error) {
	entries, ok := m.vocabEntries[userID]
	if !ok {
		return []capture.VocabEntryRecord{}, nil
	}
	var matched []capture.VocabEntryRecord
	for _, e := range entries {
		if e.HeadwordKey == headwordKey {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

func (m *mockCaptureRepository) AddVocabEntry(entry capture.VocabEntryRecord) {
	m.vocabEntries[entry.UserID] = append(m.vocabEntries[entry.UserID], entry)
}

// -------------------------------------------------------------------------
// Unit Tests
// -------------------------------------------------------------------------

func TestNormalization(t *testing.T) {
	repo := newMockCaptureRepository()
	dict := capture.NewMockDictionaryClient()
	svc := capture.NewService(repo, dict)

	t.Run("Valid Word Normalization", func(t *testing.T) {
		got, err := svc.NormalizeHeadword("   Ephemeral   ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ephemeral" {
			t.Errorf("expected 'ephemeral', got '%s'", got)
		}
	})

	t.Run("Empty Word Error", func(t *testing.T) {
		_, err := svc.NormalizeHeadword("   ")
		if err != capture.ErrEmptyWord {
			t.Errorf("expected ErrEmptyWord, got %v", err)
		}
	})

	t.Run("Word Exceeding Limit Error", func(t *testing.T) {
		longWord := strings.Repeat("a", 101)
		_, err := svc.NormalizeHeadword(longWord)
		if err != capture.ErrWordTooLong {
			t.Errorf("expected ErrWordTooLong, got %v", err)
		}
	})

	t.Run("Control Character Error", func(t *testing.T) {
		_, err := svc.NormalizeHeadword("bad\nword")
		if err != capture.ErrInvalidWordFormat {
			t.Errorf("expected ErrInvalidWordFormat, got %v", err)
		}
	})
}

func TestLookupWord(t *testing.T) {
	repo := newMockCaptureRepository()
	dict := capture.NewMockDictionaryClient()
	svc := capture.NewService(repo, dict)
	ctx := context.Background()

	res, err := svc.LookupWord(ctx, "Ephemeral")
	if err != nil {
		t.Fatalf("LookupWord failed: %v", err)
	}

	if res.HeadwordKey != "ephemeral" {
		t.Errorf("expected headword 'ephemeral', got '%s'", res.HeadwordKey)
	}
	if res.SnapshotID == uuid.Nil {
		t.Errorf("expected valid non-nil SnapshotID")
	}
	if len(res.Senses) == 0 {
		t.Errorf("expected parsed senses, got 0")
	}

	// Verify snapshot hash was recorded
	snap, err := repo.GetSnapshotByID(ctx, res.SnapshotID)
	if err != nil {
		t.Fatalf("snapshot not found in repo: %v", err)
	}
	if len(snap.ContentHash) != 32 {
		t.Errorf("expected 32-byte SHA-256 hash, got %d bytes", len(snap.ContentHash))
	}

	// Repeated lookup must be idempotent and return identical SnapshotID
	res2, err := svc.LookupWord(ctx, "ephemeral")
	if err != nil {
		t.Fatalf("second lookup failed: %v", err)
	}
	if res2.SnapshotID != res.SnapshotID {
		t.Errorf("expected identical SnapshotID, got %v vs %v", res2.SnapshotID, res.SnapshotID)
	}
}

func TestStartSessionFreshAndResumed(t *testing.T) {
	repo := newMockCaptureRepository()
	dict := capture.NewMockDictionaryClient()
	svc := capture.NewService(repo, dict)
	ctx := context.Background()
	userID := uuid.New()

	// 1. Initial StartSession (Fresh Session)
	resp1, err := svc.StartSession(ctx, userID, capture.StartSessionRequest{
		WordText:    "  Serendipity ",
		ContextNote: "Reading an article about scientific discoveries",
	})
	if err != nil {
		t.Fatalf("initial StartSession failed: %v", err)
	}

	if resp1.IsResumed {
		t.Errorf("expected fresh session, got IsResumed=true")
	}
	if resp1.HeadwordKey != "serendipity" {
		t.Errorf("expected normalized headword 'serendipity', got '%s'", resp1.HeadwordKey)
	}
	if resp1.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", resp1.Status)
	}

	// 2. Second StartSession for same word (Should Idempotently Resume)
	resp2, err := svc.StartSession(ctx, userID, capture.StartSessionRequest{
		WordText: "serendipity",
	})
	if err != nil {
		t.Fatalf("second StartSession failed: %v", err)
	}

	if !resp2.IsResumed {
		t.Errorf("expected resumed session, got IsResumed=false")
	}
	if resp2.SessionID != resp1.SessionID {
		t.Errorf("expected matching SessionID %v, got %v", resp1.SessionID, resp2.SessionID)
	}
}

func TestDuplicateEntryAwareness(t *testing.T) {
	repo := newMockCaptureRepository()
	dict := capture.NewMockDictionaryClient()
	svc := capture.NewService(repo, dict)
	ctx := context.Background()
	userID := uuid.New()

	// Simulate that the user previously saved "ubiquitous"
	existingEntryID := uuid.New()
	repo.AddVocabEntry(capture.VocabEntryRecord{
		ID:                 existingEntryID,
		UserID:             userID,
		WordText:           "ubiquitous",
		HeadwordKey:        "ubiquitous",
		SenseKey:           "adjective.0",
		OfficialDefinition: "Present, appearing, or found everywhere.",
		PersonalDefinition: "Something you see in every corner of life.",
		CreatedAt:          time.Now().Add(-48 * time.Hour),
	})

	// Start a session for "ubiquitous"
	resp, err := svc.StartSession(ctx, userID, capture.StartSessionRequest{
		WordText: "ubiquitous",
	})
	if err != nil {
		t.Fatalf("StartSession with existing entries failed: %v", err)
	}

	if len(resp.ExistingEntries) != 1 {
		t.Fatalf("expected 1 existing entry notification, got %d", len(resp.ExistingEntries))
	}
	if resp.ExistingEntries[0].ID != existingEntryID {
		t.Errorf("expected entry ID %v, got %v", existingEntryID, resp.ExistingEntries[0].ID)
	}
}
