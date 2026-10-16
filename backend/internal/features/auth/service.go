// Purpose: Domain authentication service implementing registration, credential verification, and JWT generation.
// Architecture: backend/internal/features/auth/service.go
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials indicates the email was not found or the password mismatch.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrUserAlreadyExists indicates an active account with the given email already exists.
	ErrUserAlreadyExists = errors.New("user with this email already exists")

	// ErrPasswordTooShort enforces minimum password complexity standards.
	ErrPasswordTooShort = errors.New("password must be at least 8 characters long")

	// ErrInvalidEmail indicates a missing or malformed email address.
	ErrInvalidEmail = errors.New("valid email address is required")
)

/// Claims defines the cryptographic JWT claims containing standard RFC 7519 fields.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

/// Service defines authentication and credential lifecycle operations.
type Service interface {
	Login(ctx context.Context, req LoginRequest) (*AuthResponse, error)
	Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error)
	GenerateToken(userID uuid.UUID, email string) (string, time.Time, error)
}

type service struct {
	repo      UserRepository
	jwtSecret string
}

/// NewService instantiates a production-ready authentication service.
/// Parameters:
/// - repo: domain repository abstraction decoupling persistence from business logic
/// - jwtSecret: HMAC secret key used to cryptographically sign session tokens
func NewService(repo UserRepository, jwtSecret string) Service {
	return &service{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

/// Register provisions a new user record with a bcrypt-hashed password and issues a session JWT.
func (s *service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, ErrInvalidEmail
	}
	if len(req.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	// 1. Guard against duplicate registration
	existingUser, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("checking existing user: %w", err)
	}

	// 2. Hash password with bcrypt cost factor (DefaultCost = 10)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	// 3. Persist new user via repository abstraction
	user, err := s.repo.CreateUser(ctx, email, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("persisting user: %w", err)
	}

	// 4. Issue authenticated JWT token
	token, expiresAt, err := s.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}

	return &AuthResponse{
		Token:     token,
		UserID:    user.ID,
		Email:     user.Email,
		ExpiresAt: expiresAt,
	}, nil
}

/// Login validates user credentials against the hashed password and returns a session JWT.
func (s *service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" || req.Password == "" {
		return nil, ErrInvalidCredentials
	}

	// 1. Fetch user by email via repository abstraction
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Do not leak whether the email exists or not
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("fetching user for login: %w", err)
	}

	// 2. Verify account has a password hash provisioned
	if user.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}

	// 3. Compare supplied password with stored bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 4. Issue session token
	token, expiresAt, err := s.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generating login token: %w", err)
	}

	return &AuthResponse{
		Token:     token,
		UserID:    user.ID,
		Email:     user.Email,
		ExpiresAt: expiresAt,
	}, nil
}

/// GenerateToken creates an HMAC-SHA256 signed JWT valid for 24 hours.
func (s *service) GenerateToken(userID uuid.UUID, email string) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wordivate-api",
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signedString, expiresAt, nil
}
