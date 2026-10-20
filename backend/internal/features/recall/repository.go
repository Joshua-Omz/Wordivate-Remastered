// Purpose: Persistence layer interface and SQL implementation for SRS recall schedules and reviews.
// Architecture: backend/internal/features/recall/repository.go
package recall

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/database/sqlc"
)

// SaveReviewParams encapsulates the parameters for updating a schedule and logging a review.
type SaveReviewParams struct {
	EntryID         uuid.UUID
	UserID          uuid.UUID
	Rating          int
	IntervalDays    int
	EaseFactor      float64
	RepetitionCount int
	NextReviewAt    time.Time
	ReviewedAt      time.Time
}

// Repository defines the storage contract for the recall engine.
// Decouples domain logic from SQL drivers and enables unit testing with mocks.
type Repository interface {
	EnsureSchedulesForUser(ctx context.Context, userID uuid.UUID) error
	GetDueEntries(ctx context.Context, userID uuid.UUID, nextReviewBefore time.Time, limit int32) ([]DueWordItem, error)
	GetSchedule(ctx context.Context, entryID, userID uuid.UUID) (*RecallSchedule, error)
	SaveReviewAndSchedule(ctx context.Context, params SaveReviewParams) (*ReviewResult, error)
	VerifyEntryOwnership(ctx context.Context, entryID, userID uuid.UUID) (bool, error)
}

type sqlRepository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository instantiates a concrete Repository backed by PostgreSQL and SQLC.
func NewRepository(db *sql.DB) Repository {
	return &sqlRepository{
		db: db,
		q:  sqlc.New(db),
	}
}

// EnsureSchedulesForUser seeds recall schedules for any committed vocab_entries lacking one.
func (r *sqlRepository) EnsureSchedulesForUser(ctx context.Context, userID uuid.UUID) error {
	return r.q.EnsureSchedulesForUser(ctx, userID)
}

// GetDueEntries retrieves words due for review up to nextReviewBefore.
func (r *sqlRepository) GetDueEntries(ctx context.Context, userID uuid.UUID, nextReviewBefore time.Time, limit int32) ([]DueWordItem, error) {
	rows, err := r.q.GetDueRecallEntries(ctx, sqlc.GetDueRecallEntriesParams{
		UserID:       userID,
		NextReviewAt: nextReviewBefore,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}

	items := make([]DueWordItem, 0, len(rows))
	for _, row := range rows {
		var lastRev *time.Time
		if row.LastReviewedAt.Valid {
			t := row.LastReviewedAt.Time
			lastRev = &t
		}

		var pos, example, notes string
		if row.PartOfSpeech.Valid {
			pos = row.PartOfSpeech.String
		}
		if row.ExampleSentence.Valid {
			example = row.ExampleSentence.String
		}
		if row.ContextualNotes.Valid {
			notes = row.ContextualNotes.String
		}

		items = append(items, DueWordItem{
			ID:                 row.EntryID,
			Term:               row.WordText,
			Definition:         row.PersonalDefinition,
			OfficialDefinition: row.OfficialDefinition,
			PersonalDefinition: row.PersonalDefinition,
			PartOfSpeech:       pos,
			ExampleSentence:    example,
			ContextualNotes:    notes,
			UnderstandingFlag:  row.UnderstandingFlag,
			Interval:           int(row.IntervalDays),
			IntervalDays:       int(row.IntervalDays),
			EaseFactor:         row.EaseFactor,
			RepetitionCount:    int(row.RepetitionCount),
			NextReview:         row.NextReviewAt,
			LastReviewed:       lastRev,
		})
	}

	return items, nil
}

// GetSchedule returns the current recall schedule for a vocabulary entry.
// Returns (nil, nil) if no schedule exists yet.
func (r *sqlRepository) GetSchedule(ctx context.Context, entryID, userID uuid.UUID) (*RecallSchedule, error) {
	row, err := r.q.GetRecallSchedule(ctx, sqlc.GetRecallScheduleParams{
		EntryID: entryID,
		UserID:  userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var lastRev *time.Time
	if row.LastReviewedAt.Valid {
		t := row.LastReviewedAt.Time
		lastRev = &t
	}

	return &RecallSchedule{
		EntryID:         row.EntryID,
		UserID:          row.UserID,
		IntervalDays:    int(row.IntervalDays),
		EaseFactor:      row.EaseFactor,
		RepetitionCount: int(row.RepetitionCount),
		NextReviewAt:    row.NextReviewAt,
		LastReviewedAt:  lastRev,
		CreatedAt:       row.CreatedAt,
	}, nil
}

// VerifyEntryOwnership checks if the given entry exists and belongs to the user.
func (r *sqlRepository) VerifyEntryOwnership(ctx context.Context, entryID, userID uuid.UUID) (bool, error) {
	_, err := r.q.GetVocabEntryByID(ctx, sqlc.GetVocabEntryByIDParams{
		ID:     entryID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SaveReviewAndSchedule atomically updates the entry schedule and appends the review audit log.
func (r *sqlRepository) SaveReviewAndSchedule(ctx context.Context, params SaveReviewParams) (*ReviewResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := r.q.WithTx(tx)

	// 1. Upsert recall schedule
	sched, err := qtx.UpsertRecallSchedule(ctx, sqlc.UpsertRecallScheduleParams{
		EntryID:         params.EntryID,
		UserID:          params.UserID,
		IntervalDays:    int32(params.IntervalDays),
		EaseFactor:      params.EaseFactor,
		RepetitionCount: int32(params.RepetitionCount),
		NextReviewAt:    params.NextReviewAt,
		LastReviewedAt: sql.NullTime{
			Time:  params.ReviewedAt,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}

	// 2. Append review event audit
	_, err = qtx.CreateRecallReview(ctx, sqlc.CreateRecallReviewParams{
		EntryID:    params.EntryID,
		UserID:     params.UserID,
		Rating:     int32(params.Rating),
		ReviewedAt: params.ReviewedAt,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &ReviewResult{
		EntryID:         sched.EntryID,
		RepetitionCount: int(sched.RepetitionCount),
		IntervalDays:    int(sched.IntervalDays),
		EaseFactor:      sched.EaseFactor,
		NextReviewAt:    sched.NextReviewAt,
		ReviewedAt:      params.ReviewedAt,
	}, nil
}
