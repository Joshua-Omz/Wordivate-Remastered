// Purpose: Unit and HTTP integration tests for Socratic SSE handler and session history.
// Architecture: backend/internal/features/socratic/handler_test.go
package socratic_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/socratic"
	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

func setupTestRouter(h *socratic.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/v1/sessions", func(r chi.Router) {
		r.Get("/{id}", h.GetSession)
		r.Post("/{id}/messages", h.PostMessage)
		r.Post("/{id}/abandon", h.AbandonSession)
		r.Post("/{id}/resume", h.ResumeSession)
	})
	return r
}

func TestHandler_PostMessage_SSEStream(t *testing.T) {
	repo := newMockRepository()
	llm := socratic.NewMockLLMClient(0)
	svc := socratic.NewService(repo, llm)
	hdl := socratic.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:           sessionID,
		UserID:       userID,
		WordText:     "ubiquitous",
		Status:       socratic.SessionStatusActive,
		LastActiveAt: time.Now(),
	}

	body := bytes.NewBufferString(`{"content": "I notice smartphones are ubiquitous today."}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	// Inject authenticated user into context
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("expected text/event-stream, got %s", contentType)
	}

	responseBody := rec.Body.String()
	if !strings.Contains(responseBody, "event: token") {
		t.Errorf("expected SSE token events in body, got: %s", responseBody)
	}
	if !strings.Contains(responseBody, "event: done") {
		t.Errorf("expected SSE done event in body, got: %s", responseBody)
	}
}

func TestHandler_PostMessage_ValidationErrors(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))
	hdl := socratic.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "ubiquitous",
		Status:   socratic.SessionStatusActive,
	}

	// 1. Invalid UUID
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/invalid-uuid/messages", bytes.NewBufferString(`{"content":"test"}`))
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for invalid UUID, got %d", rec.Code)
	}

	// 2. Empty Content
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/messages", bytes.NewBufferString(`{"content":"  "}`))
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for empty content, got %d", rec.Code)
	}

	// 3. Unauthenticated (no user ID in context)
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/messages", bytes.NewBufferString(`{"content":"hello"}`))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected HTTP 401 for missing auth, got %d", rec.Code)
	}
}

func TestHandler_GetSession_SuccessAndNotFound(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))
	hdl := socratic.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:       sessionID,
		UserID:   userID,
		WordText: "serendipity",
		Status:   socratic.SessionStatusActive,
	}
	_, _ = repo.CreateMessage(context.Background(), sessionID, 1, socratic.RoleUser, "What does it mean?", nil)

	// Success case
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID.String(), nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}

	var res socratic.SessionHistoryResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Session.WordText != "serendipity" || len(res.Messages) != 1 {
		t.Errorf("unexpected history content: %+v", res)
	}

	// Not Found case
	randomID := uuid.New()
	req = httptest.NewRequest(http.MethodGet, "/v1/sessions/"+randomID.String(), nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected HTTP 404 for missing session, got %d", rec.Code)
	}
}

func TestHandler_AbandonSession(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))
	hdl := socratic.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:          sessionID,
		UserID:      userID,
		WordText:    "ephemeral",
		HeadwordKey: "ephemeral",
		Status:      socratic.SessionStatusActive,
	}

	t.Run("HTTP 200 OK on abandoning active session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/abandon", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var s socratic.Session
		if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if s.Status != socratic.SessionStatusAbandoned {
			t.Errorf("got status %v, want abandoned", s.Status)
		}
	})

	t.Run("HTTP 409 Conflict when session already abandoned", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/abandon", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d", rec.Code)
		}
	})

	t.Run("HTTP 401 Unauthorized when missing user context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/abandon", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected HTTP 401, got %d", rec.Code)
		}
	})

	t.Run("HTTP 404 Not Found when session does not exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+uuid.New().String()+"/abandon", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected HTTP 404, got %d", rec.Code)
		}
	})
}

func TestHandler_ResumeSession(t *testing.T) {
	repo := newMockRepository()
	svc := socratic.NewService(repo, socratic.NewMockLLMClient(0))
	hdl := socratic.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &socratic.Session{
		ID:          sessionID,
		UserID:      userID,
		WordText:    "ephemeral",
		HeadwordKey: "ephemeral",
		Status:      socratic.SessionStatusAbandoned,
	}

	t.Run("HTTP 200 OK on resuming abandoned session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/resume", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var s socratic.Session
		if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if s.Status != socratic.SessionStatusActive {
			t.Errorf("got status %v, want active", s.Status)
		}
	})

	t.Run("HTTP 409 Conflict when resuming already active session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/resume", nil)
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d", rec.Code)
		}
	})

	t.Run("HTTP 401 Unauthorized when missing user context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/resume", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected HTTP 401, got %d", rec.Code)
		}
	})
}

