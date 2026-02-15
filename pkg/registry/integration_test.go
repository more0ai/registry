//go:build integration

package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/events"
)

const regIntegrationPrefix = "registry:integration_test"

// testUserID is a valid UUID for created_by/modified_by (DB columns are UUID type).
const testUserID = "00000000-0000-0000-0000-000000000002"

func testDBEnv(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("registry:integration_test - DATABASE_URL not set, skipping")
	}
	return url
}

func setupRegistry(t *testing.T) (ctx context.Context, reg *Registry, cleanup func()) {
	t.Helper()
	ctx = context.Background()
	url := testDBEnv(t)

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("%s - NewPool failed: %v", regIntegrationPrefix, err)
	}

	migrationPath := "migrations"
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		migrationPath = filepath.Join("..", "..", "migrations")
	}
	migrationSQL, err := db.LoadMigrationFiles(migrationPath)
	if err != nil {
		pool.Close()
		t.Fatalf("%s - LoadMigrationFiles failed: %v", regIntegrationPrefix, err)
	}
	if err := db.RunMigrations(ctx, pool, migrationSQL); err != nil {
		pool.Close()
		t.Fatalf("%s - RunMigrations failed: %v", regIntegrationPrefix, err)
	}

	repo := db.NewRepository(pool)
	cfg := DefaultConfig()
	cfg.NatsUrl = "nats://127.0.0.1:4222"
	reg = NewRegistry(NewRegistryParams{
		Repo:      repo,
		Publisher: &events.NoOpPublisher{},
		Config:    cfg,
	})
	cleanup = func() { reg.Close(); pool.Close() }
	return ctx, reg, cleanup
}

func TestIntegration_Resolve_Success(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	// Upsert a capability with default
	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "resolve.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
		SetAsDefault: true,
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	out, err := reg.Resolve(ctx, &ResolveInput{Cap: "intg.resolve.cap"})
	if err != nil {
		t.Fatalf("%s - Resolve failed: %v", regIntegrationPrefix, err)
	}
	if out.Subject != "cap.intg.resolve_cap.v1" {
		t.Errorf("%s - Subject = %q, want cap.intg.resolve_cap.v1", regIntegrationPrefix, out.Subject)
	}
	if out.ResolvedVersion != "1.0.0" {
		t.Errorf("%s - ResolvedVersion = %q, want 1.0.0", regIntegrationPrefix, out.ResolvedVersion)
	}
	if out.NatsUrl == "" {
		t.Errorf("%s - NatsUrl should be set", regIntegrationPrefix)
	}
}

