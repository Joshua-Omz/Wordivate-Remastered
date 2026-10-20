// Purpose: HTTP integration tests for commit-draft, commit, and library endpoints.
// Architecture: backend/internal/features/library/handler_test.go
package library_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/library"
	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

func setupTestRouter(h *library.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Post("/sessions/{id}/commit-draft", h.CommitDraft)
		r.Post("/sessions/{id}/commit", h.CommitSession)
		r.Get("/words", h.ListWords)
		r.Get("/words/{id}", h.GetWord)
		r.Patch("/words/{id}", h.UpdateWord)
		r.Delete("/words/{id}", h.DeleteWord)
	})
	return r
}

func TestHandler_CommitDraft_Success(t *testing.T) {
	repo := newMockRepository()
	ai := library.NewMockAssessmentClient()
	svc := library.NewService(repo, ai)
	hdl := library.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &library.SessionRecord{
		ID:        sessionID,
		UserID:    userID,
		WordText:  "ephemeral",
		Status:    "active",
		StartedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/commit-draft", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var draft library.CommitDraftResponse
	if err := json.NewDecoder(rec.Body).Decode(&draft); err != nil {
		t.Fatalf("decoding response failed: %v", err)
	}
	if draft.Verdict != "sufficient" {
		t.Errorf("expected sufficient verdict, got %s", draft.Verdict)
	}
}

func TestHandler_CommitSession_SuccessAndIdempotency(t *testing.T) {
	repo := newMockRepository()
	ai := library.NewMockAssessmentClient()
	svc := library.NewService(repo, ai)
	hdl := library.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	sessionID := uuid.New()

	repo.sessions[sessionID] = &library.SessionRecord{
		ID:        sessionID,
		UserID:    userID,
		WordText:  "ephemeral",
		Status:    "committing",
		StartedAt: time.Now(),
	}

	body := bytes.NewBufferString(`{
		"personal_definition": "Something temporary.",
		"personal_def_origin": "user_written",
		"understanding_flag": "ok"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/commit", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-key-001")
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	// Repeat request with same idempotency key must return HTTP 200 with IsDuplicate = true
	repeatBody := bytes.NewBufferString(`{
		"personal_definition": "Something temporary.",
		"personal_def_origin": "user_written",
		"understanding_flag": "ok"
	}`)
	repeatReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID.String()+"/commit", repeatBody)
	repeatReq.Header.Set("Content-Type", "application/json")
	repeatReq.Header.Set("Idempotency-Key", "test-key-001")
	repeatReq = repeatReq.WithContext(middleware.WithUserID(repeatReq.Context(), userID))

	repeatRec := httptest.NewRecorder()
	router.ServeHTTP(repeatRec, repeatReq)

	if repeatRec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for idempotency duplicate, got %d: %s", repeatRec.Code, repeatRec.Body.String())
	}

	var repeatResp library.CommitSessionResponse
	_ = json.NewDecoder(repeatRec.Body).Decode(&repeatResp)
	if !repeatResp.IsDuplicate {
		t.Errorf("expected IsDuplicate to be true on repeat request")
	}
}

func TestHandler_LibraryCRUD(t *testing.T) {
	repo := newMockRepository()
	svc := library.NewService(repo, library.NewMockAssessmentClient())
	hdl := library.NewHandler(svc)
	router := setupTestRouter(hdl)

	userID := uuid.New()
	entryID := uuid.New()

	repo.entries[entryID] = &library.VocabEntry{
		ID:                 entryID,
		UserID:             userID,
		WordText:           "ephemeral",
		OfficialDefinition: "Lasting a short time.",
		PersonalDefinition: "Fleeting like morning fog.",
		UnderstandingFlag:  library.UnderstandingFlagOK,
	}

	// 1. GET /v1/words
	req := httptest.NewRequest(http.MethodGet, "/v1/words", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on ListWords, got %d", rec.Code)
	}

	var list []library.VocabEntry
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("expected 1 entry in list, got %d", len(list))
	}

	// 2. GET /v1/words/{id}
	req = httptest.NewRequest(http.MethodGet, "/v1/words/"+entryID.String(), nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on GetWord, got %d", rec.Code)
	}

	// 3. PATCH /v1/words/{id}
	updateBody := bytes.NewBufferString(`{"personal_definition": "Updated definition."}`)
	req = httptest.NewRequest(http.MethodPatch, "/v1/words/"+entryID.String(), updateBody)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on UpdateWord, got %d", rec.Code)
	}

	// 4. DELETE /v1/words/{id}
	req = httptest.NewRequest(http.MethodDelete, "/v1/words/"+entryID.String(), nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on DeleteWord, got %d", rec.Code)
	}
}
