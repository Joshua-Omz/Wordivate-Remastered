// Purpose: HTTP transport handler for transactional commits, idempotency, and vocabulary library endpoints.
// Architecture: backend/internal/features/library/handler.go
package library

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

/// Handler exposes HTTP REST endpoints for the vocabulary library and commit flows.
type Handler struct {
	service Service
}

/// NewHandler instantiates a library HTTP transport handler.
func NewHandler(svc Service) *Handler {
	return &Handler{service: svc}
}

/// CommitDraft handles POST /v1/sessions/{id}/commit-draft returning AI synthesis for review.
func (h *Handler) CommitDraft(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil || sessionID == uuid.Nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid session id format")
		return
	}

	draft, err := h.service.GenerateCommitDraft(r.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrAlreadyCommitted) || errors.Is(err, ErrSessionNotActive) {
			middleware.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrAssessmentUnavailable) {
			middleware.ErrorJSON(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to generate commit draft")
		return
	}

	middleware.JSON(w, http.StatusOK, draft)
}

/// CommitSession handles POST /v1/sessions/{id}/commit wrapping persistence in an atomic transaction.
func (h *Handler) CommitSession(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil || sessionID == uuid.Nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid session id format")
		return
	}

	var req CommitSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	res, err := h.service.CommitSession(r.Context(), userID, sessionID, idempotencyKey, req)
	if err != nil {
		if errors.Is(err, ErrMissingPersonalDefinition) || errors.Is(err, ErrInvalidOrigin) || errors.Is(err, ErrInvalidUnderstandingFlag) {
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrAlreadyCommitted) || errors.Is(err, ErrSessionNotActive) {
			middleware.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to atomically commit vocabulary entry")
		return
	}

	// If duplicate request resolved from idempotency cache, return OK (200), otherwise Created (201)
	statusCode := http.StatusCreated
	if res.IsDuplicate {
		statusCode = http.StatusOK
	}

	middleware.JSON(w, statusCode, res)
}

/// ListWords handles GET /v1/words with query filtering, part of speech, and full-text search.
func (h *Handler) ListWords(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	q := r.URL.Query()
	query := q.Get("q")
	pos := q.Get("pos")
	flag := q.Get("flag")

	limit := int32(20)
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}

	offset := int32(0)
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	var startDate, endDate *time.Time
	if start := q.Get("start_date"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			startDate = &t
		} else if t, err := time.Parse("2006-01-02", start); err == nil {
			startDate = &t
		}
	}
	if end := q.Get("end_date"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			endDate = &t
		} else if t, err := time.Parse("2006-01-02", end); err == nil {
			endDate = &t
		}
	}

	filter := ListEntriesFilter{
		Query:             query,
		PartOfSpeech:      pos,
		UnderstandingFlag: flag,
		StartDate:         startDate,
		EndDate:           endDate,
		Limit:             limit,
		Offset:            offset,
	}

	entries, err := h.service.ListEntries(r.Context(), userID, filter)
	if err != nil {
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to query vocabulary entries")
		return
	}

	middleware.JSON(w, http.StatusOK, entries)
}

/// GetWord handles GET /v1/words/{id} returning detailed entry view and revision audit history.
func (h *Handler) GetWord(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	entryIDStr := chi.URLParam(r, "id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil || entryID == uuid.Nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid entry id format")
		return
	}

	detail, err := h.service.GetEntryDetail(r.Context(), userID, entryID)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to retrieve entry details")
		return
	}

	middleware.JSON(w, http.StatusOK, detail)
}

/// UpdateWord handles PATCH /v1/words/{id} to edit personal fields and append an entry revision.
func (h *Handler) UpdateWord(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	entryIDStr := chi.URLParam(r, "id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil || entryID == uuid.Nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid entry id format")
		return
	}

	var req UpdateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	updated, err := h.service.UpdateEntry(r.Context(), userID, entryID, req)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrMissingPersonalDefinition) || errors.Is(err, ErrInvalidUnderstandingFlag) {
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to update vocabulary entry")
		return
	}

	middleware.JSON(w, http.StatusOK, updated)
}

/// DeleteWord handles DELETE /v1/words/{id} cascading deletion of entry and revisions.
func (h *Handler) DeleteWord(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	entryIDStr := chi.URLParam(r, "id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil || entryID == uuid.Nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid entry id format")
		return
	}

	if err := h.service.DeleteEntry(r.Context(), userID, entryID); err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to delete vocabulary entry")
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
