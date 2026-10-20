// Purpose: HTTP transport handler for spaced repetition retrieval and review evaluations.
// Architecture: backend/internal/features/recall/handler.go
package recall

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

// Handler handles HTTP requests for adaptive recall.
type Handler struct {
	service Service
}

// NewHandler creates a new recall HTTP handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// GetDue handles GET /v1/recall/due.
// Returns list of vocabulary entries currently due for retrieval practice.
func (h *Handler) GetDue(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	res, err := h.service.GetDueWords(r.Context(), userID)
	if err != nil {
		middleware.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.JSON(w, http.StatusOK, res)
}

// SubmitReview handles POST /v1/recall/reviews.
// Evaluates recall rating (0-5), updates interval via SM-2, and returns updated schedule.
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req ReviewSubmission
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid json request body")
		return
	}

	res, err := h.service.SubmitReview(r.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRating), errors.Is(err, ErrInvalidWordID):
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrEntryNotFound):
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrUnauthorizedUser):
			middleware.ErrorJSON(w, http.StatusUnauthorized, err.Error())
		default:
			middleware.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	middleware.JSON(w, http.StatusOK, res)
}
