package recall

import (
	"context"
	"time"

	"github.com/Joshua-Omz/wordivate/backend/internal/database"
)

// Service defines adaptive recall / SRS operations.
type Service interface {
	GetDueWords(ctx context.Context, userID string) (*DueWordsResponse, error)
	CalculateNextReview(interval int, easeFactor float32, rating int) (nextInterval int, newEaseFactor float32, nextReview time.Time)
}

type service struct {
	db *database.DB
}

// NewService instantiates a new recall service.
func NewService(db *database.DB) Service {
	return &service{db: db}
}

func (s *service) GetDueWords(ctx context.Context, userID string) (*DueWordsResponse, error) {
	// TODO: Replace with query to database via sqlc (GetDueWords)
	due := []DueWordItem{
		{
			ID:         "00000000-0000-0000-0000-000000000002",
			Term:       "ephemeral",
			Definition: "lasting for a very short time",
			Interval:   1,
			EaseFactor: 2.5,
			NextReview: time.Now(),
		},
	}

	return &DueWordsResponse{
		Words: due,
		Count: len(due),
	}, nil
}

// CalculateNextReview implements the SuperMemo-2 (SM-2) adaptive spaced repetition algorithm.
func (s *service) CalculateNextReview(interval int, easeFactor float32, rating int) (int, float32, time.Time) {
	// Constrain rating 0 to 5
	if rating < 0 {
		rating = 0
	} else if rating > 5 {
		rating = 5
	}

	// Update Ease Factor (EF' = EF + (0.1 - (5 - q) * (0.08 + (5 - q) * 0.02)))
	q := float32(rating)
	newEase := easeFactor + (0.1 - (5.0-q)*(0.08+(5.0-q)*0.02))
	if newEase < 1.3 {
		newEase = 1.3
	}

	var nextInterval int
	if rating < 3 {
		// Failed recall, reset interval
		nextInterval = 1
	} else {
		switch interval {
		case 0:
			nextInterval = 1
		case 1:
			nextInterval = 6
		default:
			nextInterval = int(float32(interval) * newEase)
		}
	}

	nextReview := time.Now().AddDate(0, 0, nextInterval)
	return nextInterval, newEase, nextReview
}
