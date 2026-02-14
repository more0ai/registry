package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

const registriesLogPrefix = "db:registries"

// GetRegistryByAlias retrieves a registry entry by its alias.
func (r *Repository) GetRegistryByAlias(ctx context.Context, alias string) (*RegistryEntry, error) {
	slog.Debug(fmt.Sprintf("%s - GetRegistryByAlias alias=%s", registriesLogPrefix, alias))

	var e RegistryEntry
	err := r.pool.QueryRow(ctx,
		`SELECT id, alias, nats_url, registry_subject, is_default, config, created, modified
		 FROM registries
		 WHERE alias = $1
		 LIMIT 1`, alias,
	).Scan(
		&e.ID, &e.Alias, &e.NatsUrl, &e.RegistrySubject,
		&e.IsDefault, &e.Config, &e.Created, &e.Modified,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s - GetRegistryByAlias failed: %w", registriesLogPrefix, err)
	}
	return &e, nil
}

// GetDefaultRegistry returns the registry entry marked as default.
func (r *Repository) GetDefaultRegistry(ctx context.Context) (*RegistryEntry, error) {
	var e RegistryEntry
	err := r.pool.QueryRow(ctx,
		`SELECT id, alias, nats_url, registry_subject, is_default, config, created, modified
		 FROM registries
		 WHERE is_default = true
		 LIMIT 1`,
	).Scan(
		&e.ID, &e.Alias, &e.NatsUrl, &e.RegistrySubject,
		&e.IsDefault, &e.Config, &e.Created, &e.Modified,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s - GetDefaultRegistry failed: %w", registriesLogPrefix, err)
	}
	return &e, nil
}

// ListRegistries returns all active registry entries.
func (r *Repository) ListRegistries(ctx context.Context) ([]RegistryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, alias, nats_url, registry_subject, is_default, config, created, modified
		 FROM registries
		 ORDER BY alias ASC`)
	if err != nil {
		return nil, fmt.Errorf("%s - ListRegistries failed: %w", registriesLogPrefix, err)
	}
	defer rows.Close()

	var entries []RegistryEntry
	for rows.Next() {
		var e RegistryEntry
		if err := rows.Scan(
			&e.ID, &e.Alias, &e.NatsUrl, &e.RegistrySubject,
			&e.IsDefault, &e.Config, &e.Created, &e.Modified,
		); err != nil {
			return nil, fmt.Errorf("%s - ListRegistries scan failed: %w", registriesLogPrefix, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
