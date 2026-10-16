// Purpose: Unit tests for authentication service business logic and validation using in-memory mock repository.
// Architecture: backend/internal/features/auth/service_test.go
package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/auth"
)

const testSecret = "super-secret-jwt-key-for-testing-12345"

type mockUserRepository struct {
	users map[string]*auth.User
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[string]*auth.User),
	}
}

func (m *mockUserRepository) CreateUser(ctx context.Context, email, passwordHash string) (*auth.User, error) {
	u := &auth.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	m.users[email] = u
	return u, nil
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*auth.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, auth.ErrNotFound
}

func TestValidationErrors(t *testing.T) {
	mockRepo := newMockUserRepository()
	svc := auth.NewService(mockRepo, testSecret)
	ctx := context.Background()

	t.Run("Register Empty Email", func(t *testing.T) {
		_, err := svc.Register(ctx, auth.RegisterRequest{
			Email:    "",
			Password: "password123",
		})
		if err != auth.ErrInvalidEmail {
			t.Errorf("expected ErrInvalidEmail, got %v", err)
		}
	})

	t.Run("Register Password Too Short", func(t *testing.T) {
		_, err := svc.Register(ctx, auth.RegisterRequest{
			Email:    "test@example.com",
			Password: "short",
		})
		if err != auth.ErrPasswordTooShort {
			t.Errorf("expected ErrPasswordTooShort, got %v", err)
		}
	})

	t.Run("Login Empty Email", func(t *testing.T) {
		_, err := svc.Login(ctx, auth.LoginRequest{
			Email:    "",
			Password: "password123",
		})
		if err != auth.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Login Empty Password", func(t *testing.T) {
		_, err := svc.Login(ctx, auth.LoginRequest{
			Email:    "test@example.com",
			Password: "",
		})
		if err != auth.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestRegisterAndLoginFlow(t *testing.T) {
	mockRepo := newMockUserRepository()
	svc := auth.NewService(mockRepo, testSecret)
	ctx := context.Background()

	email := "scholar@wordivate.org"
	pass := "superSecret123"

	// 1. Successful Registration
	regResp, err := svc.Register(ctx, auth.RegisterRequest{
		Email:    email,
		Password: pass,
	})
	if err != nil {
		t.Fatalf("expected successful registration, got: %v", err)
	}
	if regResp.Token == "" || regResp.UserID == uuid.Nil {
		t.Fatalf("expected valid token and UserID in registration response")
	}

	// 2. Duplicate Registration Guard
	_, err = svc.Register(ctx, auth.RegisterRequest{
		Email:    email,
		Password: "anotherPassword123",
	})
	if err != auth.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists on duplicate registration, got: %v", err)
	}

	// 3. Successful Login
	loginResp, err := svc.Login(ctx, auth.LoginRequest{
		Email:    email,
		Password: pass,
	})
	if err != nil {
		t.Fatalf("expected successful login, got: %v", err)
	}
	if loginResp.Token == "" || loginResp.UserID != regResp.UserID {
		t.Errorf("expected matching UserID on login, got %v vs %v", loginResp.UserID, regResp.UserID)
	}

	// 4. Invalid Password Login Failure
	_, err = svc.Login(ctx, auth.LoginRequest{
		Email:    email,
		Password: "wrongPassword123",
	})
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials on wrong password, got: %v", err)
	}

	// 5. Non-existent User Login Failure
	_, err = svc.Login(ctx, auth.LoginRequest{
		Email:    "ghost@wordivate.org",
		Password: pass,
	})
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for non-existent user, got: %v", err)
	}
}

func TestGenerateToken(t *testing.T) {
	mockRepo := newMockUserRepository()
	svc := auth.NewService(mockRepo, testSecret)
	userID := uuid.New()
	email := "learner@example.com"

	tokenString, expiresAt, err := svc.GenerateToken(userID, email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if tokenString == "" {
		t.Errorf("expected non-empty token string")
	}

	if expiresAt.Before(time.Now().Add(23 * time.Hour)) {
		t.Errorf("expected expiration ~24h in future, got %v", expiresAt)
	}

	// Verify token claims using jwt parser
	claims := &auth.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("generated token failed parsing: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf("expected subject %s, got %s", userID.String(), claims.Subject)
	}

	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}
}

func TestBcryptHashVerification(t *testing.T) {
	password := "securePassword123!"
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	// Correct password must match
	if err := bcrypt.CompareHashAndPassword(hashed, []byte(password)); err != nil {
		t.Errorf("expected password to match, got %v", err)
	}

	// Wrong password must fail
	if err := bcrypt.CompareHashAndPassword(hashed, []byte("wrongPassword")); err == nil {
		t.Errorf("expected wrong password to fail comparison")
	}
}