func TestIntegration_Resolve_NotFound(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Resolve(ctx, &ResolveInput{Cap: "nonexistent.cap"})
	if err == nil {
		t.Fatal("registry:integration_test - expected error for nonexistent cap")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "NOT_FOUND" {
		t.Errorf("registry:integration_test - expected NOT_FOUND, got %v", err)
	}
}

func TestIntegration_Resolve_InvalidCapRef(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Resolve(ctx, &ResolveInput{Cap: "invalid-ref-bad"})
	if err == nil {
		t.Fatal("registry:integration_test - expected error for invalid cap ref")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INVALID_ARGUMENT" {
		t.Errorf("registry:integration_test - expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestIntegration_Resolve_RemoteAlias_FederationPoolNil(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()
	reg.federationPool = nil

	_, err := reg.Resolve(ctx, &ResolveInput{Cap: "@other/app.cap"})
	if err == nil {
		t.Fatal("registry:integration_test - expected error when federation pool is nil")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("registry:integration_test - expected INTERNAL_ERROR, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "Federation pool") {
		t.Errorf("registry:integration_test - error should mention federation pool, got %v", err)
	}
}

func TestIntegration_ListMajors_IncludeInactive(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "inactive.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}
	_, err = reg.Disable(ctx, &DisableInput{Cap: "intg.inactive.cap", Reason: "Test"}, testUserID)
	if err != nil {
		t.Fatalf("%s - Disable failed: %v", regIntegrationPrefix, err)
	}

	// Exclude inactive by default
	majors, err := reg.ListMajors(ctx, &ListMajorsInput{Cap: "intg.inactive.cap"})
	if err != nil {
		t.Fatalf("%s - ListMajors failed: %v", regIntegrationPrefix, err)
	}
	if len(majors.Majors) != 0 {
		t.Errorf("registry:integration_test - without IncludeInactive expected 0 majors (disabled filtered), got %d", len(majors.Majors))
	}

	// Include inactive
	majors, err = reg.ListMajors(ctx, &ListMajorsInput{Cap: "intg.inactive.cap", IncludeInactive: true})
	if err != nil {
		t.Fatalf("%s - ListMajors IncludeInactive failed: %v", regIntegrationPrefix, err)
	}
	if len(majors.Majors) != 1 {
		t.Errorf("registry:integration_test - with IncludeInactive expected 1 major, got %d", len(majors.Majors))
	}
	if len(majors.Majors) > 0 && majors.Majors[0].Status != "disabled" {
		t.Errorf("registry:integration_test - expected status disabled, got %q", majors.Majors[0].Status)
	}
}

func TestIntegration_Discover_Success(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "discover.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
		SetAsDefault: true,
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	out, err := reg.Discover(ctx, &DiscoverInput{Status: "all", Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("%s - Discover failed: %v", regIntegrationPrefix, err)
	}
	if out.Pagination.Total < 1 {
		t.Errorf("%s - expected at least 1 capability, got total=%d", regIntegrationPrefix, out.Pagination.Total)
	}
	found := false
	for _, c := range out.Capabilities {
		if c.Cap == "intg.discover.cap" {
			found = true
			break
		}
	}
	if !found && out.Pagination.Total > 0 {
		t.Error("registry:integration_test - expected to find intg.discover.cap in Discover result")
	}
}

func TestIntegration_Describe_Success(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "describe.cap",
		Description: "Integration describe test",
		Version: VersionInput{Major: 2, Minor: 1, Patch: 0, Description: "v2.1", Changelog: "Changes"},
		Methods: []MethodDefinition{
			{Name: "run", Description: "Run it", Modes: []string{"sync"}, InputSchema: map[string]interface{}{"type": "object"}, OutputSchema: map[string]interface{}{"type": "object"}},
		},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	out, err := reg.Describe(ctx, &DescribeInput{Cap: "intg.describe.cap"})
	if err != nil {
		t.Fatalf("%s - Describe failed: %v", regIntegrationPrefix, err)
	}
	if out.Cap != "intg.describe.cap" {
		t.Errorf("%s - Cap = %q, want intg.describe.cap", regIntegrationPrefix, out.Cap)
	}
	if out.Version != "2.1.0" {
		t.Errorf("%s - Version = %q, want 2.1.0", regIntegrationPrefix, out.Version)
	}
	if out.Description != "Integration describe test" {
		t.Errorf("%s - Description = %q", regIntegrationPrefix, out.Description)
	}
	if len(out.Methods) != 1 || out.Methods[0].Name != "run" {
		t.Errorf("%s - expected one method run, got %d methods", regIntegrationPrefix, len(out.Methods))
	}
}

func TestIntegration_Describe_ByMajor(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "describe2.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}
	_, err = reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "describe2.cap",
		Version: VersionInput{Major: 2, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert v2 failed: %v", regIntegrationPrefix, err)
	}

	major := 2
	out, err := reg.Describe(ctx, &DescribeInput{Cap: "intg.describe2.cap", Major: &major})
	if err != nil {
		t.Fatalf("%s - Describe by major failed: %v", regIntegrationPrefix, err)
	}
	if out.Major != 2 || out.Version != "2.0.0" {
		t.Errorf("%s - expected major 2 version 2.0.0, got %d %s", regIntegrationPrefix, out.Major, out.Version)
	}
}

func TestIntegration_Describe_NotFound(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Describe(ctx, &DescribeInput{Cap: "nonexistent.cap"})
	if err == nil {
		t.Fatal("registry:integration_test - expected error for nonexistent cap")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "NOT_FOUND" {
		t.Errorf("registry:integration_test - expected NOT_FOUND, got %v", err)
	}
}

func TestIntegration_Describe_VersionNotFound(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "vernotfound.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	_, err = reg.Describe(ctx, &DescribeInput{Cap: "intg.vernotfound.cap", Version: "99.0.0"})
	if err == nil {
		t.Fatal("registry:integration_test - expected error for non-existent version")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "NOT_FOUND" {
		t.Errorf("registry:integration_test - expected NOT_FOUND, got %v", err)
	}
}

func TestIntegration_Upsert_FullFlow(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	out, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "upsert.cap",
		Description: "Upsert test",
		Tags: []string{"tag1"},
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0, Description: "Initial", Changelog: "First"},
		Methods: []MethodDefinition{
			{Name: "run", Description: "Run", Modes: []string{"sync"}, InputSchema: map[string]interface{}{"type": "object"}, OutputSchema: map[string]interface{}{"type": "object"}},
		},
		SetAsDefault: true,
		Env:          "production",
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}
	if out.Action != "created" {
		t.Errorf("%s - Action = %q, want created", regIntegrationPrefix, out.Action)
	}
	if out.Cap != "intg.upsert.cap" || out.Version != "1.0.0" {
		t.Errorf("%s - Cap/Version = %s %s", regIntegrationPrefix, out.Cap, out.Version)
	}

	// Update same version
	out2, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "upsert.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert update failed: %v", regIntegrationPrefix, err)
	}
	if out2.Action != "updated" {
		t.Errorf("%s - second Upsert Action = %q, want updated", regIntegrationPrefix, out2.Action)
	}
}

func TestIntegration_SetDefaultMajor_ListMajors(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "majors.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}
	_, err = reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "majors.cap",
		Version: VersionInput{Major: 2, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert v2 failed: %v", regIntegrationPrefix, err)
	}

	_, err = reg.SetDefaultMajor(ctx, &SetDefaultMajorInput{Cap: "intg.majors.cap", Major: 2, Env: "production"}, testUserID)
	if err != nil {
		t.Fatalf("%s - SetDefaultMajor failed: %v", regIntegrationPrefix, err)
	}

	majors, err := reg.ListMajors(ctx, &ListMajorsInput{Cap: "intg.majors.cap"})
	if err != nil {
		t.Fatalf("%s - ListMajors failed: %v", regIntegrationPrefix, err)
	}
	if len(majors.Majors) < 2 {
		t.Errorf("%s - expected at least 2 majors, got %d", regIntegrationPrefix, len(majors.Majors))
	}
	var defaultMajor *MajorInfo
	for i := range majors.Majors {
		if majors.Majors[i].IsDefault {
			defaultMajor = &majors.Majors[i]
			break
		}
	}
	if defaultMajor == nil || defaultMajor.Major != 2 {
		t.Errorf("registry:integration_test - expected default major 2, got %v", defaultMajor)
	}
}

func TestIntegration_Deprecate_Disable(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "dep.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	depOut, err := reg.Deprecate(ctx, &DeprecateInput{Cap: "intg.dep.cap", Reason: "EOL"}, testUserID)
	if err != nil {
		t.Fatalf("%s - Deprecate failed: %v", regIntegrationPrefix, err)
	}
	if !depOut.Success || len(depOut.AffectedVersions) < 1 {
		t.Errorf("registry:integration_test - Deprecate expected success and affected versions")
	}

	disOut, err := reg.Disable(ctx, &DisableInput{Cap: "intg.dep.cap", Reason: "Removed"}, testUserID)
	if err != nil {
		t.Fatalf("%s - Disable failed: %v", regIntegrationPrefix, err)
	}
	if !disOut.Success {
		t.Error("registry:integration_test - Disable expected success")
	}
}

func TestIntegration_GetBootstrapCapabilities_WithData(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "bootstrap.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
		SetAsDefault: true,
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	caps, err := reg.GetBootstrapCapabilities(ctx, "production", true, false)
	if err != nil {
		t.Fatalf("%s - GetBootstrapCapabilities failed: %v", regIntegrationPrefix, err)
	}
	if len(caps) < 1 {
		t.Errorf("%s - expected at least 1 bootstrap capability, got %d", regIntegrationPrefix, len(caps))
	}
	ro, ok := caps["intg.bootstrap.cap"]
	if !ok {
		t.Error("registry:integration_test - expected intg.bootstrap.cap in bootstrap capabilities")
	}
	if ok && len(ro.Methods) < 1 {
		t.Errorf("registry:integration_test - expected methods in bootstrap output")
	}
}

func TestIntegration_Health_WithRepo(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	out := reg.Health(ctx)
	if out.Status != "healthy" {
		t.Errorf("registry:integration_test - Health status = %q, want healthy", out.Status)
	}
	if !out.Checks.Database {
		t.Error("registry:integration_test - Health Database check should be true")
	}
}

func TestIntegration_Resolve_IncludeMethodsAndSchemas(t *testing.T) {
	ctx, reg, cleanup := setupRegistry(t)
	defer cleanup()

	_, err := reg.Upsert(ctx, &UpsertInput{
		App: "intg", Name: "schemas.cap",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{
			{Name: "run", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"x": map[string]interface{}{"type": "string"}}}, OutputSchema: map[string]interface{}{"type": "object"}},
		},
		SetAsDefault: true,
	}, testUserID)
	if err != nil {
		t.Fatalf("%s - Upsert failed: %v", regIntegrationPrefix, err)
	}

	out, err := reg.Resolve(ctx, &ResolveInput{Cap: "intg.schemas.cap", IncludeMethods: true, IncludeSchemas: true})
	if err != nil {
		t.Fatalf("%s - Resolve with methods/schemas failed: %v", regIntegrationPrefix, err)
	}
	if len(out.Methods) < 1 {
		t.Errorf("registry:integration_test - expected methods in resolve output")
	}
	if out.Schemas == nil || out.Schemas["run"].Input == nil {
		t.Error("registry:integration_test - expected schemas in resolve output")
	}
}
