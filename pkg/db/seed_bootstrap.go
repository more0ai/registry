// Package db provides bootstrap-based seeding of system capabilities.
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/morezero/capabilities-registry/pkg/bootstrap"
)

const seedBootstrapLogPrefix = "db:seed_bootstrap"

// System user UUID used for created_by/modified_by when seeding bootstrap capabilities.
const systemUserID = "00000000-0000-0000-0000-000000000001"

// SeedBootstrap loads bootstrap config from the given path and seeds the database
// with system capabilities (capabilities, capability_versions, capability_methods,
// capability_defaults). Idempotent: uses ON CONFLICT DO NOTHING / DO UPDATE where appropriate.
func SeedBootstrap(ctx context.Context, pool *pgxpool.Pool, bootstrapFilePath string) error {
	slog.Info(fmt.Sprintf("%s - seeding from %s", seedBootstrapLogPrefix, bootstrapFilePath))

	cfg, err := bootstrap.LoadBootstrapConfig(bootstrapFilePath)
	if err != nil {
		return fmt.Errorf("%s - load bootstrap config: %w", seedBootstrapLogPrefix, err)
	}
	if cfg == nil || len(cfg.Capabilities) == 0 {
		slog.Info(fmt.Sprintf("%s - no capabilities to seed", seedBootstrapLogPrefix))
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s - begin tx: %w", seedBootstrapLogPrefix, err)
	}
	defer tx.Rollback(ctx)

	for capRef, cap := range cfg.Capabilities {
		app, name := parseCapRef(capRef)
		if app == "" || name == "" {
			slog.Warn(fmt.Sprintf("%s - skip invalid cap ref %q", seedBootstrapLogPrefix, capRef))
			continue
		}

		// 1. Insert or update capability (sync description)
		var capID string
		desc := cap.Description
		status := "Active"
		err := tx.QueryRow(ctx,
			`INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
			 VALUES ($1, $2, $3, '{}', $4, $5::uuid, $5::uuid)
			 ON CONFLICT (app, name) DO UPDATE SET
			   description = COALESCE(EXCLUDED.description, capabilities.description),
			   modified = NOW(),
			   modified_by = EXCLUDED.modified_by
			 RETURNING id`,
			app, name, desc, status, systemUserID).Scan(&capID)
		if err != nil {
			return fmt.Errorf("%s - insert capability %s: %w", seedBootstrapLogPrefix, capRef, err)
		}

		// 2. Insert version (major from bootstrap; minor/patch 0). Use WHERE NOT EXISTS
		// because UNIQUE(capability_id, major, minor, patch, prerelease) treats NULL prerelease
		// as distinct in PostgreSQL, so ON CONFLICT would not prevent duplicates.
		major := cap.Major
		if major < 1 {
			major = 1
		}
		versionStatus := strings.ToLower(cap.Status)
		if versionStatus == "" {
			versionStatus = "active"
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
			 SELECT $1::uuid, $2, 0, 0, $3, $4::uuid, $4::uuid
			 WHERE NOT EXISTS (
			   SELECT 1 FROM capability_versions
			   WHERE capability_id = $1::uuid AND major = $2 AND minor = 0 AND patch = 0 AND prerelease IS NULL
			 )`,
			capID, major, versionStatus, systemUserID)
		if err != nil {
			return fmt.Errorf("%s - insert version %s: %w", seedBootstrapLogPrefix, capRef, err)
		}

		// 3. Get version id for methods
		var versionID string
		err = tx.QueryRow(ctx,
			`SELECT id FROM capability_versions WHERE capability_id = $1::uuid AND major = $2 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1`,
			capID, major).Scan(&versionID)
		if err != nil {
			return fmt.Errorf("%s - get version id %s: %w", seedBootstrapLogPrefix, capRef, err)
		}

		// 4. Insert methods (with optional metadata from methodsMetadata)
		for _, methodName := range cap.Methods {
			meta := cap.MethodsMetadata[methodName]
			var descPtr *string
			if meta.Description != "" {
				descPtr = &meta.Description
			}
			inputJSON := mustMarshalJSON(meta.InputSchema, []byte("{}"))
			outputJSON := mustMarshalJSON(meta.OutputSchema, []byte("{}"))
			modes := meta.Modes
			if len(modes) == 0 {
				modes = []string{"sync"}
			}
			tags := meta.Tags
			if tags == nil {
				tags = []string{}
			}
			examplesJSON := mustMarshalJSON(meta.Examples, []byte("[]"))

			_, err = tx.Exec(ctx,
				`INSERT INTO capability_methods (version_id, name, description, input_schema, output_schema, tags, policies, examples, modes, created_by, modified_by)
				 VALUES ($1::uuid, $2, $3, $4, $5, $6, '{}'::jsonb, $7, $8, $9::uuid, $9::uuid)
				 ON CONFLICT (version_id, name) DO UPDATE SET
				   description = COALESCE(NULLIF(EXCLUDED.description, ''), capability_methods.description),
				   input_schema = EXCLUDED.input_schema,
				   output_schema = EXCLUDED.output_schema,
				   tags = EXCLUDED.tags,
				   examples = EXCLUDED.examples,
				   modes = EXCLUDED.modes,
				   modified = NOW(),
				   modified_by = EXCLUDED.modified_by`,
				versionID, methodName, descPtr, inputJSON, outputJSON, tags, examplesJSON, modes, systemUserID)
			if err != nil {
				return fmt.Errorf("%s - insert method %s.%s: %w", seedBootstrapLogPrefix, capRef, methodName, err)
			}
		}

		// 5. Insert default major for production env
		_, err = tx.Exec(ctx,
			`INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
			 VALUES ($1::uuid, $2, 'production', $3::uuid, $3::uuid)
			 ON CONFLICT (capability_id, env) DO NOTHING`,
			capID, major, systemUserID)
		if err != nil {
			return fmt.Errorf("%s - insert default %s: %w", seedBootstrapLogPrefix, capRef, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s - commit: %w", seedBootstrapLogPrefix, err)
	}
	slog.Info(fmt.Sprintf("%s - seeded %d capabilities", seedBootstrapLogPrefix, len(cfg.Capabilities)))
	return nil
}

// parseCapRef splits "app.name" into app and name (e.g. "system.registry" -> "system", "registry").
func parseCapRef(capRef string) (app, name string) {
	parts := strings.SplitN(capRef, ".", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

// mustMarshalJSON marshals v to JSON; returns defaultJSON if v is nil or marshaling fails.
func mustMarshalJSON(v interface{}, defaultJSON []byte) []byte {
	if v == nil {
		return defaultJSON
	}
	b, err := json.Marshal(v)
	if err != nil {
		return defaultJSON
	}
	return b
}
