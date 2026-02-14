// Package db provides registry data clearing.
package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

const clearLogPrefix = "db:clear"

// ClearRegistry truncates all registry tables (capability_methods, capability_versions,
// capability_defaults, capability_tenant_rules, capabilities) in dependency order.
// Schema is preserved; only data is removed. RESTART IDENTITY resets sequences.
func ClearRegistry(ctx context.Context, pool *pgxpool.Pool) error {
	slog.Info(fmt.Sprintf("%s - Clearing registry tables", clearLogPrefix))

	// Truncate in dependency order: children first, then capabilities.
	// CASCADE handles any other tables that reference these.
	_, err := pool.Exec(ctx, `TRUNCATE TABLE
		capability_methods,
		capability_versions,
		capability_defaults,
		capability_tenant_rules,
		capabilities
		RESTART IDENTITY CASCADE`)
	if err != nil {
		return fmt.Errorf("%s - truncate failed: %w", clearLogPrefix, err)
	}

	slog.Info(fmt.Sprintf("%s - Registry cleared", clearLogPrefix))
	return nil
}
