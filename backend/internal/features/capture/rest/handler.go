package rest

import (
	"encoding/json"
	"net/http"
)

// WordsCommitHandler handles POST /v1/words/commit.
func WordsCommitHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}
