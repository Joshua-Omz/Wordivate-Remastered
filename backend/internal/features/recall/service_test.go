// Purpose: Unit tests for adaptive recall SM-2 calculations, repository orchestration, and multi-tenant security.
// Architecture: backend/internal/features/recall/service_test.go
package recall

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockRecallRepo implements Repository for isolated unit tests.
type mockRecallRepo struct {
	ensureSchedulesFunc   func(ctx context.Context, userID uuid.UUID) error
	getDueEntriesFunc     func(ctx context.Context, userID uuid.UUID, nextReviewBefore time.Time, limit int32) ([]DueWordItem, error)
	getScheduleFunc       func(ctx context.Context, entryID, userID uuid.UUID) (*RecallSchedule, error)
	saveReviewFunc        func(ctx context.Context, params SaveReviewParams) (*ReviewResult, error)
	verifyOwnershipFunc   func(ctx context.Context, entryID, userID uuid.UUID) (bool, error)
}

func (m *mockRecallRepo) EnsureSchedulesForUser(ctx context.Context, userID uuid.UUID) error {
	if m.ensureSchedulesFunc != nil {
		return m.ensureSchedulesFunc(ctx, userID)
	}
	return nil
}

func (m *mockRecallRepo) GetDueEntries(ctx context.Context, userID uuid.UUID, nextReviewBefore time.Time, limit int32) ([]DueWordItem, error) {
	if m.getDueEntriesFunc != nil {
		return m.getDueEntriesFunc(ctx, userID, nextReviewBefore, limit)
	}
	return nil, nil
}

func (m *mockRecallRepo) GetSchedule(ctx context.Context, entryID, userID uuid.UUID) (*RecallSchedule, error) {
	if m.getScheduleFunc != nil {
		return m.getScheduleFunc(ctx, entryID, userID)
	}
	return nil, nil
}

func (m *mockRecallRepo) SaveReviewAndSchedule(ctx context.Context, params SaveReviewParams) (*ReviewResult, error) {
	if m.saveReviewFunc != nil {
		return m.saveReviewFunc(ctx, params)
	}
	return &ReviewResult{
		EntryID:         params.EntryID,
		RepetitionCount: params.RepetitionCount,
		IntervalDays:    params.IntervalDays,
		EaseFactor:      params.EaseFactor,
		NextReviewAt:    params.NextReviewAt,
		ReviewedAt:      params.ReviewedAt,
	}, nil
}

func (m *mockRecallRepo) VerifyEntryOwnership(ctx context.Context, entryID, userID uuid.UUID) (bool, error) {
	if m.verifyOwnershipFunc != nil {
		return m.verifyOwnershipFunc(ctx, entryID, userID)
	}
	return true, nil
}

// TestSM2_CalculateNextReview verifies interval expansion, ease factor clamping, and repetition tracking.
func TestSM2_CalculateNextReview(t *testing.T) {
	svc := NewService(&mockRecallRepo{})

	tests := []struct {
		name          string
		interval      int
		easeFactor    float64
		repetitions   int
		rating        int
		wantInterval  int
		wantEaseFloor float64
		wantReps      int
	}{
		{
			name:          "First successful review (rating 4)",
			interval:      1,
			easeFactor:    2.5,
			repetitions:   0,
			rating:        4,
			wantInterval:  1,
			wantEaseFloor: 2.50,
			wantReps:      1,
		},
		{
			name:          "Second successful review (rating 5)",
			interval:      1,
			easeFactor:    2.5,
			repetitions:   1,
			rating:        5,
			wantInterval:  6,
			wantEaseFloor: 2.60,
			wantReps:      2,
		},
		{
			name:          "Third successful review (rating 4)",
			interval:      6,
			easeFactor:    2.6,
			repetitions:   2,
			rating:        4,
			wantInterval:  16, // round(6 * 2.60) = 16
			wantEaseFloor: 2.60,
			wantReps:      3,
		},
		{
			name:          "Failed recall resets interval and repetitions (rating 1)",
			interval:      16,
			easeFactor:    2.6,
			repetitions:   3,
			rating:        1,
			wantInterval:  1,
			wantEaseFloor: 2.06, // 2.6 + (0.1 - (4)*(0.08 + 4*0.02)) = 2.6 + (0.1 - 0.64) = 2.06
			wantReps:      0,
		},
		{
			name:          "Ease factor clamped at 1.3 minimum",
			interval:      5,
			easeFactor:    1.3,
			repetitions:   2,
			rating:        0, // severe penalty
			wantInterval:  1,
			wantEaseFloor: 1.3,
			wantReps:      0,
		},
		{
			name:          "Negative rating clamped to 0",
			interval:      6,
			easeFactor:    2.5,
			repetitions:   2,
			rating:        -2,
			wantInterval:  1,
			wantEaseFloor: 1.70, // 2.5 + (0.1 - (5)*(0.08 + 5*0.02)) = 2.5 + (0.1 - 0.90) = 1.70
			wantReps:      0,
		},
		{
			name:          "Rating > 5 clamped to 5",
			interval:      1,
			easeFactor:    2.5,
			repetitions:   0,
			rating:        10,
			wantInterval:  1,
			wantEaseFloor: 2.60,
			wantReps:      1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotInterval, gotEase, gotReps, nextReview := svc.CalculateNextReview(
				tc.interval,
				tc.easeFactor,
				tc.repetitions,
				tc.rating,
			)

			if gotInterval != tc.wantInterval {
				t.Errorf("got interval %d, want %d", gotInterval, tc.wantInterval)
			}
			if gotEase < tc.wantEaseFloor-0.01 || gotEase > tc.wantEaseFloor+0.01 {
				t.Errorf("got ease factor %.2f, want %.2f", gotEase, tc.wantEaseFloor)
			}
			if gotReps != tc.wantReps {
				t.Errorf("got repetitions %d, want %d", gotReps, tc.wantReps)
			}
			if nextReview.Before(time.Now()) {
				t.Errorf("nextReview %v should be scheduled in the future", nextReview)
			}
		})
	}
}

