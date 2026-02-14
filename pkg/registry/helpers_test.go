package registry

import (
	"testing"

	"github.com/morezero/capabilities-registry/pkg/db"
)

func TestDbVersionsToRecords(t *testing.T) {
	pre := "alpha.1"
	versions := []db.CapabilityVersion{
		{ID: "v1", Major: 3, Minor: 4, Patch: 2, Prerelease: nil, Status: "active"},
		{ID: "v2", Major: 2, Minor: 1, Patch: 0, Prerelease: &pre, Status: "deprecated"},
		{ID: "v3", Major: 1, Minor: 0, Patch: 0, Prerelease: nil, Status: "disabled"},
	}

	records := dbVersionsToRecords(versions)

	if len(records) != 3 {
		t.Fatalf("registry:helpers_test - expected 3 records, got %d", len(records))
	}

	// Check first record
	if records[0].ID != "v1" {
		t.Errorf("registry:helpers_test - records[0].ID = %q, want %q", records[0].ID, "v1")
	}
	if records[0].Major != 3 || records[0].Minor != 4 || records[0].Patch != 2 {
		t.Errorf("registry:helpers_test - records[0] version = %d.%d.%d, want 3.4.2",
			records[0].Major, records[0].Minor, records[0].Patch)
	}
	if records[0].Prerelease != "" {
		t.Errorf("registry:helpers_test - records[0].Prerelease = %q, want empty", records[0].Prerelease)
	}
	if records[0].VersionString != "3.4.2" {
		t.Errorf("registry:helpers_test - records[0].VersionString = %q, want %q", records[0].VersionString, "3.4.2")
	}
	if records[0].Status != "active" {
		t.Errorf("registry:helpers_test - records[0].Status = %q, want %q", records[0].Status, "active")
	}

	// Check record with prerelease
	if records[1].Prerelease != "alpha.1" {
		t.Errorf("registry:helpers_test - records[1].Prerelease = %q, want %q", records[1].Prerelease, "alpha.1")
	}
	if records[1].VersionString != "2.1.0-alpha.1" {
		t.Errorf("registry:helpers_test - records[1].VersionString = %q, want %q", records[1].VersionString, "2.1.0-alpha.1")
	}
}

func TestDbVersionsToRecords_Empty(t *testing.T) {
	records := dbVersionsToRecords([]db.CapabilityVersion{})
	if len(records) != 0 {
		t.Errorf("registry:helpers_test - expected 0 records for empty input, got %d", len(records))
	}
}

func TestOrDefault(t *testing.T) {
	tests := []struct {
		name string
		s    string
		def  string
		want string
	}{
		{"non-empty string", "hello", "default", "hello"},
		{"empty string returns default", "", "default", "default"},
		{"both empty", "", "", ""},
		{"default ignored when non-empty", "value", "", "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orDefault(tt.s, tt.def)
			if got != tt.want {
				t.Errorf("registry:helpers_test - orDefault(%q, %q) = %q, want %q", tt.s, tt.def, got, tt.want)
			}
		})
	}
}

func TestPtrStringOr(t *testing.T) {
	value := "hello"

	tests := []struct {
		name string
		p    *string
		def  string
		want string
	}{
		{"non-nil pointer", &value, "default", "hello"},
		{"nil pointer returns default", nil, "default", "default"},
		{"nil with empty default", nil, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ptrStringOr(tt.p, tt.def)
			if got != tt.want {
				t.Errorf("registry:helpers_test - ptrStringOr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJsonBytesToMap(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantKeys []string
		wantLen  int
	}{
		{
			name:     "valid JSON object",
			data:     []byte(`{"key":"value","num":42}`),
			wantKeys: []string{"key", "num"},
			wantLen:  2,
		},
		{
			name:    "nil data returns empty map",
			data:    nil,
			wantLen: 0,
		},
		{
			name:    "invalid JSON returns empty map",
			data:    []byte(`{invalid}`),
			wantLen: 0,
		},
		{
			name:    "empty object",
			data:    []byte(`{}`),
			wantLen: 0,
		},
		{
			name:     "nested object",
			data:     []byte(`{"outer":{"inner":true}}`),
			wantKeys: []string{"outer"},
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonBytesToMap(tt.data)
			if len(got) != tt.wantLen {
				t.Errorf("registry:helpers_test - jsonBytesToMap() len = %d, want %d", len(got), tt.wantLen)
			}
			for _, key := range tt.wantKeys {
				if _, ok := got[key]; !ok {
					t.Errorf("registry:helpers_test - jsonBytesToMap() missing key %q", key)
				}
			}
		})
	}
}

func TestJsonBytesToSlice(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantLen int
	}{
		{
			name:    "valid JSON array",
			data:    []byte(`[1, 2, 3]`),
			wantLen: 3,
		},
		{
			name:    "nil data returns empty slice",
			data:    nil,
			wantLen: 0,
		},
		{
			name:    "invalid JSON returns empty slice",
			data:    []byte(`[invalid`),
			wantLen: 0,
		},
		{
			name:    "empty array",
			data:    []byte(`[]`),
			wantLen: 0,
		},
		{
			name:    "array of objects",
			data:    []byte(`[{"a":1},{"b":2}]`),
			wantLen: 2,
		},
		{
			name:    "array of strings",
			data:    []byte(`["hello","world"]`),
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonBytesToSlice(tt.data)
			if len(got) != tt.wantLen {
				t.Errorf("registry:helpers_test - jsonBytesToSlice() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestRegistryError_Error(t *testing.T) {
	tests := []struct {
		name string
		code string
		msg  string
		want string
	}{
		{
			name: "not found",
			code: "NOT_FOUND",
			msg:  "Capability not found",
			want: "NOT_FOUND: Capability not found",
		},
		{
			name: "internal error",
			code: "INTERNAL_ERROR",
			msg:  "database connection failed",
			want: "INTERNAL_ERROR: database connection failed",
		},
		{
			name: "invalid argument",
			code: "INVALID_ARGUMENT",
			msg:  "missing required field",
			want: "INVALID_ARGUMENT: missing required field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RegistryError{Code: tt.code, Message: tt.msg}
			if err.Error() != tt.want {
				t.Errorf("registry:helpers_test - Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRegistryError_WithDetails(t *testing.T) {
	err := &RegistryError{
		Code:    "VALIDATION_ERROR",
		Message: "invalid input",
		Details: map[string]string{"field": "name"},
	}

	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("registry:helpers_test - Code = %q, want %q", err.Code, "VALIDATION_ERROR")
	}
	if err.Details == nil {
		t.Error("registry:helpers_test - expected Details to be non-nil")
	}

	details, ok := err.Details.(map[string]string)
	if !ok {
		t.Fatalf("registry:helpers_test - expected Details to be map[string]string, got %T", err.Details)
	}
	if details["field"] != "name" {
		t.Errorf("registry:helpers_test - Details[field] = %q, want %q", details["field"], "name")
	}
}
