//go:build integration

package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dbIntegrationPrefix = "db:integration_test"

// testDBEnv returns the database URL for integration tests; skips the test if not set.
// Use platform Postgres and registry_test: create DBs once with scripts/ensure-databases.ps1, then
// set DATABASE_URL=postgres://morezero:morezero@localhost:5432/registry_test?sslmode=disable
func testDBEnv(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("db:integration_test - DATABASE_URL not set (e.g. .../registry_test; create with scripts/ensure-databases.ps1), skipping")
	}
	return url
}

// setupIntegrationDB creates a pool, runs migrations, and returns repo and cleanup.
// Caller must run from registry module root so "migrations" resolves to registry/migrations.
func setupIntegrationDB(t *testing.T) (ctx context.Context, repo *Repository, cleanup func()) {
	t.Helper()
	ctx = context.Background()
	url := testDBEnv(t)

	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("%s - NewPool failed: %v", dbIntegrationPrefix, err)
	}

	migrationPath := "migrations"
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		// When running from pkg/db, migrations are at ../../migrations
		migrationPath = filepath.Join("..", "..", "migrations")
	}
	migrationSQL, err := LoadMigrationFiles(migrationPath)
	if err != nil {
		pool.Close()
		t.Fatalf("%s - LoadMigrationFiles failed: %v", dbIntegrationPrefix, err)
	}
	if err := RunMigrations(ctx, pool, migrationSQL); err != nil {
		pool.Close()
		t.Fatalf("%s - RunMigrations failed: %v", dbIntegrationPrefix, err)
	}

	repo = NewRepository(pool)
	cleanup = func() { pool.Close() }
	return ctx, repo, cleanup
}

// setupIntegrationPool creates a pool with migrations applied, for tests that need the pool directly (e.g. RunMigrations, ClearRegistry, Seed).
func setupIntegrationPool(t *testing.T) (ctx context.Context, pool *pgxpool.Pool, cleanup func()) {
	t.Helper()
	ctx = context.Background()
	url := testDBEnv(t)

	p, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("%s - NewPool failed: %v", dbIntegrationPrefix, err)
	}

	migrationPath := "migrations"
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		migrationPath = filepath.Join("..", "..", "migrations")
	}
	migrationSQL, err := LoadMigrationFiles(migrationPath)
	if err != nil {
		p.Close()
		t.Fatalf("%s - LoadMigrationFiles failed: %v", dbIntegrationPrefix, err)
	}
	if err := RunMigrations(ctx, p, migrationSQL); err != nil {
		p.Close()
		t.Fatalf("%s - RunMigrations failed: %v", dbIntegrationPrefix, err)
	}

	cleanup = func() { p.Close() }
	return ctx, p, cleanup
}

var testUserID = "00000000-0000-0000-0000-000000000001"

func TestIntegration_NewRepository_UpsertAndGetCapability(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testapp", "integration.cap"
	desc := "Integration test capability"
	tags := []string{"integration", "test"}

	cap, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{
		App:         app,
		Name:        name,
		Description: &desc,
		Tags:        tags,
		UserID:      testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}
	if cap.ID == "" {
		t.Errorf("%s - expected non-empty ID", dbIntegrationPrefix)
	}
	if cap.App != app || cap.Name != name {
		t.Errorf("%s - app/name = %s/%s, want %s/%s", dbIntegrationPrefix, cap.App, cap.Name, app, name)
	}
	if cap.Status != "Active" {
		t.Errorf("%s - status = %q, want Active", dbIntegrationPrefix, cap.Status)
	}

	got, err := repo.GetCapability(ctx, app, name)
	if err != nil {
		t.Fatalf("%s - GetCapability failed: %v", dbIntegrationPrefix, err)
	}
	if got.ID != cap.ID {
		t.Errorf("%s - GetCapability ID = %q, want %q", dbIntegrationPrefix, got.ID, cap.ID)
	}
}

