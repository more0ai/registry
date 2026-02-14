package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const repoLogPrefix = "db:repository"

// Repository provides database access for registry operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository with the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// =========================================================================
// CAPABILITY OPERATIONS
// =========================================================================

// GetCapability finds a capability by app and name.
func (r *Repository) GetCapability(ctx context.Context, app, name string) (*Capability, error) {
	slog.Debug(fmt.Sprintf("%s - GetCapability app=%s name=%s", repoLogPrefix, app, name))

	row := r.pool.QueryRow(ctx,
		`SELECT id, app, name, description, tags, status, object, revision,
		        created, created_by, modified, modified_by, config, ext
		 FROM capabilities
		 WHERE app = $1 AND name = $2
		 LIMIT 1`, app, name)

	return scanCapability(row)
}

// GetCapabilityByID finds a capability by ID.
func (r *Repository) GetCapabilityByID(ctx context.Context, id string) (*Capability, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, app, name, description, tags, status, object, revision,
		        created, created_by, modified, modified_by, config, ext
		 FROM capabilities
		 WHERE id = $1
		 LIMIT 1`, id)

	return scanCapability(row)
}

// UpsertCapability creates or updates a capability.
func (r *Repository) UpsertCapability(ctx context.Context, params UpsertCapabilityParams) (*Capability, error) {
	slog.Info(fmt.Sprintf("%s - UpsertCapability app=%s name=%s", repoLogPrefix, params.App, params.Name))

	now := time.Now().UTC()

	row := r.pool.QueryRow(ctx,
		`INSERT INTO capabilities (app, name, description, tags, created_by, modified_by, created, modified)
		 VALUES ($1, $2, $3, $4, $5, $5, $6, $6)
		 ON CONFLICT (app, name) DO UPDATE SET
		   description = COALESCE($3, capabilities.description),
		   tags = COALESCE($4, capabilities.tags),
		   revision = capabilities.revision + 1,
		   modified = $6,
		   modified_by = $5
		 RETURNING id, app, name, description, tags, status, object, revision,
		           created, created_by, modified, modified_by, config, ext`,
		params.App, params.Name, params.Description, params.Tags, params.UserID, now)

	return scanCapability(row)
}

// UpsertCapabilityParams holds parameters for UpsertCapability.
type UpsertCapabilityParams struct {
	App         string
	Name        string
	Description *string
	Tags        []string
	UserID      string
}

// ListCapabilities lists capabilities with optional filters.
func (r *Repository) ListCapabilities(ctx context.Context, params ListCapabilitiesParams) ([]Capability, int, error) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	limit := params.Limit
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Build query dynamically
	query := `SELECT id, app, name, description, tags, status, object, revision,
	                 created, created_by, modified, modified_by, config, ext
	          FROM capabilities WHERE 1=1`
	countQuery := `SELECT COUNT(*)::int FROM capabilities WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if params.App != "" {
		clause := fmt.Sprintf(` AND app = $%d`, argIdx)
		query += clause
		countQuery += clause
		args = append(args, params.App)
		argIdx++
	}
	if params.Status != "" && params.Status != "all" {
		clause := fmt.Sprintf(` AND status = $%d`, argIdx)
		query += clause
		countQuery += clause
		args = append(args, params.Status)
		argIdx++
	}
	if params.Query != "" {
		clause := fmt.Sprintf(` AND (name ILIKE $%d OR description ILIKE $%d)`, argIdx, argIdx)
		query += clause
		countQuery += clause
		args = append(args, "%"+params.Query+"%")
		argIdx++
	}
	if len(params.Tags) > 0 {
		clause := fmt.Sprintf(` AND tags && $%d`, argIdx)
		query += clause
		countQuery += clause
		args = append(args, params.Tags)
		argIdx++
	}

	// Count
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("%s - ListCapabilities count failed: %w", repoLogPrefix, err)
	}

	// Data
	query += ` ORDER BY modified DESC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%s - ListCapabilities query failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	var caps []Capability
	for rows.Next() {
		cap, err := scanCapabilityFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		caps = append(caps, *cap)
	}

	return caps, total, nil
}

// ListCapabilitiesParams holds parameters for ListCapabilities.
type ListCapabilitiesParams struct {
	App    string
	Tags   []string
	Query  string
	Status string
	Page   int
	Limit  int
}

// =========================================================================
// VERSION OPERATIONS
// =========================================================================

// GetVersions returns all versions for a capability, ordered by semver descending.
func (r *Repository) GetVersions(ctx context.Context, capabilityID string) ([]CapabilityVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, capability_id, major, minor, patch, prerelease, build_metadata,
		        version_string, status, deprecation_reason, deprecated_at, disabled_at,
		        description, changelog, metadata, object, created, created_by, modified, modified_by, config, ext
		 FROM capability_versions
		 WHERE capability_id = $1
		 ORDER BY major DESC, minor DESC, patch DESC`, capabilityID)
	if err != nil {
		return nil, fmt.Errorf("%s - GetVersions failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	return scanVersions(rows)
}

// GetVersionsByCapabilityIDs returns all versions for the given capability IDs, keyed by capability_id.
func (r *Repository) GetVersionsByCapabilityIDs(ctx context.Context, capabilityIDs []string) (map[string][]CapabilityVersion, error) {
	if len(capabilityIDs) == 0 {
		return map[string][]CapabilityVersion{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, capability_id, major, minor, patch, prerelease, build_metadata,
		        version_string, status, deprecation_reason, deprecated_at, disabled_at,
		        description, changelog, metadata, object, created, created_by, modified, modified_by, config, ext
		 FROM capability_versions
		 WHERE capability_id = ANY($1)
		 ORDER BY capability_id, major DESC, minor DESC, patch DESC`, capabilityIDs)
	if err != nil {
		return nil, fmt.Errorf("%s - GetVersionsByCapabilityIDs failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	result := make(map[string][]CapabilityVersion)
	for rows.Next() {
		var v CapabilityVersion
		if err := rows.Scan(
			&v.ID, &v.CapabilityID, &v.Major, &v.Minor, &v.Patch,
			&v.Prerelease, &v.BuildMetadata, &v.VersionString,
			&v.Status, &v.DeprecationReason, &v.DeprecatedAt, &v.DisabledAt,
			&v.Description, &v.Changelog, &v.Metadata,
			&v.Object, &v.Created, &v.CreatedBy, &v.Modified, &v.ModifiedBy, &v.Config, &v.Ext,
		); err != nil {
			return nil, fmt.Errorf("%s - GetVersionsByCapabilityIDs scan failed: %w", repoLogPrefix, err)
		}
		result[v.CapabilityID] = append(result[v.CapabilityID], v)
	}
	return result, nil
}

// GetDefaultsBatch returns default major per capability for the given env, keyed by capability_id.
func (r *Repository) GetDefaultsBatch(ctx context.Context, capabilityIDs []string, env string) (map[string]*CapabilityDefault, error) {
	if len(capabilityIDs) == 0 {
		return map[string]*CapabilityDefault{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, capability_id, default_major, env, object, created, created_by, modified, modified_by, config, ext
		 FROM capability_defaults
		 WHERE capability_id = ANY($1) AND env = $2`, capabilityIDs, env)
	if err != nil {
		return nil, fmt.Errorf("%s - GetDefaultsBatch failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	result := make(map[string]*CapabilityDefault)
	for rows.Next() {
		var d CapabilityDefault
		if err := rows.Scan(
			&d.ID, &d.CapabilityID, &d.DefaultMajor, &d.Env,
			&d.Object, &d.Created, &d.CreatedBy, &d.Modified, &d.ModifiedBy, &d.Config, &d.Ext,
		); err != nil {
			return nil, fmt.Errorf("%s - GetDefaultsBatch scan failed: %w", repoLogPrefix, err)
		}
		result[d.CapabilityID] = &d
	}
	return result, nil
}

// GetVersionsByMajor returns versions for a specific major, ordered descending.
func (r *Repository) GetVersionsByMajor(ctx context.Context, capabilityID string, major int) ([]CapabilityVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, capability_id, major, minor, patch, prerelease, build_metadata,
		        version_string, status, deprecation_reason, deprecated_at, disabled_at,
		        description, changelog, metadata, object, created, created_by, modified, modified_by, config, ext
		 FROM capability_versions
		 WHERE capability_id = $1 AND major = $2
		 ORDER BY minor DESC, patch DESC`, capabilityID, major)
	if err != nil {
		return nil, fmt.Errorf("%s - GetVersionsByMajor failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	return scanVersions(rows)
}

// GetVersion finds a specific version.
func (r *Repository) GetVersion(ctx context.Context, params GetVersionParams) (*CapabilityVersion, error) {
	query := `SELECT id, capability_id, major, minor, patch, prerelease, build_metadata,
	                 version_string, status, deprecation_reason, deprecated_at, disabled_at,
	                 description, changelog, metadata, object, created, created_by, modified, modified_by, config, ext
	          FROM capability_versions
	          WHERE capability_id = $1 AND major = $2 AND minor = $3 AND patch = $4`
	args := []interface{}{params.CapabilityID, params.Major, params.Minor, params.Patch}

	if params.Prerelease != nil {
		query += ` AND prerelease = $5`
		args = append(args, *params.Prerelease)
	} else {
		query += ` AND prerelease IS NULL`
	}
	query += ` LIMIT 1`

	row := r.pool.QueryRow(ctx, query, args...)
	return scanVersion(row)
}

// GetVersionParams holds parameters for GetVersion.
type GetVersionParams struct {
	CapabilityID string
	Major        int
	Minor        int
	Patch        int
	Prerelease   *string
}

// UpsertVersion creates or updates a version.
func (r *Repository) UpsertVersion(ctx context.Context, params UpsertVersionParams) (*CapabilityVersion, error) {
	slog.Info(fmt.Sprintf("%s - UpsertVersion capabilityID=%s %d.%d.%d", repoLogPrefix, params.CapabilityID, params.Major, params.Minor, params.Patch))

	now := time.Now().UTC()
	metadataJSON, _ := json.Marshal(params.Metadata)
	if params.Metadata == nil {
		metadataJSON = []byte("{}")
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO capability_versions
		   (capability_id, major, minor, patch, prerelease, description, changelog, metadata, created_by, modified_by, created, modified)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10, $10)
		 ON CONFLICT (capability_id, major, minor, patch, prerelease) DO UPDATE SET
		   description = COALESCE($6, capability_versions.description),
		   changelog = COALESCE($7, capability_versions.changelog),
		   metadata = COALESCE($8, capability_versions.metadata),
		   modified = $10,
		   modified_by = $9
		 RETURNING id, capability_id, major, minor, patch, prerelease, build_metadata,
		           version_string, status, deprecation_reason, deprecated_at, disabled_at,
		           description, changelog, metadata, object, created, created_by, modified, modified_by, config, ext`,
		params.CapabilityID, params.Major, params.Minor, params.Patch,
		params.Prerelease, params.Description, params.Changelog,
		metadataJSON, params.UserID, now)

	return scanVersion(row)
}

// UpsertVersionParams holds parameters for UpsertVersion.
type UpsertVersionParams struct {
	CapabilityID string
	Major        int
	Minor        int
	Patch        int
	Prerelease   *string
	Description  *string
	Changelog    *string
	Metadata     map[string]interface{}
	UserID       string
}

// UpdateVersionStatus updates the status of a version.
func (r *Repository) UpdateVersionStatus(ctx context.Context, params UpdateVersionStatusParams) (*CapabilityVersion, error) {
	now := time.Now().UTC()

	query := `UPDATE capability_versions SET status = $1, modified = $2, modified_by = $3`
	args := []interface{}{params.Status, now, params.UserID}
	argIdx := 4

	if params.Status == "deprecated" {
		query += fmt.Sprintf(`, deprecation_reason = $%d, deprecated_at = $%d`, argIdx, argIdx+1)
		args = append(args, params.Reason, now)
		argIdx += 2
	} else if params.Status == "disabled" {
		query += fmt.Sprintf(`, deprecation_reason = $%d, disabled_at = $%d`, argIdx, argIdx+1)
		args = append(args, params.Reason, now)
		argIdx += 2
	}

	query += fmt.Sprintf(` WHERE id = $%d`, argIdx)
	args = append(args, params.VersionID)
	argIdx++

	query += ` RETURNING id, capability_id, major, minor, patch, prerelease, build_metadata,
	           version_string, status, deprecation_reason, deprecated_at, disabled_at,
	           description, changelog, metadata, object, created, created_by, modified, modified_by, config, ext`

	row := r.pool.QueryRow(ctx, query, args...)
	return scanVersion(row)
}

// UpdateVersionStatusParams holds parameters for UpdateVersionStatus.
type UpdateVersionStatusParams struct {
	VersionID string
	Status    string // "active", "deprecated", "disabled"
	Reason    *string
	UserID    string
}

// =========================================================================
// METHOD OPERATIONS
// =========================================================================

// GetMethods returns all methods for a version.
func (r *Repository) GetMethods(ctx context.Context, versionID string) ([]CapabilityMethod, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, version_id, name, description, input_schema, output_schema,
		        tags, policies, examples, modes, object, created, created_by, modified, modified_by, config, ext
		 FROM capability_methods
		 WHERE version_id = $1
		 ORDER BY name ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("%s - GetMethods failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	var methods []CapabilityMethod
	for rows.Next() {
		var m CapabilityMethod
		if err := rows.Scan(
			&m.ID, &m.VersionID, &m.Name, &m.Description,
			&m.InputSchema, &m.OutputSchema, &m.Tags, &m.Policies,
			&m.Examples, &m.Modes, &m.Object, &m.Created, &m.CreatedBy,
			&m.Modified, &m.ModifiedBy, &m.Config, &m.Ext,
		); err != nil {
			return nil, fmt.Errorf("%s - GetMethods scan failed: %w", repoLogPrefix, err)
		}
		methods = append(methods, m)
	}
	return methods, nil
}

// UpsertMethod creates or updates a method.
func (r *Repository) UpsertMethod(ctx context.Context, params UpsertMethodParams) (*CapabilityMethod, error) {
	now := time.Now().UTC()
	inputJSON, _ := json.Marshal(params.InputSchema)
	if params.InputSchema == nil {
		inputJSON = []byte("{}")
	}
	outputJSON, _ := json.Marshal(params.OutputSchema)
	if params.OutputSchema == nil {
		outputJSON = []byte("{}")
	}
	policiesJSON, _ := json.Marshal(params.Policies)
	if params.Policies == nil {
		policiesJSON = []byte("{}")
	}
	examplesJSON, _ := json.Marshal(params.Examples)
	if params.Examples == nil {
		examplesJSON = []byte("[]")
	}
	modes := params.Modes
	if len(modes) == 0 {
		modes = []string{"sync"}
	}
	tags := params.Tags
	if tags == nil {
		tags = []string{}
	}

	var m CapabilityMethod
	err := r.pool.QueryRow(ctx,
		`INSERT INTO capability_methods
		   (version_id, name, description, input_schema, output_schema, tags, policies, examples, modes,
		    created_by, modified_by, created, modified)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11, $11)
		 ON CONFLICT (version_id, name) DO UPDATE SET
		   description = COALESCE($3, capability_methods.description),
		   input_schema = COALESCE($4, capability_methods.input_schema),
		   output_schema = COALESCE($5, capability_methods.output_schema),
		   tags = COALESCE($6, capability_methods.tags),
		   policies = COALESCE($7, capability_methods.policies),
		   examples = COALESCE($8, capability_methods.examples),
		   modes = COALESCE($9, capability_methods.modes),
		   modified = $11,
		   modified_by = $10
		 RETURNING id, version_id, name, description, input_schema, output_schema,
		           tags, policies, examples, modes, object, created, created_by, modified, modified_by, config, ext`,
		params.VersionID, params.Name, params.Description,
		inputJSON, outputJSON, tags, policiesJSON, examplesJSON, modes,
		params.UserID, now,
	).Scan(
		&m.ID, &m.VersionID, &m.Name, &m.Description,
		&m.InputSchema, &m.OutputSchema, &m.Tags, &m.Policies,
		&m.Examples, &m.Modes, &m.Object, &m.Created, &m.CreatedBy,
		&m.Modified, &m.ModifiedBy, &m.Config, &m.Ext,
	)
	if err != nil {
		return nil, fmt.Errorf("%s - UpsertMethod failed: %w", repoLogPrefix, err)
	}
	return &m, nil
}

// UpsertMethodParams holds parameters for UpsertMethod.
type UpsertMethodParams struct {
	VersionID    string
	Name         string
	Description  *string
	InputSchema  map[string]interface{}
	OutputSchema map[string]interface{}
	Modes        []string
	Tags         []string
	Policies     map[string]interface{}
	Examples     []interface{}
	UserID       string
}

// DeleteMethods deletes all methods for a version.
func (r *Repository) DeleteMethods(ctx context.Context, versionID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM capability_methods WHERE version_id = $1`, versionID)
	return err
}

// =========================================================================
// DEFAULT OPERATIONS
// =========================================================================

// GetDefault returns the default major for a capability in an environment.
func (r *Repository) GetDefault(ctx context.Context, capabilityID, env string) (*CapabilityDefault, error) {
	var d CapabilityDefault
	err := r.pool.QueryRow(ctx,
		`SELECT id, capability_id, default_major, env, object, created, created_by, modified, modified_by, config, ext
		 FROM capability_defaults
		 WHERE capability_id = $1 AND env = $2
		 LIMIT 1`, capabilityID, env,
	).Scan(
		&d.ID, &d.CapabilityID, &d.DefaultMajor, &d.Env,
		&d.Object, &d.Created, &d.CreatedBy, &d.Modified, &d.ModifiedBy, &d.Config, &d.Ext,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s - GetDefault failed: %w", repoLogPrefix, err)
	}
	return &d, nil
}

// SetDefault sets the default major for a capability in an environment.
func (r *Repository) SetDefault(ctx context.Context, params SetDefaultParams) (*CapabilityDefault, error) {
	now := time.Now().UTC()

	var d CapabilityDefault
	err := r.pool.QueryRow(ctx,
		`INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by, created, modified)
		 VALUES ($1, $2, $3, $4, $4, $5, $5)
		 ON CONFLICT (capability_id, env) DO UPDATE SET
		   default_major = $2,
		   modified = $5,
		   modified_by = $4
		 RETURNING id, capability_id, default_major, env, object, created, created_by, modified, modified_by, config, ext`,
		params.CapabilityID, params.Major, params.Env, params.UserID, now,
	).Scan(
		&d.ID, &d.CapabilityID, &d.DefaultMajor, &d.Env,
		&d.Object, &d.Created, &d.CreatedBy, &d.Modified, &d.ModifiedBy, &d.Config, &d.Ext,
	)
	if err != nil {
		return nil, fmt.Errorf("%s - SetDefault failed: %w", repoLogPrefix, err)
	}
	return &d, nil
}

// SetDefaultParams holds parameters for SetDefault.
type SetDefaultParams struct {
	CapabilityID string
	Major        int
	Env          string
	UserID       string
}

// =========================================================================
// TENANT RULES OPERATIONS
// =========================================================================

// GetTenantRules returns tenant rules for a capability matching the resolution context.
func (r *Repository) GetTenantRules(ctx context.Context, capabilityID string, rctx ResolutionContext) ([]CapabilityTenantRule, error) {
	query := `SELECT id, capability_id, tenant_id, env, aud, rule_type,
	                 allowed_majors, denied_majors, required_features, priority,
	                 object, status, created_by, modified_by
	          FROM capability_tenant_rules
	          WHERE capability_id = $1`
	args := []interface{}{capabilityID}
	argIdx := 2

	if rctx.TenantID != "" {
		query += fmt.Sprintf(` AND (tenant_id IS NULL OR tenant_id = $%d)`, argIdx)
		args = append(args, rctx.TenantID)
		argIdx++
	}
	if rctx.Env != "" {
		query += fmt.Sprintf(` AND (env IS NULL OR env = $%d)`, argIdx)
		args = append(args, rctx.Env)
		argIdx++
	}

	query += ` ORDER BY priority ASC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s - GetTenantRules failed: %w", repoLogPrefix, err)
	}
	defer rows.Close()

	var rules []CapabilityTenantRule
	for rows.Next() {
		var rule CapabilityTenantRule
		if err := rows.Scan(
			&rule.ID, &rule.CapabilityID, &rule.TenantID, &rule.Env, &rule.Aud,
			&rule.RuleType, &rule.AllowedMajors, &rule.DeniedMajors,
			&rule.RequiredFeatures, &rule.Priority, &rule.Object, &rule.Status,
			&rule.CreatedBy, &rule.ModifiedBy,
		); err != nil {
			return nil, fmt.Errorf("%s - GetTenantRules scan failed: %w", repoLogPrefix, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// CheckTenantAccess checks if a tenant has access to a specific major version.
func (r *Repository) CheckTenantAccess(ctx context.Context, capabilityID string, major int, rctx ResolutionContext) (bool, string) {
	rules, err := r.GetTenantRules(ctx, capabilityID, rctx)
	if err != nil {
		slog.Error(fmt.Sprintf("%s - CheckTenantAccess failed to get rules: %v", repoLogPrefix, err))
		return true, "" // fail open
	}

	for _, rule := range rules {
		// Check feature requirements
		if len(rule.RequiredFeatures) > 0 {
			hasAll := true
			for _, f := range rule.RequiredFeatures {
				found := false
				for _, uf := range rctx.Features {
					if uf == f {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}

		if rule.RuleType == "deny" {
			if len(rule.DeniedMajors) == 0 || containsInt(rule.DeniedMajors, major) {
				return false, "Denied by tenant rule"
			}
		}

		if rule.RuleType == "allow" {
			if len(rule.AllowedMajors) > 0 && !containsInt(rule.AllowedMajors, major) {
				return false, "Major not in allowed list"
			}
		}
	}

	return true, ""
}

// =========================================================================
// INCREMENT REVISION
// =========================================================================

// IncrementRevision atomically increments the revision counter on a capability.
func (r *Repository) IncrementRevision(ctx context.Context, capabilityID string) (int, error) {
	now := time.Now().UTC()
	var revision int
	err := r.pool.QueryRow(ctx,
		`UPDATE capabilities SET revision = revision + 1, modified = $1
		 WHERE id = $2
		 RETURNING revision`, now, capabilityID).Scan(&revision)
	if err != nil {
		return 1, fmt.Errorf("%s - IncrementRevision failed: %w", repoLogPrefix, err)
	}
	return revision, nil
}

// =========================================================================
// SCAN HELPERS
// =========================================================================

func scanCapability(row pgx.Row) (*Capability, error) {
	var c Capability
	err := row.Scan(
		&c.ID, &c.App, &c.Name, &c.Description, &c.Tags, &c.Status, &c.Object, &c.Revision,
		&c.Created, &c.CreatedBy, &c.Modified, &c.ModifiedBy, &c.Config, &c.Ext,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s - scan capability failed: %w", repoLogPrefix, err)
	}
	return &c, nil
}

func scanCapabilityFromRows(rows pgx.Rows) (*Capability, error) {
	var c Capability
	err := rows.Scan(
		&c.ID, &c.App, &c.Name, &c.Description, &c.Tags, &c.Status, &c.Object, &c.Revision,
		&c.Created, &c.CreatedBy, &c.Modified, &c.ModifiedBy, &c.Config, &c.Ext,
	)
	if err != nil {
		return nil, fmt.Errorf("%s - scan capability from rows failed: %w", repoLogPrefix, err)
	}
	return &c, nil
}

func scanVersion(row pgx.Row) (*CapabilityVersion, error) {
	var v CapabilityVersion
	err := row.Scan(
		&v.ID, &v.CapabilityID, &v.Major, &v.Minor, &v.Patch,
		&v.Prerelease, &v.BuildMetadata, &v.VersionString,
		&v.Status, &v.DeprecationReason, &v.DeprecatedAt, &v.DisabledAt,
		&v.Description, &v.Changelog, &v.Metadata,
		&v.Object, &v.Created, &v.CreatedBy, &v.Modified, &v.ModifiedBy, &v.Config, &v.Ext,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s - scan version failed: %w", repoLogPrefix, err)
	}
	return &v, nil
}

func scanVersions(rows pgx.Rows) ([]CapabilityVersion, error) {
	var versions []CapabilityVersion
	for rows.Next() {
		var v CapabilityVersion
		if err := rows.Scan(
			&v.ID, &v.CapabilityID, &v.Major, &v.Minor, &v.Patch,
			&v.Prerelease, &v.BuildMetadata, &v.VersionString,
			&v.Status, &v.DeprecationReason, &v.DeprecatedAt, &v.DisabledAt,
			&v.Description, &v.Changelog, &v.Metadata,
			&v.Object, &v.Created, &v.CreatedBy, &v.Modified, &v.ModifiedBy, &v.Config, &v.Ext,
		); err != nil {
			return nil, fmt.Errorf("%s - scan versions failed: %w", repoLogPrefix, err)
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
