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

// MigrationStatus reports whether migrations have been applied (by checking for capabilities table).
func MigrationStatus(ctx context.Context, pool *pgxpool.Pool, migrationPath string) error {
	const statusLogPrefix = "db:MigrationStatus"

	// Check if schema exists (capabilities table is created in first migration)
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'capabilities')`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("%s - failed to check schema: %w", statusLogPrefix, err)
	}

	files, err := LoadMigrationFiles(migrationPath)
	if err != nil {
		return fmt.Errorf("%s - load migration list: %w", statusLogPrefix, err)
	}

	if exists {
		fmt.Printf("Migration status: applied (schema present, %d migration files in %s)\n", len(files), migrationPath)
	} else {
		fmt.Printf("Migration status: not applied (run 'registry migrate up'). %d migration files in %s\n", len(files), migrationPath)
	}
	return nil
}

// MigrationDown rolls back the last migration. Current implementation does not support down migrations
// (migrations are forward-only); this is a no-op with a message.
func MigrationDown(ctx context.Context, pool *pgxpool.Pool, _ string) error {
	fmt.Println("Migration down: not supported (migrations are forward-only). Use a database backup to roll back.")
	return nil
}