func TestIntegration_ListCapabilities(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	caps, total, err := repo.ListCapabilities(ctx, ListCapabilitiesParams{
		App:    "",
		Status: "all",
		Page:   1,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("%s - ListCapabilities failed: %v", dbIntegrationPrefix, err)
	}
	if total < 0 {
		t.Errorf("%s - total = %d, want >= 0", dbIntegrationPrefix, total)
	}
	if len(caps) > 10 {
		t.Errorf("%s - len(caps) = %d, want <= 10", dbIntegrationPrefix, len(caps))
	}
}

func TestIntegration_UpsertVersion_GetVersions_GetDefault_SetDefault(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testver", "versioned.cap"
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{
		App:    app,
		Name:   name,
		UserID: testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}

	cap, err := repo.GetCapability(ctx, app, name)
	if err != nil || cap == nil {
		t.Fatalf("%s - GetCapability failed or nil", dbIntegrationPrefix)
	}

	v, err := repo.UpsertVersion(ctx, UpsertVersionParams{
		CapabilityID: cap.ID,
		Major:        1,
		Minor:        0,
		Patch:        0,
		UserID:       testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertVersion failed: %v", dbIntegrationPrefix, err)
	}
	if v.ID == "" {
		t.Errorf("%s - version ID empty", dbIntegrationPrefix)
	}

	versions, err := repo.GetVersions(ctx, cap.ID)
	if err != nil {
		t.Fatalf("%s - GetVersions failed: %v", dbIntegrationPrefix, err)
	}
	if len(versions) < 1 {
		t.Errorf("%s - expected at least 1 version, got %d", dbIntegrationPrefix, len(versions))
	}

	// Use unique env to avoid cross-test or cross-package collision when packages run in parallel
	env := "testenv_setdefault"
	def, err := repo.GetDefault(ctx, cap.ID, env)
	if err != nil {
		t.Fatalf("%s - GetDefault failed: %v", dbIntegrationPrefix, err)
	}
	if def != nil {
		t.Errorf("%s - expected no default initially, got %+v", dbIntegrationPrefix, def)
	}

	setDef, err := repo.SetDefault(ctx, SetDefaultParams{
		CapabilityID: cap.ID,
		Major:        1,
		Env:          env,
		UserID:       testUserID,
	})
	if err != nil {
		t.Fatalf("%s - SetDefault failed: %v", dbIntegrationPrefix, err)
	}
	if setDef.DefaultMajor != 1 {
		t.Errorf("%s - SetDefault DefaultMajor = %d, want 1", dbIntegrationPrefix, setDef.DefaultMajor)
	}

	def, err = repo.GetDefault(ctx, cap.ID, env)
	if err != nil || def == nil {
		t.Fatalf("%s - GetDefault after SetDefault failed or nil", dbIntegrationPrefix)
	}
	if def.DefaultMajor != 1 {
		t.Errorf("%s - GetDefault DefaultMajor = %d, want 1", dbIntegrationPrefix, def.DefaultMajor)
	}
}

func TestIntegration_UpdateVersionStatus(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testdep", "deprecate.cap"
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{App: app, Name: name, UserID: testUserID})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}
	cap, _ := repo.GetCapability(ctx, app, name)
	v, err := repo.UpsertVersion(ctx, UpsertVersionParams{
		CapabilityID: cap.ID,
		Major:        1, Minor:        0, Patch:        0,
		UserID: testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertVersion failed: %v", dbIntegrationPrefix, err)
	}

	reason := "EOL"
	updated, err := repo.UpdateVersionStatus(ctx, UpdateVersionStatusParams{
		VersionID: v.ID,
		Status:    "deprecated",
		Reason:    &reason,
		UserID:    testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpdateVersionStatus failed: %v", dbIntegrationPrefix, err)
	}
	if updated.Status != "deprecated" {
		t.Errorf("%s - status = %q, want deprecated", dbIntegrationPrefix, updated.Status)
	}
}

func TestIntegration_UpsertMethod_GetMethods(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testmeth", "method.cap"
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{App: app, Name: name, UserID: testUserID})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}
	cap, _ := repo.GetCapability(ctx, app, name)
	v, err := repo.UpsertVersion(ctx, UpsertVersionParams{
		CapabilityID: cap.ID,
		Major:        1, Minor:        0, Patch:        0,
		UserID: testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertVersion failed: %v", dbIntegrationPrefix, err)
	}

	desc := "Do something"
	_, err = repo.UpsertMethod(ctx, UpsertMethodParams{
		VersionID:    v.ID,
		Name:         "doSomething",
		Description:  &desc,
		InputSchema:  map[string]interface{}{"type": "object"},
		OutputSchema: map[string]interface{}{"type": "object"},
		Modes:        []string{"sync"},
		UserID:       testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertMethod failed: %v", dbIntegrationPrefix, err)
	}

	methods, err := repo.GetMethods(ctx, v.ID)
	if err != nil {
		t.Fatalf("%s - GetMethods failed: %v", dbIntegrationPrefix, err)
	}
	if len(methods) != 1 {
		t.Fatalf("%s - expected 1 method, got %d", dbIntegrationPrefix, len(methods))
	}
	if methods[0].Name != "doSomething" {
		t.Errorf("%s - method name = %q, want doSomething", dbIntegrationPrefix, methods[0].Name)
	}
}

func TestIntegration_IncrementRevision(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testrev", "revision.cap"
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{App: app, Name: name, UserID: testUserID})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}
	cap, _ := repo.GetCapability(ctx, app, name)
	initialRev := cap.Revision

	rev, err := repo.IncrementRevision(ctx, cap.ID)
	if err != nil {
		t.Fatalf("%s - IncrementRevision failed: %v", dbIntegrationPrefix, err)
	}
	if rev != initialRev+1 {
		t.Errorf("%s - IncrementRevision = %d, want %d", dbIntegrationPrefix, rev, initialRev+1)
	}
}

