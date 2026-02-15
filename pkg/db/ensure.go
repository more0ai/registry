// Package db provides database connection pooling via pgx.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ensureLogPrefix = "db:ensure"

// safeDBName matches allowed database names (alphanumeric and underscore only).
var safeDBName = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// EnsureDatabase creates the database from databaseURL if it does not exist, and ensures
// required extensions (uuid-ossp, pgcrypto) are enabled. Call before NewPool when the
// app should auto-create its DB on platform Postgres (e.g. registry, registry_test).
func EnsureDatabase(ctx context.Context, databaseURL string) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("%s - invalid database URL: %w", ensureLogPrefix, err)
	}
	dbname := strings.TrimPrefix(u.Path, "/")
	if idx := strings.Index(dbname, "?"); idx >= 0 {
		dbname = dbname[:idx]
	}
	dbname = strings.TrimSpace(dbname)
	if dbname == "" {
		return fmt.Errorf("%s - database name empty in URL", ensureLogPrefix)
	}
	if !safeDBName.MatchString(dbname) {
		return fmt.Errorf("%s - database name %q contains invalid characters", ensureLogPrefix, dbname)
	}

	postgresURL := buildPostgresURL(u)
	config, err := pgxpool.ParseConfig(postgresURL)
	if err != nil {
		return fmt.Errorf("%s - failed to parse postgres URL: %w", ensureLogPrefix, err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("%s - failed to connect to postgres: %w", ensureLogPrefix, err)
	}
	defer pool.Close()

	var exists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, dbname).Scan(&exists)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("%s - failed to check database: %w", ensureLogPrefix, err)
	}

	if !exists {
		slog.Info(fmt.Sprintf("%s - Creating database %q", ensureLogPrefix, dbname))
		_, err = pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdent(dbname)))
		if err != nil {
			return fmt.Errorf("%s - CREATE DATABASE failed: %w", ensureLogPrefix, err)
		}
	}

	pool.Close()

	connConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("%s - failed to parse database URL: %w", ensureLogPrefix, err)
	}
	connPool, err := pgxpool.NewWithConfig(ctx, connConfig)
	if err != nil {
		return fmt.Errorf("%s - failed to connect to %q: %w", ensureLogPrefix, dbname, err)
	}
	defer connPool.Close()

	for _, ext := range []string{"uuid-ossp", "pgcrypto"} {
		_, err = connPool.Exec(ctx, fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s", quoteIdent(ext)))
		if err != nil {
			return fmt.Errorf("%s - CREATE EXTENSION %s: %w", ensureLogPrefix, ext, err)
		}
	}

	return nil
}

func buildPostgresURL(u *url.URL) string {
	postgres := *u
	postgres.Path = "/postgres"
	return postgres.String()
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
