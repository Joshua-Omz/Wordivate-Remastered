// Purpose: Transport handler for word lookup and Socratic session initialization.
// Architecture: backend/internal/features/capture/handler.go
package capture

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

/// Handler exposes HTTP REST endpoints for the word capture feature.
type Handler struct {
	service Service
}

/// NewHandler instantiates a capture HTTP handler injecting the domain service.
func NewHandler(svc Service) *Handler {
	return &Handler{service: svc}
}

/// Lookup processes GET /v1/words/lookup?word=... requests.
func (h *Handler) Lookup(w http.ResponseWriter, r *http.Request) {
	word := strings.TrimSpace(r.URL.Query().Get("word"))
	if word == "" {
		middleware.ErrorJSON(w, http.StatusBadRequest, "query parameter 'word' is required")
		return
	}

	res, err := h.service.LookupWord(r.Context(), word)
	if err != nil {
		if errors.Is(err, ErrEmptyWord) || errors.Is(err, ErrWordTooLong) || errors.Is(err, ErrInvalidWordFormat) {
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrWordNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrDictionaryUnavailable) {
			middleware.ErrorJSON(w, http.StatusBadGateway, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to lookup word definition")
		return
	}

	middleware.JSON(w, http.StatusOK, res)
}

/// StartSession processes POST /v1/sessions requests to initiate or resume an active session.
func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req StartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "malformed request payload")
		return
	}

	res, err := h.service.StartSession(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrEmptyWord) || errors.Is(err, ErrWordTooLong) || errors.Is(err, ErrInvalidWordFormat) {
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrWordNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to start learning session")
		return
	}

	// If resuming an already active session, return 200 OK; otherwise 201 Created
	status := http.StatusCreated
	if res.IsResumed {
		status = http.StatusOK
	}

	middleware.JSON(w, status, res)
}
