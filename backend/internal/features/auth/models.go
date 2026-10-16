// Purpose: Data transfer objects (DTOs) and request/response models for authentication.
// Architecture: backend/internal/features/auth/models.go
package auth

import (
	"time"

	"github.com/google/uuid"
)

/// LoginRequest defines the credentials required to authenticate an existing user.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

/// RegisterRequest defines the parameters required to provision a new user account.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

/// AuthResponse encapsulates the session token and user metadata returned upon authentication.
type AuthResponse struct {
	Token     string    `json:"token"`
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}
