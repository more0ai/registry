// Package db provides database connection pooling via pgx.
package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

const logPrefix = "db:pool"

// NewPool creates a new pgx connection pool from the given database URL.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	slog.Info(fmt.Sprintf("%s - Connecting to database", logPrefix))

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("%s - failed to parse database URL: %w", logPrefix, err)
	}

	// Set sensible pool defaults
	config.MaxConns = 20
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("%s - failed to create pool: %w", logPrefix, err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%s - failed to ping database: %w", logPrefix, err)
	}

	slog.Info(fmt.Sprintf("%s - Database connection established", logPrefix))
	return pool, nil
}

// RunMigrations applies SQL migration files in order.
// For production use, consider golang-migrate or similar.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFiles []string) error {
	slog.Info(fmt.Sprintf("%s - Running %d migrations", logPrefix, len(migrationFiles)))

	for _, sql := range migrationFiles {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("%s - migration failed: %w", logPrefix, err)
		}
	}

	slog.Info(fmt.Sprintf("%s - Migrations complete", logPrefix))
	return nil
}
