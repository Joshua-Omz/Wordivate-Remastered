// Purpose: HTTP transport integration tests for recall endpoints (/v1/recall/due and /v1/recall/reviews).
// Architecture: backend/internal/features/recall/handler_test.go
package recall_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/recall"
	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

type mockRecallService struct {
	getDueWordsFunc func(ctx context.Context, userID uuid.UUID) (*recall.DueWordsResponse, error)
	submitReviewFunc func(ctx context.Context, userID uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error)
}

func (m *mockRecallService) GetDueWords(ctx context.Context, userID uuid.UUID) (*recall.DueWordsResponse, error) {
	if m.getDueWordsFunc != nil {
		return m.getDueWordsFunc(ctx, userID)
	}
	return &recall.DueWordsResponse{Words: []recall.DueWordItem{}, Count: 0}, nil
}

func (m *mockRecallService) SubmitReview(ctx context.Context, userID uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error) {
	if m.submitReviewFunc != nil {
		return m.submitReviewFunc(ctx, userID, sub)
	}
	entryID, _ := uuid.Parse(sub.WordID)
	return &recall.ReviewResult{
		EntryID:         entryID,
		RepetitionCount: 1,
		IntervalDays:    1,
		EaseFactor:      2.5,
		NextReviewAt:    time.Now().AddDate(0, 0, 1),
		ReviewedAt:      time.Now(),
	}, nil
}

func (m *mockRecallService) CalculateNextReview(interval int, easeFactor float64, repetitions int, rating int) (int, float64, int, time.Time) {
	return 1, 2.5, 1, time.Now().AddDate(0, 0, 1)
}

func setupTestRouter(h *recall.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Get("/recall/due", h.GetDue)
		r.Post("/recall/reviews", h.SubmitReview)
	})
	return r
}

func TestHandler_GetDue(t *testing.T) {
	userID := uuid.New()
	wordID := uuid.New()

	t.Run("HTTP 200 OK on authenticated valid request", func(t *testing.T) {
		svc := &mockRecallService{
			getDueWordsFunc: func(ctx context.Context, u uuid.UUID) (*recall.DueWordsResponse, error) {
				return &recall.DueWordsResponse{
					Words: []recall.DueWordItem{
						{
							ID:                 wordID,
							Term:               "solitude",
							OfficialDefinition: "the state of being alone",
							PersonalDefinition: "peaceful alone time to recharge",
							IntervalDays:       1,
							EaseFactor:         2.5,
						},
					},
					Count: 1,
				}, nil
			},
		}

		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/v1/recall/due", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp recall.DueWordsResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Count != 1 || len(resp.Words) != 1 {
			t.Fatalf("expected 1 due word, got %d", resp.Count)
		}
		if resp.Words[0].Term != "solitude" {
			t.Errorf("got term %s, want solitude", resp.Words[0].Term)
		}
	})

	t.Run("HTTP 401 Unauthorized when user context is missing", func(t *testing.T) {
		svc := &mockRecallService{}
		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/v1/recall/due", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected HTTP 401, got %d", rec.Code)
		}
	})

	t.Run("HTTP 500 Internal Server Error when service fails", func(t *testing.T) {
		svc := &mockRecallService{
			getDueWordsFunc: func(ctx context.Context, u uuid.UUID) (*recall.DueWordsResponse, error) {
				return nil, errors.New("database connection failed")
			},
		}
		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodGet, "/v1/recall/due", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected HTTP 500, got %d", rec.Code)
		}
	})
}

func TestHandler_SubmitReview(t *testing.T) {
	userID := uuid.New()
	wordID := uuid.New()

	t.Run("HTTP 200 OK on successful review submission", func(t *testing.T) {
		svc := &mockRecallService{
			submitReviewFunc: func(ctx context.Context, u uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error) {
				return &recall.ReviewResult{
					EntryID:         wordID,
					RepetitionCount: 2,
					IntervalDays:    6,
					EaseFactor:      2.6,
					NextReviewAt:    time.Now().AddDate(0, 0, 6),
					ReviewedAt:      time.Now(),
				}, nil
			},
		}

		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		body := map[string]interface{}{
			"word_id": wordID.String(),
			"rating":  5,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader(bodyBytes))
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var res recall.ReviewResult
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.IntervalDays != 6 {
			t.Errorf("got interval %d, want 6", res.IntervalDays)
		}
	})

	t.Run("HTTP 400 Bad Request on invalid rating", func(t *testing.T) {
		svc := &mockRecallService{
			submitReviewFunc: func(ctx context.Context, u uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error) {
				return nil, recall.ErrInvalidRating
			},
		}

		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		body := map[string]interface{}{
			"word_id": wordID.String(),
			"rating":  7, // invalid rating
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader(bodyBytes))
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d", rec.Code)
		}
	})

	t.Run("HTTP 404 Not Found on unowned entry", func(t *testing.T) {
		svc := &mockRecallService{
			submitReviewFunc: func(ctx context.Context, u uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error) {
				return nil, recall.ErrEntryNotFound
			},
		}

		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		body := map[string]interface{}{
			"word_id": wordID.String(),
			"rating":  4,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader(bodyBytes))
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected HTTP 404, got %d", rec.Code)
		}
	})

	t.Run("HTTP 401 Unauthorized without auth context", func(t *testing.T) {
		svc := &mockRecallService{}
		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		body := map[string]interface{}{
			"word_id": wordID.String(),
			"rating":  4,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected HTTP 401, got %d", rec.Code)
		}
	})

	t.Run("HTTP 400 Bad Request on malformed JSON body", func(t *testing.T) {
		svc := &mockRecallService{}
		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader([]byte("invalid json")))
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected HTTP 400, got %d", rec.Code)
		}
	})

	t.Run("HTTP 401 Unauthorized when service returns ErrUnauthorizedUser", func(t *testing.T) {
		svc := &mockRecallService{
			submitReviewFunc: func(ctx context.Context, u uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error) {
				return nil, recall.ErrUnauthorizedUser
			},
		}

		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		body := map[string]interface{}{
			"word_id": wordID.String(),
			"rating":  4,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader(bodyBytes))
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected HTTP 401, got %d", rec.Code)
		}
	})

	t.Run("HTTP 500 Internal Server Error when service fails unexpectedly", func(t *testing.T) {
		svc := &mockRecallService{
			submitReviewFunc: func(ctx context.Context, u uuid.UUID, sub recall.ReviewSubmission) (*recall.ReviewResult, error) {
				return nil, errors.New("unexpected database error")
			},
		}

		h := recall.NewHandler(svc)
		router := setupTestRouter(h)

		body := map[string]interface{}{
			"word_id": wordID.String(),
			"rating":  4,
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/recall/reviews", bytes.NewReader(bodyBytes))
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected HTTP 500, got %d", rec.Code)
		}
	})
}

