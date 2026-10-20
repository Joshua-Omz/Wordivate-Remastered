// Purpose: HTTP transport handler providing Server-Sent Events (SSE) streaming and session history.
// Architecture: backend/internal/features/socratic/handler.go
package socratic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

/// Handler exposes HTTP endpoints for interactive Socratic sessions and SSE streaming.
type Handler struct {
	service Service
}

/// NewHandler instantiates a Socratic handler injecting the domain service.
func NewHandler(svc Service) *Handler {
	return &Handler{service: svc}
}

/// PostMessage handles POST /v1/sessions/{id}/messages with Server-Sent Events (SSE) streaming.
func (h *Handler) PostMessage(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user from request context
	userID, err := middleware.RequireUserID(r.Context())
	if err != nil {
		middleware.ErrorJSON(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	// 2. Parse session UUID from path parameter
	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil || sessionID == uuid.Nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid session id format")
		return
	}

	// 3. Decode incoming JSON payload
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.ErrorJSON(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	// 4. Invoke domain service
	eventChan, err := h.service.PostUserMessage(r.Context(), userID, sessionID, req.Content)
	if err != nil {
		if errors.Is(err, ErrEmptyMessage) || errors.Is(err, ErrMessageTooLong) {
			middleware.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotActive) {
			middleware.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrLLMUnavailable) {
			middleware.ErrorJSON(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to initiate Socratic turn")
		return
	}

	// 5. Ensure the HTTP response writer supports streaming flushes
	flusher, ok := w.(http.Flusher)
	if !ok {
		middleware.ErrorJSON(w, http.StatusInternalServerError, "streaming unsupported by server transport")
		return
	}

	// 6. Set standard Server-Sent Events headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Prevents Nginx/reverse-proxy buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// 7. Stream tokens to the client as Server-Sent Events
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected; abort stream
			return
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed; streaming completed
				return
			}

			payloadBytes, err := json.Marshal(event)
			if err != nil {
				continue
			}

			// Format as standard W3C SSE event: event: <type>\ndata: <json>\n\n
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payloadBytes)
			flusher.Flush()
		}
	}
}

/// GetSession handles GET /v1/sessions/{id} returning session metadata and full message history.
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
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

	history, err := h.service.GetSessionHistory(r.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to retrieve session history")
		return
	}

	middleware.JSON(w, http.StatusOK, history)
}

/// AbandonSession handles POST /v1/sessions/{id}/abandon explicitly terminating an active session.
func (h *Handler) AbandonSession(w http.ResponseWriter, r *http.Request) {
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

	session, err := h.service.AbandonSession(r.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotActive) {
			middleware.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to abandon session")
		return
	}

	middleware.JSON(w, http.StatusOK, session)
}

/// ResumeSession handles POST /v1/sessions/{id}/resume reactivating an abandoned session.
func (h *Handler) ResumeSession(w http.ResponseWriter, r *http.Request) {
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

	session, err := h.service.ResumeSession(r.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			middleware.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrSessionNotAbandoned) || errors.Is(err, ErrActiveSessionExists) {
			middleware.ErrorJSON(w, http.StatusConflict, err.Error())
			return
		}
		middleware.ErrorJSON(w, http.StatusInternalServerError, "failed to resume session")
		return
	}

	middleware.JSON(w, http.StatusOK, session)
}

