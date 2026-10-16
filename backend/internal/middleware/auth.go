// Purpose: JWT authentication and tenant-isolation request context middleware.
// Architecture: backend/internal/middleware/auth.go
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// contextKey is unexported to prevent key collision across packages.
type contextKey struct{}

// userIDKey is the private key for storing authenticated user ID in context.Context.
var userIDKey = contextKey{}

/// Claims mirrors the JWT payload structure issued by the auth service.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

/// Authenticate validates the HTTP Authorization Bearer token and injects the user UUID into context.
/// If the token is missing, expired, or cryptographically invalid, it halts the request with HTTP 401.
func Authenticate(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				ErrorJSON(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				ErrorJSON(w, http.StatusUnauthorized, "invalid authorization header format; expected Bearer <token>")
				return
			}

			tokenString := parts[1]
			claims := &Claims{}

			// Cryptographically verify token signature with HMAC-SHA256
			token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				ErrorJSON(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Parse user UUID from the Subject claim
			userID, err := uuid.Parse(claims.Subject)
			if err != nil || userID == uuid.Nil {
				ErrorJSON(w, http.StatusUnauthorized, "malformed user identifier in token")
				return
			}

			// Inject user ID into request context for multi-tenant isolation
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

/// WithUserID injects a user UUID into the given context (useful for tests and synthetic execution).
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

/// GetUserID extracts the authenticated user UUID from the context.
/// Returns the UUID and true if present and valid; otherwise returns uuid.Nil and false.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	val, ok := ctx.Value(userIDKey).(uuid.UUID)
	if !ok || val == uuid.Nil {
		return uuid.Nil, false
	}
	return val, true
}

/// RequireUserID extracts the user UUID or returns an error if unauthenticated.
func RequireUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := GetUserID(ctx)
	if !ok {
		return uuid.Nil, errors.New("unauthenticated user context")
	}
	return userID, nil
}
