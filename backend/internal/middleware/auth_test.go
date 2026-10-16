// Purpose: Unit tests for JWT authentication middleware and context isolation.
// Architecture: backend/internal/middleware/auth_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/middleware"
)

const testSecret = "test-secret-key-32-chars-long-123456"

func makeTestToken(t *testing.T, userID uuid.UUID, secret string, expired bool) string {
	t.Helper()
	exp := time.Now().Add(1 * time.Hour)
	if expired {
		exp = time.Now().Add(-1 * time.Hour)
	}

	claims := &middleware.Claims{
		Email: "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wordivate-api",
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestAuthenticateMiddleware(t *testing.T) {
	testUserID := uuid.New()

	tests := []struct {
		name           string
		authHeader     string
		secret         string
		expectedStatus int
		expectUserID   bool
	}{
		{
			name:           "Valid Bearer Token",
			authHeader:     "Bearer " + makeTestToken(t, testUserID, testSecret, false),
			secret:         testSecret,
			expectedStatus: http.StatusOK,
			expectUserID:   true,
		},
		{
			name:           "Missing Authorization Header",
			authHeader:     "",
			secret:         testSecret,
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "Malformed Authorization Header (No Bearer)",
			authHeader:     "Token " + makeTestToken(t, testUserID, testSecret, false),
			secret:         testSecret,
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "Invalid Signature (Wrong Secret)",
			authHeader:     "Bearer " + makeTestToken(t, testUserID, "wrong-secret-key", false),
			secret:         testSecret,
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "Expired Token",
			authHeader:     "Bearer " + makeTestToken(t, testUserID, testSecret, true),
			secret:         testSecret,
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedUserID uuid.UUID
			var wasAuthenticated bool

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedUserID, wasAuthenticated = middleware.GetUserID(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.Authenticate(tc.secret)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectUserID {
				if !wasAuthenticated {
					t.Errorf("expected authenticated user in context, but was missing")
				}
				if capturedUserID != testUserID {
					t.Errorf("expected user ID %s, got %s", testUserID, capturedUserID)
				}
			} else {
				if wasAuthenticated {
					t.Errorf("did not expect user in context, but got %s", capturedUserID)
				}
			}
		})
	}
}
