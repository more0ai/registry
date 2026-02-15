// Package db provides seeding of capability metadata from registry/capabilities/*.json.
// SeedFromCapabilityMetadataFile loads a single capability metadata file (e.g. metadata.json
// defining system.registry) and upserts the capability, version, methods with full schemas,
// and default major. The bootstrap file does not seed the database; bootstrap response
// is built from the DB when clients request system.registry.bootstrap.

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const seedCapabilityMetadataLogPrefix = "db:seed_capability_metadata"

// CapabilityMetadataFile is the shape of registry/capabilities/metadata.json (one capability).
type CapabilityMetadataFile struct {
	Capability  string                           `json:"capability"`
	Subject     string                           `json:"subject,omitempty"`
	Major       int                              `json:"major"`
	Version     string                           `json:"version"`
	Status      string                           `json:"status"`
	Description string                           `json:"description,omitempty"`
	IsSystem    bool                             `json:"isSystem,omitempty"`
	Methods     map[string]CapabilityMethodMeta  `json:"methods"`
}

// CapabilityMethodMeta holds per-method metadata (description, schemas, modes, tags).
type CapabilityMethodMeta struct {
	Description  string                   `json:"description,omitempty"`
	InputSchema  map[string]interface{}  `json:"inputSchema,omitempty"`
	OutputSchema map[string]interface{}   `json:"outputSchema,omitempty"`
	Modes        []string                 `json:"modes,omitempty"`
	Tags         []string                 `json:"tags,omitempty"`
	Examples     []interface{}            `json:"examples,omitempty"`
}

// SeedFromCapabilityMetadataFile loads the capability metadata file at path and seeds the database
// with that capability (capabilities, capability_versions, capability_methods, capability_defaults).
// Idempotent: uses ON CONFLICT DO UPDATE.
// If baseDir is non-empty, path must resolve to a location under baseDir (path traversal protection).
func SeedFromCapabilityMetadataFile(ctx context.Context, pool *pgxpool.Pool, path string, baseDir string) error {
	if path == "" {
		return nil
	}
	if baseDir != "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("%s - resolve path: %w", seedCapabilityMetadataLogPrefix, err)
		}
		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			return fmt.Errorf("%s - resolve base dir: %w", seedCapabilityMetadataLogPrefix, err)
		}
		rel, err := filepath.Rel(absBase, absPath)
		if err != nil {
			return fmt.Errorf("%s - path not under base: %w", seedCapabilityMetadataLogPrefix, err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%s - path must be under base directory", seedCapabilityMetadataLogPrefix)
		}
		path = absPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug(fmt.Sprintf("%s - file not found, skip: %s", seedCapabilityMetadataLogPrefix, path))
			return nil
		}
		return fmt.Errorf("%s - read file: %w", seedCapabilityMetadataLogPrefix, err)
	}

	var meta CapabilityMetadataFile
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("%s - parse %s: %w", seedCapabilityMetadataLogPrefix, path, err)
	}
	if meta.Capability == "" || len(meta.Methods) == 0 {
		return fmt.Errorf("%s - %s: capability and methods required", seedCapabilityMetadataLogPrefix, path)
	}

	app, name := parseCapRef(meta.Capability)
	if app == "" || name == "" {
		return fmt.Errorf("%s - %s: invalid capability ref %q", seedCapabilityMetadataLogPrefix, path, meta.Capability)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s - begin tx: %w", seedCapabilityMetadataLogPrefix, err)
	}
	defer tx.Rollback(ctx)

	// 1. Insert or update capability
	var capID string
	status := "Active"
	if len(meta.Status) > 0 {
		switch strings.ToLower(meta.Status) {
		case "deprecated":
			status = "Deprecated"
		case "disabled":
			status = "Disabled"
		default:
			status = "Active"
		}
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
		 VALUES ($1, $2, $3, '{}', $4, $5::uuid, $5::uuid)
		 ON CONFLICT (app, name) DO UPDATE SET
		   description = COALESCE(NULLIF(EXCLUDED.description, ''), capabilities.description),
		   modified = NOW(),
		   modified_by = EXCLUDED.modified_by
		 RETURNING id`,
		app, name, meta.Description, status, systemUserID).Scan(&capID)
	if err != nil {
		return fmt.Errorf("%s - insert capability %s: %w", seedCapabilityMetadataLogPrefix, meta.Capability, err)
	}

	// 2. Insert version (major from file; minor/patch from version string or 0)
	major := meta.Major
	if major < 1 {
		major = 1
	}
	versionStatus := strings.ToLower(meta.Status)
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
		return fmt.Errorf("%s - insert version %s: %w", seedCapabilityMetadataLogPrefix, meta.Capability, err)
	}

	// 3. Get version id for methods
	var versionID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM capability_versions WHERE capability_id = $1::uuid AND major = $2 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1`,
		capID, major).Scan(&versionID)
	if err != nil {
		return fmt.Errorf("%s - get version id %s: %w", seedCapabilityMetadataLogPrefix, meta.Capability, err)
	}

	// 4. Insert or update each method with full metadata
	for methodName, methodMeta := range meta.Methods {
		var descPtr *string
		if methodMeta.Description != "" {
			descPtr = &methodMeta.Description
		}
		inputJSON := mustMarshalJSON(methodMeta.InputSchema, []byte("{}"))
		outputJSON := mustMarshalJSON(methodMeta.OutputSchema, []byte("{}"))
		modes := methodMeta.Modes
		if len(modes) == 0 {
			modes = []string{"sync"}
		}
		tags := methodMeta.Tags
		if tags == nil {
			tags = []string{}
		}
		examplesJSON := mustMarshalJSON(methodMeta.Examples, []byte("[]"))

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
			return fmt.Errorf("%s - insert method %s.%s: %w", seedCapabilityMetadataLogPrefix, meta.Capability, methodName, err)
		}
	}

	// 5. Ensure default major for production
	_, err = tx.Exec(ctx,
		`INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
		 VALUES ($1::uuid, $2, 'production', $3::uuid, $3::uuid)
		 ON CONFLICT (capability_id, env) DO NOTHING`,
		capID, major, systemUserID)
	if err != nil {
		return fmt.Errorf("%s - insert default %s: %w", seedCapabilityMetadataLogPrefix, meta.Capability, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s - commit: %w", seedCapabilityMetadataLogPrefix, err)
	}
	slog.Info(fmt.Sprintf("%s - seeded capability %s from %s (%d methods)", seedCapabilityMetadataLogPrefix, meta.Capability, path, len(meta.Methods)))
	return nil
}
