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
