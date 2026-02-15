package registry

import (
	"context"
	"fmt"
	"testing"
)

const upsertTestPrefix = "registry:upsert_test"

func TestValidateUpsertInput_InvalidApp(t *testing.T) {
	tests := []struct {
		app  string
		name string
	}{
		{"Invalid", "validname"},
		{"UPPER", "validname"},
		{"has_space", "validname"},
		{"", "validname"},
	}
	for _, tt := range tests {
		err := validateUpsertInput(&UpsertInput{
			App: tt.app, Name: tt.name,
			Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
			Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
		})
		if err == nil {
			t.Errorf("%s - validateUpsertInput(app=%q) expected error", upsertTestPrefix, tt.app)
		}
		if err != nil && err.Code != "INVALID_ARGUMENT" {
			t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
		}
	}
}

func TestValidateUpsertInput_InvalidName(t *testing.T) {
	tests := []string{"1invalid", "inv lid", ""}
	for _, name := range tests {
		err := validateUpsertInput(&UpsertInput{
			App: "validapp", Name: name,
			Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
			Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
		})
		if err == nil {
			t.Errorf("%s - validateUpsertInput(name=%q) expected error", upsertTestPrefix, name)
		}
		if err != nil && err.Code != "INVALID_ARGUMENT" {
			t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
		}
	}
}

func TestValidateUpsertInput_VersionBounds(t *testing.T) {
	tests := []struct {
		major, minor, patch int
		expectErr           bool
	}{
		{-1, 0, 0, true},
		{0, -1, 0, true},
		{0, 0, -1, true},
		{10000, 0, 0, true},
		{0, 10000, 0, true},
		{0, 0, 10000, true},
		{0, 0, 0, false},
		{1, 2, 3, false},
		{9999, 9999, 9999, false},
	}
	for _, tt := range tests {
		err := validateUpsertInput(&UpsertInput{
			App: "validapp", Name: "validname",
			Version: VersionInput{Major: tt.major, Minor: tt.minor, Patch: tt.patch},
			Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
		})
		if tt.expectErr && err == nil {
			t.Errorf("%s - version %d.%d.%d expected error", upsertTestPrefix, tt.major, tt.minor, tt.patch)
		}
		if !tt.expectErr && err != nil {
			t.Errorf("%s - version %d.%d.%d unexpected error: %v", upsertTestPrefix, tt.major, tt.minor, tt.patch, err)
		}
	}
}

func TestValidateUpsertInput_ZeroMethods(t *testing.T) {
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{},
	})
	if err == nil {
		t.Fatalf("%s - expected error for zero methods", upsertTestPrefix)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
	if err.Message != "at least one method is required" {
		t.Errorf("%s - Message = %q", upsertTestPrefix, err.Message)
	}
}

func TestValidateUpsertInput_TooManyMethods(t *testing.T) {
	methods := make([]MethodDefinition, maxUpsertMethods+1)
	for i := range methods {
		methods[i] = MethodDefinition{Name: fmt.Sprintf("run%d", i), Modes: []string{"sync"}}
	}
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: methods,
	})
	if err == nil {
		t.Fatalf("%s - expected error for too many methods", upsertTestPrefix)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
}

func TestValidateUpsertInput_InvalidMethodName(t *testing.T) {
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "1invalid", Modes: []string{"sync"}}},
	})
	if err == nil {
		t.Fatalf("%s - expected error for invalid method name", upsertTestPrefix)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
}

func TestValidateUpsertInput_Valid(t *testing.T) {
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "valid.name",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	})
	if err != nil {
		t.Errorf("%s - unexpected error for valid input: %v", upsertTestPrefix, err)
	}
}

func TestUpsert_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	input := &UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	}
	_, err := reg.Upsert(ctx, input, "test-user")
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", upsertTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", upsertTestPrefix, err)
	}
}

func TestValidateUpsertInput_MetadataTooLarge(t *testing.T) {
	// maxMetadataBytes is 64KB; build a metadata map that marshals to > 64KB
	bigBlob := make([]byte, maxMetadataBytes+1)
	for i := range bigBlob {
		bigBlob[i] = 'x'
	}
	meta := map[string]interface{}{"data": string(bigBlob)}
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0, Metadata: meta},
		Methods: []MethodDefinition{{Name: "run", Modes: []string{"sync"}}},
	})
	if err == nil {
		t.Fatalf("%s - expected error for metadata exceeding %d bytes", upsertTestPrefix, maxMetadataBytes)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
}

func TestValidateUpsertInput_InputSchemaTooLarge(t *testing.T) {
	bigBlob := make([]byte, maxMethodSchemaBytes+1)
	for i := range bigBlob {
		bigBlob[i] = 'x'
	}
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{
			Name:         "run",
			Modes:        []string{"sync"},
			InputSchema:  map[string]interface{}{"big": string(bigBlob)},
			OutputSchema: map[string]interface{}{"type": "object"},
		}},
	})
	if err == nil {
		t.Fatalf("%s - expected error for inputSchema exceeding %d bytes", upsertTestPrefix, maxMethodSchemaBytes)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
}

func TestValidateUpsertInput_OutputSchemaTooLarge(t *testing.T) {
	bigBlob := make([]byte, maxMethodSchemaBytes+1)
	for i := range bigBlob {
		bigBlob[i] = 'x'
	}
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{
			Name:         "run",
			Modes:        []string{"sync"},
			InputSchema:  map[string]interface{}{"type": "object"},
			OutputSchema: map[string]interface{}{"big": string(bigBlob)},
		}},
	})
	if err == nil {
		t.Fatalf("%s - expected error for outputSchema exceeding %d bytes", upsertTestPrefix, maxMethodSchemaBytes)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
}

func TestValidateUpsertInput_ExamplesTooLarge(t *testing.T) {
	bigBlob := make([]byte, maxMethodExamplesBytes+1)
	for i := range bigBlob {
		bigBlob[i] = 'x'
	}
	err := validateUpsertInput(&UpsertInput{
		App: "validapp", Name: "validname",
		Version: VersionInput{Major: 1, Minor: 0, Patch: 0},
		Methods: []MethodDefinition{{
			Name:     "run",
			Modes:    []string{"sync"},
			Examples: []interface{}{map[string]interface{}{"data": string(bigBlob)}},
		}},
	})
	if err == nil {
		t.Fatalf("%s - expected error for examples exceeding %d bytes", upsertTestPrefix, maxMethodExamplesBytes)
	}
	if err.Code != "INVALID_ARGUMENT" {
		t.Errorf("%s - Code = %q, want INVALID_ARGUMENT", upsertTestPrefix, err.Code)
	}
}