func TestIntegration_GetCapabilityByID(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testid", "byid.cap"
	created, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{App: app, Name: name, UserID: testUserID})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}

	got, err := repo.GetCapabilityByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("%s - GetCapabilityByID failed: %v", dbIntegrationPrefix, err)
	}
	if got.ID != created.ID || got.App != app || got.Name != name {
		t.Errorf("%s - GetCapabilityByID mismatch: got %+v", dbIntegrationPrefix, got)
	}
}

func TestIntegration_ListCapabilities_WithFilters(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	// Ensure we have at least one capability from previous tests or create one
	app, name := "filterapp", "filter.cap"
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{App: app, Name: name, UserID: testUserID})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}

	caps, total, err := repo.ListCapabilities(ctx, ListCapabilitiesParams{
		App:    app,
		Status: "all",
		Page:   1,
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("%s - ListCapabilities with App filter failed: %v", dbIntegrationPrefix, err)
	}
	if total < 1 {
		t.Errorf("%s - expected total >= 1 for app %q, got %d", dbIntegrationPrefix, app, total)
	}
	found := false
	for _, c := range caps {
		if c.App == app && c.Name == name {
			found = true
			break
		}
	}
	if !found && total > 0 {
		t.Errorf("%s - expected to find %s/%s in list", dbIntegrationPrefix, app, name)
	}
}

func TestIntegration_GetVersionsByMajor(t *testing.T) {
	ctx, repo, cleanup := setupIntegrationDB(t)
	defer cleanup()

	app, name := "testmajor", "major.cap"
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{App: app, Name: name, UserID: testUserID})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}
	cap, _ := repo.GetCapability(ctx, app, name)
	_, err = repo.UpsertVersion(ctx, UpsertVersionParams{
		CapabilityID: cap.ID,
		Major:        2, Minor:        1, Patch:        0,
		UserID: testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertVersion failed: %v", dbIntegrationPrefix, err)
	}

	versions, err := repo.GetVersionsByMajor(ctx, cap.ID, 2)
	if err != nil {
		t.Fatalf("%s - GetVersionsByMajor failed: %v", dbIntegrationPrefix, err)
	}
	if len(versions) < 1 {
		t.Errorf("%s - expected at least 1 version for major 2", dbIntegrationPrefix)
	}
	if len(versions) > 0 && versions[0].Major != 2 {
		t.Errorf("%s - version major = %d, want 2", dbIntegrationPrefix, versions[0].Major)
	}
}

