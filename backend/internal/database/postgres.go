// Package database manages connection pooling and encapsulates type-safe sqlc query access.
// Architecture: backend/internal/database/postgres.go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Joshua-Omz/wordivate/backend/internal/database/sqlc"
)

/// DB encapsulates the underlying database connection pool and compiled sqlc queries.
type DB struct {
	SQL     *sql.DB
	Queries *sqlc.Queries
}

/// Connect establishes an optimized PostgreSQL connection pool and initializes the sqlc Querier.
/// Parameters:
/// - driverName: database driver identifier (e.g. "postgres")
/// - dataSourceName: database DSN / connection string URL
func Connect(driverName, dataSourceName string) (*DB, error) {
	sqlDB, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open db error: %w", err)
	}

	// Connection pool tuning to prevent resource starvation under load
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// sqlc.New binds all compiled SQL queries to the connection pool
	queries := sqlc.New(sqlDB)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		// Return the initialized instance with a non-fatal error to support offline development
		return &DB{SQL: sqlDB, Queries: queries}, fmt.Errorf("ping db error: %w", err)
	}

	return &DB{SQL: sqlDB, Queries: queries}, nil
}

/// Close gracefully terminates all idle and active connections in the pool.
func (db *DB) Close() error {
	if db.SQL != nil {
		return db.SQL.Close()
	}
	return nil
}
