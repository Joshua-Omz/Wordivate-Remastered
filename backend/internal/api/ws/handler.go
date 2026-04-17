package ws

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// Allow same-host connections (no Origin header means non-browser client).
		if origin == "" {
			return true
		}
		host := r.Host
		return origin == "http://"+host || origin == "https://"+host
	},
}

// SessionStreamHandler handles GET /v1/session/stream (WebSocket).
func SessionStreamHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Placeholder: echo messages back to the client.
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("ws read error: %v", err)
			break
		}
		if err = conn.WriteMessage(mt, msg); err != nil {
			log.Printf("ws write error: %v", err)
			break
		}
	}
}
