// Purpose: Domain service for SuperMemo-2 adaptive recall scheduling and review evaluations.
// Architecture: backend/internal/features/recall/service.go
package recall

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
)

// Default limits and parameters for SM-2 scheduling.
const (
	DefaultLimit      int32   = 100
	DefaultEaseFactor float64 = 2.5
	MinEaseFactor     float64 = 1.3
)

// Service defines adaptive recall / SRS operations.
type Service interface {
	GetDueWords(ctx context.Context, userID uuid.UUID) (*DueWordsResponse, error)
	SubmitReview(ctx context.Context, userID uuid.UUID, sub ReviewSubmission) (*ReviewResult, error)
	CalculateNextReview(interval int, easeFactor float64, repetitions int, rating int) (nextInterval int, newEaseFactor float64, newRepetitions int, nextReview time.Time)
}

type service struct {
	repo Repository
}

// NewService instantiates a recall domain service with an injected repository.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// GetDueWords ensures schedules are initialized for all user entries and returns due words.
func (s *service) GetDueWords(ctx context.Context, userID uuid.UUID) (*DueWordsResponse, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthorizedUser
	}

	// 1. Lazy seeding: Guarantee unindexed committed words have active recall schedules
	if err := s.repo.EnsureSchedulesForUser(ctx, userID); err != nil {
		return nil, err
	}

	// 2. Fetch words due for review up to now
	now := time.Now()
	items, err := s.repo.GetDueEntries(ctx, userID, now, DefaultLimit)
	if err != nil {
		return nil, err
	}

	return &DueWordsResponse{
		Words: items,
		Count: len(items),
	}, nil
}

// SubmitReview evaluates a recall attempt, updates the entry's schedule, and logs review history.
func (s *service) SubmitReview(ctx context.Context, userID uuid.UUID, sub ReviewSubmission) (*ReviewResult, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthorizedUser
	}

	// 1. Validate rating range (0 to 5)
	if sub.Rating < 0 || sub.Rating > 5 {
		return nil, ErrInvalidRating
	}

	// 2. Validate word ID
	entryID, err := uuid.Parse(sub.WordID)
	if err != nil || entryID == uuid.Nil {
		return nil, ErrInvalidWordID
	}

	// 3. Verify user ownership of the vocabulary entry
	owned, err := s.repo.VerifyEntryOwnership(ctx, entryID, userID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, ErrEntryNotFound
	}

	// 4. Fetch current schedule (or fallback to defaults if not yet created)
	sched, err := s.repo.GetSchedule(ctx, entryID, userID)
	if err != nil {
		return nil, err
	}

	currentInterval := 1
	currentEase := DefaultEaseFactor
	currentReps := 0

	if sched != nil {
		currentInterval = sched.IntervalDays
		currentEase = sched.EaseFactor
		currentReps = sched.RepetitionCount
	}

	// 5. Compute next review schedule using SM-2
	nextInterval, newEase, newReps, nextReview := s.CalculateNextReview(currentInterval, currentEase, currentReps, sub.Rating)

	// 6. Atomically persist schedule and review log
	now := time.Now()
	result, err := s.repo.SaveReviewAndSchedule(ctx, SaveReviewParams{
		EntryID:         entryID,
		UserID:          userID,
		Rating:          sub.Rating,
		IntervalDays:    nextInterval,
		EaseFactor:      newEase,
		RepetitionCount: newReps,
		NextReviewAt:    nextReview,
		ReviewedAt:      now,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// CalculateNextReview implements the SuperMemo-2 (SM-2) spaced repetition algorithm:
//
// EF' = EF + (0.1 - (5 - q) * (0.08 + (5 - q) * 0.02)) clamped at >= 1.3
// If rating < 3 (failure): repetitions reset to 0, interval resets to 1 day.
// If rating >= 3 (success):
//   - rep 1: interval = 1 day
//   - rep 2: interval = 6 days
//   - rep n > 2: interval = round(previous_interval * EF')
func (s *service) CalculateNextReview(interval int, easeFactor float64, repetitions int, rating int) (int, float64, int, time.Time) {
	// Clamp rating between 0 and 5
	if rating < 0 {
		rating = 0
	} else if rating > 5 {
		rating = 5
	}

	// Ensure ease factor has valid floor
	if easeFactor < MinEaseFactor {
		easeFactor = MinEaseFactor
	}

	// Update Ease Factor
	q := float64(rating)
	newEase := easeFactor + (0.1 - (5.0-q)*(0.08+(5.0-q)*0.02))
	if newEase < MinEaseFactor {
		newEase = MinEaseFactor
	}
	// Round ease factor to 2 decimal places for clean storage
	newEase = math.Round(newEase*100) / 100

	var nextInterval int
	var newReps int

	if rating < 3 {
		// Recall failure: Reset repetition counter and interval
		newReps = 0
		nextInterval = 1
	} else {
		// Recall success: Increment repetition count
		newReps = repetitions + 1
		switch newReps {
		case 1:
			nextInterval = 1
		case 2:
			nextInterval = 6
		default:
			calc := float64(interval) * newEase
			nextInterval = int(math.Round(calc))
			if nextInterval <= interval {
				nextInterval = interval + 1
			}
		}
	}

	nextReview := time.Now().AddDate(0, 0, nextInterval)
	return nextInterval, newEase, newReps, nextReview
}
