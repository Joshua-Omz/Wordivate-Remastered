// Purpose: HTTP transport handler for user registration and authentication endpoints.
// Architecture: backend/internal/features/auth/handler.go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

/// Handler processes HTTP requests for authentication.
type Handler struct {
	service Service
}

/// NewHandler creates a new auth HTTP handler with injected domain service.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

/// Login handles POST /v1/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.service.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			middleware.ErrorJSON(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	middleware.JSON(w, http.StatusOK, res)
}

/// Register handles POST /v1/auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.service.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			middleware.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrPasswordTooShort) || errors.Is(err, ErrInvalidEmail) {
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "registration failed")
		return
	}

	middleware.JSON(w, http.StatusCreated, res)
}
