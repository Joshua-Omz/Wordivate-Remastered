package config

import (
	"os"
)

// Config holds all configuration values for the application.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	Environment string
}

// Load reads configuration from environment variables with fallback defaults.
func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/wordivate?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "wordivate-secret-key-change-in-production"),
		Environment: getEnv("ENV", "development"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
