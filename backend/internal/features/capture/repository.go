// Purpose: Persistence adapter for lexical snapshots, learning sessions, and duplicate entry checks.
// Architecture: backend/internal/features/capture/repository.go
package capture

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/database/sqlc"
)

// ErrNotFound indicates the queried record does not exist in the database.
var ErrNotFound = errors.New("record not found")

/// DefinitionSnapshot represents an immutable dictionary snapshot grounded by cryptographic hash.
type DefinitionSnapshot struct {
	ID          uuid.UUID
	HeadwordKey string
	Source      string
	SourceRef   string
	Payload     []byte
	ContentHash []byte
	RetrievedAt time.Time
}

/// LearningSession represents the lifecycle state of a Socratic exploration.
type LearningSession struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	WordText      string
	HeadwordKey   string
	Status        string
	ContextNote   string
	SnapshotID    uuid.UUID
	PromptVersion string
	ModelID       string
	CreatedAt     time.Time
	LastActiveAt  time.Time
	EndedAt       *time.Time
}

/// VocabEntryRecord encapsulates a user's committed vocabulary record for duplicate detection.
type VocabEntryRecord struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	WordText           string
	HeadwordKey        string
	SenseKey           string
	OfficialDefinition string
	PersonalDefinition string
	CreatedAt          time.Time
}

/// CaptureRepository defines the persistence contract required by the word capture domain.
type CaptureRepository interface {
	GetOrCreateSnapshot(ctx context.Context, headwordKey, source string, sourceRef string, payload, contentHash []byte) (*DefinitionSnapshot, error)
	GetSnapshotByID(ctx context.Context, id uuid.UUID) (*DefinitionSnapshot, error)
	GetActiveSessionForWord(ctx context.Context, userID uuid.UUID, headwordKey string) (*LearningSession, error)
	CreateSession(ctx context.Context, userID uuid.UUID, wordText, headwordKey, contextNote string, snapshotID uuid.UUID, promptVersion, modelID string) (*LearningSession, error)
	GetExistingEntriesByWord(ctx context.Context, userID uuid.UUID, headwordKey string) ([]VocabEntryRecord, error)
}

type sqlRepository struct {
	q *sqlc.Queries
}

/// NewRepository instantiates a concrete CaptureRepository backed by sqlc and PostgreSQL.
func NewRepository(db *sql.DB) CaptureRepository {
	return &sqlRepository{
		q: sqlc.New(db),
	}
}

func mapSnapshot(s sqlc.DefinitionSnapshot) *DefinitionSnapshot {
	var ref string
	if s.SourceRef.Valid {
		ref = s.SourceRef.String
	}
	return &DefinitionSnapshot{
		ID:          s.ID,
		HeadwordKey: s.HeadwordKey,
		Source:      s.Source,
		SourceRef:   ref,
		Payload:     s.Payload,
		ContentHash: s.ContentHash,
		RetrievedAt: s.RetrievedAt,
	}
}

func mapSession(s sqlc.LearningSession) *LearningSession {
	var note string
	if s.ContextNote.Valid {
		note = s.ContextNote.String
	}
	var model string
	if s.ModelID.Valid {
		model = s.ModelID.String
	}
	var snapshotID uuid.UUID
	if s.SnapshotID.Valid {
		snapshotID = s.SnapshotID.UUID
	}
	var ended *time.Time
	if s.EndedAt.Valid {
		t := s.EndedAt.Time
		ended = &t
	}
	return &LearningSession{
		ID:            s.ID,
		UserID:        s.UserID,
		WordText:      s.WordText,
		HeadwordKey:   s.HeadwordKey,
		Status:        s.Status,
		ContextNote:   note,
		SnapshotID:    snapshotID,
		PromptVersion: s.PromptVersion,
		ModelID:       model,
		CreatedAt:     s.StartedAt,
		LastActiveAt:  s.LastActiveAt,
		EndedAt:       ended,
	}
}

func (r *sqlRepository) GetOrCreateSnapshot(ctx context.Context, headwordKey, source string, sourceRef string, payload, contentHash []byte) (*DefinitionSnapshot, error) {
	dbSnap, err := r.q.CreateDefinitionSnapshot(ctx, sqlc.CreateDefinitionSnapshotParams{
		HeadwordKey: headwordKey,
		Source:      source,
		SourceRef: sql.NullString{
			String: sourceRef,
			Valid:  sourceRef != "",
		},
		Payload:     json.RawMessage(payload),
		ContentHash: contentHash,
	})
	if err != nil {
		return nil, err
	}
	return mapSnapshot(dbSnap), nil
}

func (r *sqlRepository) GetSnapshotByID(ctx context.Context, id uuid.UUID) (*DefinitionSnapshot, error) {
	dbSnap, err := r.q.GetDefinitionSnapshotByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return mapSnapshot(dbSnap), nil
}

func (r *sqlRepository) GetActiveSessionForWord(ctx context.Context, userID uuid.UUID, headwordKey string) (*LearningSession, error) {
	dbSession, err := r.q.GetActiveSessionByWord(ctx, sqlc.GetActiveSessionByWordParams{
		UserID:      userID,
		HeadwordKey: headwordKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return mapSession(dbSession), nil
}

func (r *sqlRepository) CreateSession(ctx context.Context, userID uuid.UUID, wordText, headwordKey, contextNote string, snapshotID uuid.UUID, promptVersion, modelID string) (*LearningSession, error) {
	dbSession, err := r.q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:      userID,
		WordText:    wordText,
		HeadwordKey: headwordKey,
		Status:      "active",
		ContextNote: sql.NullString{
			String: contextNote,
			Valid:  contextNote != "",
		},
		SnapshotID: uuid.NullUUID{
			UUID:  snapshotID,
			Valid: snapshotID != uuid.Nil,
		},
		PromptVersion: promptVersion,
		ModelID: sql.NullString{
			String: modelID,
			Valid:  modelID != "",
		},
	})
	if err != nil {
		return nil, err
	}
	return mapSession(dbSession), nil
}

func (r *sqlRepository) GetExistingEntriesByWord(ctx context.Context, userID uuid.UUID, headwordKey string) ([]VocabEntryRecord, error) {
	entries, err := r.q.GetVocabEntriesByWord(ctx, sqlc.GetVocabEntriesByWordParams{
		UserID:      userID,
		HeadwordKey: headwordKey,
	})
	if err != nil {
		return nil, err
	}

	result := make([]VocabEntryRecord, 0, len(entries))
	for _, e := range entries {
		result = append(result, VocabEntryRecord{
			ID:                 e.ID,
			UserID:             e.UserID,
			WordText:           e.WordText,
			HeadwordKey:        e.HeadwordKey,
			SenseKey:           e.SenseKey,
			OfficialDefinition: e.OfficialDefinition,
			PersonalDefinition: e.PersonalDefinition,
			CreatedAt:          e.CreatedAt,
		})
	}
	return result, nil
}
