package socratic

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow any origin during development, or enforce host match
		return true
	},
}

// WSHandler manages WebSocket connections for interactive Socratic sessions.
type WSHandler struct {
	manager *SessionManager
}

// NewWSHandler creates a new Socratic WebSocket handler.
func NewWSHandler(manager *SessionManager) *WSHandler {
	return &WSHandler{manager: manager}
}

// HandleStream handles GET /v1/session/stream (WebSocket).
func (h *WSHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Initial welcome/question
	welcomeMsg := WSMessage{
		Type:      MessageTypeQuestion,
		Content:   "Welcome to your Socratic recall session. What word would you like to explore?",
		Timestamp: time.Now(),
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		log.Printf("ws write error: %v", err)
		return
	}

	for {
		var inMsg WSMessage
		if err := conn.ReadJSON(&inMsg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws unexpected close: %v", err)
			}
			break
		}

		// Process learner answer through session manager
		outMsg, err := h.manager.ProcessAnswer(r.Context(), inMsg.WordID, inMsg.Content, "", inMsg.Content)
		if err != nil {
			errReply := WSMessage{
				Type:      MessageTypeError,
				Content:   "Failed to process session step: " + err.Error(),
				Timestamp: time.Now(),
			}
			_ = conn.WriteJSON(errReply)
			continue
		}

		if err := conn.WriteJSON(outMsg); err != nil {
			log.Printf("ws write error: %v", err)
			break
		}
	}
}
