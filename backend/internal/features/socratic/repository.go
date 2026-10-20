// Purpose: Persistence layer for Socratic sessions, append-only messages, and definition snapshots.
// Architecture: backend/internal/features/socratic/repository.go
package socratic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/database/sqlc"
)

/// SnapshotRecord holds immutable lexical definition snapshot data for grounding Socratic prompts.
type SnapshotRecord struct {
	ID          uuid.UUID
	HeadwordKey string
	Source      string
	SourceRef   string
	Payload     []byte
	ContentHash []byte
	RetrievedAt time.Time
}

/// SocraticRepository defines the persistence contract required by the Socratic conversation engine.
type SocraticRepository interface {
	GetSessionByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Session, error)
	GetActiveSessionByWord(ctx context.Context, userID uuid.UUID, headwordKey string) (*Session, error)
	AbandonSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Session, error)
	ResumeSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Session, error)
	UpdateSessionStatus(ctx context.Context, id uuid.UUID, userID uuid.UUID, status SessionStatus) (*Session, error)
	UpdateSessionLastActive(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	CreateMessage(ctx context.Context, sessionID uuid.UUID, seq int32, role Role, content string, tokenCount *int32) (*Message, error)
	GetLastMessageSeq(ctx context.Context, sessionID uuid.UUID) (int32, error)
	ListMessagesBySession(ctx context.Context, sessionID uuid.UUID) ([]Message, error)
	GetSnapshotByID(ctx context.Context, id uuid.UUID) (*SnapshotRecord, error)
	AbandonInactiveSessions(ctx context.Context, cutoff time.Time) (int64, error)
}

type sqlRepository struct {
	q *sqlc.Queries
}

/// NewRepository instantiates a concrete SocraticRepository backed by sqlc and PostgreSQL.
func NewRepository(db *sql.DB) SocraticRepository {
	return &sqlRepository{
		q: sqlc.New(db),
	}
}

func mapSession(s sqlc.LearningSession) *Session {
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

	return &Session{
		ID:            s.ID,
		UserID:        s.UserID,
		WordText:      s.WordText,
		HeadwordKey:   s.HeadwordKey,
		Status:        SessionStatus(s.Status),
		ContextNote:   note,
		SnapshotID:    snapshotID,
		PromptVersion: s.PromptVersion,
		ModelID:       model,
		CreatedAt:     s.StartedAt,
		LastActiveAt:  s.LastActiveAt,
		EndedAt:       ended,
	}
}

func mapMessage(m sqlc.Message) Message {
	var tokens *int32
	if m.TokenCount.Valid {
		val := m.TokenCount.Int32
		tokens = &val
	}

	return Message{
		ID:         m.ID,
		SessionID:  m.SessionID,
		Seq:        m.Seq,
		Role:       Role(m.Role),
		Content:    m.Content,
		TokenCount: tokens,
		CreatedAt:  m.CreatedAt,
	}
}

func (r *sqlRepository) GetSessionByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Session, error) {
	s, err := r.q.GetSessionByID(ctx, sqlc.GetSessionByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("getting session by id: %w", err)
	}
	return mapSession(s), nil
}

func (r *sqlRepository) UpdateSessionStatus(ctx context.Context, id uuid.UUID, userID uuid.UUID, status SessionStatus) (*Session, error) {
	s, err := r.q.UpdateSessionStatus(ctx, sqlc.UpdateSessionStatusParams{
		ID:     id,
		Status: string(status),
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("updating session status: %w", err)
	}
	return mapSession(s), nil
}

func (r *sqlRepository) UpdateSessionLastActive(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := r.q.UpdateSessionLastActive(ctx, sqlc.UpdateSessionLastActiveParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("updating session last active: %w", err)
	}
	return nil
}

func (r *sqlRepository) CreateMessage(ctx context.Context, sessionID uuid.UUID, seq int32, role Role, content string, tokenCount *int32) (*Message, error) {
	var sqlTokens sql.NullInt32
	if tokenCount != nil {
		sqlTokens = sql.NullInt32{
			Int32: *tokenCount,
			Valid: true,
		}
	}

	m, err := r.q.CreateMessage(ctx, sqlc.CreateMessageParams{
		SessionID:  sessionID,
		Seq:        seq,
		Role:       string(role),
		Content:    content,
		TokenCount: sqlTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("persisting message (seq %d): %w", seq, err)
	}

	mapped := mapMessage(m)
	return &mapped, nil
}

func (r *sqlRepository) GetLastMessageSeq(ctx context.Context, sessionID uuid.UUID) (int32, error) {
	seq, err := r.q.GetLastMessageSeq(ctx, sessionID)
	if err != nil {
		return 0, fmt.Errorf("getting last message seq for session: %w", err)
	}
	return seq, nil
}

func (r *sqlRepository) ListMessagesBySession(ctx context.Context, sessionID uuid.UUID) ([]Message, error) {
	dbMsgs, err := r.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing messages for session: %w", err)
	}

	messages := make([]Message, 0, len(dbMsgs))
	for _, m := range dbMsgs {
		messages = append(messages, mapMessage(m))
	}
	return messages, nil
}

func (r *sqlRepository) GetSnapshotByID(ctx context.Context, id uuid.UUID) (*SnapshotRecord, error) {
	s, err := r.q.GetDefinitionSnapshotByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("getting snapshot by id: %w", err)
	}

	var sourceRef string
	if s.SourceRef.Valid {
		sourceRef = s.SourceRef.String
	}

	return &SnapshotRecord{
		ID:          s.ID,
		HeadwordKey: s.HeadwordKey,
		Source:      s.Source,
		SourceRef:   sourceRef,
		Payload:     s.Payload,
		ContentHash: s.ContentHash,
		RetrievedAt: s.RetrievedAt,
	}, nil
}

func (r *sqlRepository) AbandonInactiveSessions(ctx context.Context, cutoff time.Time) (int64, error) {
	rows, err := r.q.AbandonInactiveSessions(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("abandoning inactive sessions: %w", err)
	}
	return rows, nil
}

func (r *sqlRepository) GetActiveSessionByWord(ctx context.Context, userID uuid.UUID, headwordKey string) (*Session, error) {
	row, err := r.q.GetActiveSessionByWord(ctx, sqlc.GetActiveSessionByWordParams{
		UserID:      userID,
		HeadwordKey: headwordKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting active session by word: %w", err)
	}
	return mapSession(row), nil
}

func (r *sqlRepository) AbandonSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Session, error) {
	row, err := r.q.AbandonSession(ctx, sqlc.AbandonSessionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return mapSession(row), nil
}

func (r *sqlRepository) ResumeSession(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Session, error) {
	row, err := r.q.ResumeSession(ctx, sqlc.ResumeSessionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return mapSession(row), nil
}

