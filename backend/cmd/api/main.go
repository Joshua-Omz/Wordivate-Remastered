package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	adaptiveRecallRest "github.com/Joshua-Omz/wordivate/backend/internal/features/adaptive_recall/rest"
	authRest "github.com/Joshua-Omz/wordivate/backend/internal/features/auth/rest"
	captureRest "github.com/Joshua-Omz/wordivate/backend/internal/features/capture/rest"
	socraticSessionWS "github.com/Joshua-Omz/wordivate/backend/internal/features/socratic_session/ws"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/auth/login", authRest.LoginHandler)

		r.Get("/recall/due", adaptiveRecallRest.RecallDueHandler)

		r.Post("/words/commit", captureRest.WordsCommitHandler)

		r.Get("/session/stream", socraticSessionWS.SessionStreamHandler)
	})

	log.Println("Starting Wordivate API on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
