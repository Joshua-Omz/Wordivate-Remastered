package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Joshua-Omz/wordivate/backend/internal/config"
	"github.com/Joshua-Omz/wordivate/backend/internal/database"
	"github.com/Joshua-Omz/wordivate/backend/internal/features/auth"
	"github.com/Joshua-Omz/wordivate/backend/internal/features/capture"
	"github.com/Joshua-Omz/wordivate/backend/internal/features/recall"
	"github.com/Joshua-Omz/wordivate/backend/internal/features/socratic"
	appMiddleware "github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

func main() {
	// 1. Load application configuration
	cfg := config.Load()

	// 2. Initialize database connection pool
	// Note: If DB is unreachable on startup, we log a warning to allow offline development
	db, err := database.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Printf("Warning: database connection not established: %v", err)
	} else {
		defer db.Close()
		log.Println("Database connection pool initialized")
	}

	// 3. Initialize domain repositories, services & dependency injection
	authRepo := auth.NewRepository(db.SQL)
	authSvc := auth.NewService(authRepo, cfg.JWTSecret)

	dictClient := capture.NewMockDictionaryClient()
	captureRepo := capture.NewRepository(db.SQL)
	captureSvc := capture.NewService(captureRepo, dictClient)

	recallSvc := recall.NewService(db)
	llmClient := socratic.NewMockLLMClient()
	socraticMgr := socratic.NewSessionManager(db, llmClient)

	// 4. Initialize HTTP & WebSocket handlers
	authHdl := auth.NewHandler(authSvc)
	captureHdl := capture.NewHandler(captureSvc)
	recallHdl := recall.NewHandler(recallSvc)
	socraticHdl := socratic.NewWSHandler(socraticMgr)

	// 5. Build Chi Router with middleware stack
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(appMiddleware.CORS("*"))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		appMiddleware.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 6. Mount V1 API Routes
	r.Route("/v1", func(r chi.Router) {
		// Public Authentication
		r.Post("/auth/login", authHdl.Login)
		r.Post("/auth/register", authHdl.Register)

		// Protected Routes (Require valid JWT Bearer token)
		r.Group(func(r chi.Router) {
			r.Use(appMiddleware.Authenticate(cfg.JWTSecret))

			// Lexical Lookup & Socratic Session Initialization (Sprint 3)
			r.Get("/words/lookup", captureHdl.Lookup)
			r.Post("/sessions", captureHdl.StartSession)

			// Spaced Repetition Due Words
			r.Get("/recall/due", recallHdl.GetDue)

			// Socratic Interactive Session (WebSocket)
			r.Get("/session/stream", socraticHdl.HandleStream)
		})
	})

	// 7. Start HTTP server with graceful shutdown
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting Wordivate API on :%s", cfg.Port)
		serverErrors <- server.ListenAndServe()
	}()

	// 8. Listen for OS termination signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server encountered critical error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Captured signal %v, initiating graceful shutdown", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v, forcing close", err)
			_ = server.Close()
		}
	}

	log.Println("Wordivate API exited cleanly")
}
