package recall

import (
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
func (h *Handler) GetDue(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	userIDStr := "00000000-0000-0000-0000-000000000001"
	if ok {
		userIDStr = userID.String()
	}

	res, err := h.service.GetDueWords(r.Context(), userIDStr)
	if err != nil {
		middleware.ErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.JSON(w, http.StatusOK, res)
}