// TestService_GetDueWords verifies retrieval and lazy initialization.
func TestService_GetDueWords(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	wordID := uuid.New()

	t.Run("Successfully returns due words with seeding", func(t *testing.T) {
		seeded := false
		mock := &mockRecallRepo{
			ensureSchedulesFunc: func(ctx context.Context, u uuid.UUID) error {
				if u != userID {
					t.Fatalf("unexpected user id: %v", u)
				}
				seeded = true
				return nil
			},
			getDueEntriesFunc: func(ctx context.Context, u uuid.UUID, before time.Time, limit int32) ([]DueWordItem, error) {
				return []DueWordItem{
					{
						ID:                 wordID,
						Term:               "ephemeral",
						OfficialDefinition: "lasting for a very short time",
						PersonalDefinition: "something that vanishes quickly like dew",
						IntervalDays:       1,
						EaseFactor:         2.5,
						RepetitionCount:    0,
						NextReview:         before,
					},
				}, nil
			},
		}

		svc := NewService(mock)
		res, err := svc.GetDueWords(ctx, userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !seeded {
			t.Errorf("expected EnsureSchedulesForUser to be called")
		}
		if res.Count != 1 || len(res.Words) != 1 {
			t.Errorf("expected 1 due word, got count %d", res.Count)
		}
		if res.Words[0].Term != "ephemeral" {
			t.Errorf("got term %s, want ephemeral", res.Words[0].Term)
		}
	})

	t.Run("Rejects nil user id", func(t *testing.T) {
		svc := NewService(&mockRecallRepo{})
		_, err := svc.GetDueWords(ctx, uuid.Nil)
		if !errors.Is(err, ErrUnauthorizedUser) {
			t.Errorf("expected ErrUnauthorizedUser, got %v", err)
		}
	})

	t.Run("Propagates database seeding error", func(t *testing.T) {
		dbErr := errors.New("db failure")
		mock := &mockRecallRepo{
			ensureSchedulesFunc: func(ctx context.Context, u uuid.UUID) error {
				return dbErr
			},
		}
		svc := NewService(mock)
		_, err := svc.GetDueWords(ctx, userID)
		if !errors.Is(err, dbErr) {
			t.Errorf("expected dbErr, got %v", err)
		}
	})
}

// TestService_SubmitReview tests validation, ownership checks, and review processing.
func TestService_SubmitReview(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	wordID := uuid.New()

	t.Run("Rejects invalid rating", func(t *testing.T) {
		svc := NewService(&mockRecallRepo{})
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 6,
		})
		if !errors.Is(err, ErrInvalidRating) {
			t.Errorf("expected ErrInvalidRating, got %v", err)
		}
	})

	t.Run("Rejects invalid word ID", func(t *testing.T) {
		svc := NewService(&mockRecallRepo{})
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: "not-a-uuid",
			Rating: 4,
		})
		if !errors.Is(err, ErrInvalidWordID) {
			t.Errorf("expected ErrInvalidWordID, got %v", err)
		}
	})

	t.Run("Rejects unowned entry", func(t *testing.T) {
		mock := &mockRecallRepo{
			verifyOwnershipFunc: func(ctx context.Context, e, u uuid.UUID) (bool, error) {
				return false, nil
			},
		}
		svc := NewService(mock)
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 4,
		})
		if !errors.Is(err, ErrEntryNotFound) {
			t.Errorf("expected ErrEntryNotFound, got %v", err)
		}
	})

	t.Run("Processes initial review when no previous schedule exists", func(t *testing.T) {
		var savedParams SaveReviewParams
		mock := &mockRecallRepo{
			verifyOwnershipFunc: func(ctx context.Context, e, u uuid.UUID) (bool, error) {
				return true, nil
			},
			getScheduleFunc: func(ctx context.Context, e, u uuid.UUID) (*RecallSchedule, error) {
				return nil, nil // No previous schedule
			},
			saveReviewFunc: func(ctx context.Context, params SaveReviewParams) (*ReviewResult, error) {
				savedParams = params
				return &ReviewResult{
					EntryID:         params.EntryID,
					RepetitionCount: params.RepetitionCount,
					IntervalDays:    params.IntervalDays,
					EaseFactor:      params.EaseFactor,
					NextReviewAt:    params.NextReviewAt,
					ReviewedAt:      params.ReviewedAt,
				}, nil
			},
		}

		svc := NewService(mock)
		res, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 4,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if savedParams.RepetitionCount != 1 {
			t.Errorf("expected repetitions 1, got %d", savedParams.RepetitionCount)
		}
		if savedParams.IntervalDays != 1 {
			t.Errorf("expected interval 1, got %d", savedParams.IntervalDays)
		}
		if res.EntryID != wordID {
			t.Errorf("got entry id %v, want %v", res.EntryID, wordID)
		}
	})

	t.Run("Processes review for existing schedule with interval expansion", func(t *testing.T) {
		mock := &mockRecallRepo{
			verifyOwnershipFunc: func(ctx context.Context, e, u uuid.UUID) (bool, error) {
				return true, nil
			},
			getScheduleFunc: func(ctx context.Context, e, u uuid.UUID) (*RecallSchedule, error) {
				return &RecallSchedule{
					EntryID:         wordID,
					UserID:          userID,
					IntervalDays:    1,
					EaseFactor:      2.5,
					RepetitionCount: 1,
				}, nil
			},
			saveReviewFunc: func(ctx context.Context, params SaveReviewParams) (*ReviewResult, error) {
				return &ReviewResult{
					EntryID:         params.EntryID,
					RepetitionCount: params.RepetitionCount,
					IntervalDays:    params.IntervalDays,
					EaseFactor:      params.EaseFactor,
					NextReviewAt:    params.NextReviewAt,
					ReviewedAt:      params.ReviewedAt,
				}, nil
			},
		}

		svc := NewService(mock)
		res, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Repetition 1 -> 2 has interval 6
		if res.RepetitionCount != 2 {
			t.Errorf("expected repetitions 2, got %d", res.RepetitionCount)
		}
		if res.IntervalDays != 6 {
			t.Errorf("expected interval 6, got %d", res.IntervalDays)
		}
	})

	t.Run("Rejects nil user ID", func(t *testing.T) {
		svc := NewService(&mockRecallRepo{})
		_, err := svc.SubmitReview(ctx, uuid.Nil, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 4,
		})
		if !errors.Is(err, ErrUnauthorizedUser) {
			t.Errorf("expected ErrUnauthorizedUser, got %v", err)
		}
	})

	t.Run("Rejects negative rating", func(t *testing.T) {
		svc := NewService(&mockRecallRepo{})
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: -1,
		})
		if !errors.Is(err, ErrInvalidRating) {
			t.Errorf("expected ErrInvalidRating, got %v", err)
		}
	})

	t.Run("Propagates ownership verification database error", func(t *testing.T) {
		expectedErr := errors.New("ownership query error")
		mock := &mockRecallRepo{
			verifyOwnershipFunc: func(ctx context.Context, e, u uuid.UUID) (bool, error) {
				return false, expectedErr
			},
		}
		svc := NewService(mock)
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 4,
		})
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("Propagates schedule retrieval error", func(t *testing.T) {
		expectedErr := errors.New("schedule query error")
		mock := &mockRecallRepo{
			verifyOwnershipFunc: func(ctx context.Context, e, u uuid.UUID) (bool, error) {
				return true, nil
			},
			getScheduleFunc: func(ctx context.Context, e, u uuid.UUID) (*RecallSchedule, error) {
				return nil, expectedErr
			},
		}
		svc := NewService(mock)
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 4,
		})
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})

	t.Run("Propagates save review database error", func(t *testing.T) {
		expectedErr := errors.New("save review error")
		mock := &mockRecallRepo{
			verifyOwnershipFunc: func(ctx context.Context, e, u uuid.UUID) (bool, error) {
				return true, nil
			},
			saveReviewFunc: func(ctx context.Context, params SaveReviewParams) (*ReviewResult, error) {
				return nil, expectedErr
			},
		}
		svc := NewService(mock)
		_, err := svc.SubmitReview(ctx, userID, ReviewSubmission{
			WordID: wordID.String(),
			Rating: 4,
		})
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
	})
}

func TestRepository_Constructor(t *testing.T) {
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("expected NewRepository to return non-nil")
	}
}

