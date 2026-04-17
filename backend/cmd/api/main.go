package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Joshua-Omz/wordivate/backend/internal/api/rest"
	"github.com/Joshua-Omz/wordivate/backend/internal/api/ws"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/auth/login", rest.LoginHandler)

		r.Get("/recall/due", rest.RecallDueHandler)

		r.Post("/words/commit", rest.WordsCommitHandler)

		r.Get("/session/stream", ws.SessionStreamHandler)
	})

	log.Println("Starting Wordivate API on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