func TestIntegration_RunMigrations_EmptyList(t *testing.T) {
	ctx, pool, cleanup := setupIntegrationPool(t)
	defer cleanup()

	err := RunMigrations(ctx, pool, []string{})
	if err != nil {
		t.Errorf("%s - RunMigrations with empty list returned %v, want nil", dbIntegrationPrefix, err)
	}
}

func TestIntegration_ClearRegistry(t *testing.T) {
	ctx, pool, cleanup := setupIntegrationPool(t)
	defer cleanup()

	repo := NewRepository(pool)
	_, err := repo.UpsertCapability(ctx, UpsertCapabilityParams{
		App: "clearapp", Name: "clear.cap", UserID: testUserID,
	})
	if err != nil {
		t.Fatalf("%s - UpsertCapability failed: %v", dbIntegrationPrefix, err)
	}

	caps, total, err := repo.ListCapabilities(ctx, ListCapabilitiesParams{Status: "all", Page: 1, Limit: 10})
	if err != nil || total < 1 {
		t.Fatalf("%s - ListCapabilities before clear: err=%v total=%d", dbIntegrationPrefix, err, total)
	}

	err = ClearRegistry(ctx, pool)
	if err != nil {
		t.Fatalf("%s - ClearRegistry failed: %v", dbIntegrationPrefix, err)
	}

	caps, _, err = repo.ListCapabilities(ctx, ListCapabilitiesParams{Status: "all", Page: 1, Limit: 100})
	if err != nil {
		t.Fatalf("%s - ListCapabilities after clear failed: %v", dbIntegrationPrefix, err)
	}
	// ClearRegistry must have removed our capability (other packages may run in parallel and leave rows)
	for _, c := range caps {
		if c.App == "clearapp" && c.Name == "clear.cap" {
			t.Errorf("%s - after ClearRegistry expected clearapp.clear.cap to be gone, but it still exists", dbIntegrationPrefix)
		}
	}
}

func TestIntegration_SeedFromCapabilityMetadataFile(t *testing.T) {
	ctx, pool, cleanup := setupIntegrationPool(t)
	defer cleanup()

	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.json")
	content := []byte(`{
		"capability": "seedapp.seedcap",
		"major": 1,
		"version": "1.0.0",
		"status": "active",
		"description": "Seeded from test",
		"methods": {
			"run": {
				"description": "Run method",
				"modes": ["sync"],
				"inputSchema": {"type": "object"},
				"outputSchema": {"type": "object"}
			}
		}
	}`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("%s - write metadata file: %v", dbIntegrationPrefix, err)
	}

	err := SeedFromCapabilityMetadataFile(ctx, pool, path, "")
	if err != nil {
		t.Fatalf("%s - SeedFromCapabilityMetadataFile failed: %v", dbIntegrationPrefix, err)
	}

	repo := NewRepository(pool)
	cap, err := repo.GetCapability(ctx, "seedapp", "seedcap")
	if err != nil {
		t.Fatalf("%s - GetCapability after seed failed: %v", dbIntegrationPrefix, err)
	}
	if cap == nil {
		t.Fatalf("%s - expected capability after seed, got nil", dbIntegrationPrefix)
	}
	if cap.App != "seedapp" || cap.Name != "seedcap" {
		t.Errorf("%s - capability = %s.%s, want seedapp.seedcap", dbIntegrationPrefix, cap.App, cap.Name)
	}
}
