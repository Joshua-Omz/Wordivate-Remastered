package rest

import (
	"encoding/json"
	"net/http"
)

// RecallDueHandler handles GET /v1/recall/due.
func RecallDueHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}
