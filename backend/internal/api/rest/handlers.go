package rest

import (
	"encoding/json"
	"net/http"
)

// LoginHandler handles POST /v1/auth/login.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}

// RecallDueHandler handles GET /v1/recall/due.
func RecallDueHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}

// WordsCommitHandler handles POST /v1/words/commit.
func WordsCommitHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}
