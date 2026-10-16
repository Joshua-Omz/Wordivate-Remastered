// Purpose: Data persistence abstraction and database mapping for authentication.
// Architecture: backend/internal/features/auth/repository.go
package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Joshua-Omz/wordivate/backend/internal/database/sqlc"
)

// ErrNotFound indicates the requested user record was not found in persistence.
var ErrNotFound = errors.New("user not found")

/// User defines the sanitized domain model for an account, decoupled from database schema null types.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

/// UserRepository defines persistence operations required by the authentication domain.
type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
}

type sqlRepository struct {
	q *sqlc.Queries
}

/// NewRepository constructs a concrete UserRepository backed by sqlc and PostgreSQL.
/// Parameters:
/// - db: active SQL connection pool
func NewRepository(db *sql.DB) UserRepository {
	return &sqlRepository{
		q: sqlc.New(db),
	}
}

// mapUser maps a generated sqlc User model into a sanitized domain User entity.
func mapUser(dbUser sqlc.User) *User {
	var passwordHash string
	if dbUser.PasswordHash.Valid {
		passwordHash = dbUser.PasswordHash.String
	}
	return &User{
		ID:           dbUser.ID,
		Email:        dbUser.Email,
		PasswordHash: passwordHash,
		CreatedAt:    dbUser.CreatedAt,
	}
}

/// CreateUser persists a new user record with an optional password hash.
func (r *sqlRepository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	dbUser, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		Email: email,
		PasswordHash: sql.NullString{
			String: passwordHash,
			Valid:  passwordHash != "",
		},
	})
	if err != nil {
		return nil, err
	}
	return mapUser(dbUser), nil
}

/// GetUserByEmail retrieves a user by their unique email address.
func (r *sqlRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	dbUser, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return mapUser(dbUser), nil
}

/// GetUserByID retrieves an active user record by their UUID.
func (r *sqlRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	dbUser, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return mapUser(dbUser), nil
}
