package semver

import (
	"testing"
)

func makeVersions() []VersionRecord {
	return []VersionRecord{
		{ID: "v1", Major: 3, Minor: 4, Patch: 2, Status: "active", VersionString: "3.4.2"},
		{ID: "v2", Major: 3, Minor: 3, Patch: 0, Status: "active", VersionString: "3.3.0"},
		{ID: "v3", Major: 3, Minor: 2, Patch: 1, Status: "deprecated", VersionString: "3.2.1"},
		{ID: "v4", Major: 2, Minor: 1, Patch: 0, Status: "active", VersionString: "2.1.0"},
		{ID: "v5", Major: 2, Minor: 0, Patch: 0, Status: "active", VersionString: "2.0.0"},
		{ID: "v6", Major: 1, Minor: 0, Patch: 0, Status: "disabled", VersionString: "1.0.0"},
		{ID: "v7", Major: 3, Minor: 5, Patch: 0, Prerelease: "alpha.1", Status: "active", VersionString: "3.5.0-alpha.1"},
	}
}

func TestResolveVersion_NoRange_WithDefault(t *testing.T) {
	versions := makeVersions()

	result := ResolveVersion(ResolveVersionParams{
		Versions:          versions,
		Range:             "",
		DefaultMajor:      3,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// Should return 3.4.2 (latest in major 3, non-prerelease preferred)
	if result.VersionString != "3.4.2" {
		t.Errorf("expected 3.4.2, got %s", result.VersionString)
	}
}

func TestResolveVersion_NoRange_NoDefault(t *testing.T) {
	versions := makeVersions()

	result := ResolveVersion(ResolveVersionParams{
		Versions:          versions,
		Range:             "",
		DefaultMajor:      -1,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// Should return latest in highest major (3.4.2)
	if result.VersionString != "3.4.2" {
		t.Errorf("expected 3.4.2, got %s", result.VersionString)
	}
}

func TestResolveVersion_MajorOnly(t *testing.T) {
	versions := makeVersions()

	result := ResolveVersion(ResolveVersionParams{
		Versions:          versions,
		Range:             "2",
		DefaultMajor:      -1,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.VersionString != "2.1.0" {
		t.Errorf("expected 2.1.0, got %s", result.VersionString)
	}
}

func TestResolveVersion_CaretRange(t *testing.T) {
	versions := makeVersions()

	result := ResolveVersion(ResolveVersionParams{
		Versions:          versions,
		Range:             "^3.2.0",
		DefaultMajor:      -1,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// ^3.2.0 matches >=3.2.0 <4.0.0, so 3.5.0-alpha.1 or 3.4.2
	// Should be 3.5.0-alpha.1 since it's the highest
	// Actually, semver pre-release handling: 3.5.0-alpha.1 < 3.5.0 but > 3.4.2
	// With Masterminds/semver, 3.5.0-alpha.1 satisfies ^3.2.0
	if result.Major != 3 {
		t.Errorf("expected major 3, got %d", result.Major)
	}
}

func TestResolveVersion_ExcludeDisabled(t *testing.T) {
	versions := makeVersions()

	result := ResolveVersion(ResolveVersionParams{
		Versions:          versions,
		Range:             "1",
		DefaultMajor:      -1,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	// v6 (1.0.0) is disabled, should be excluded
	if result != nil {
		t.Errorf("expected nil (disabled excluded), got %s", result.VersionString)
	}
}

func TestResolveVersion_NoMatch(t *testing.T) {
	versions := makeVersions()

	result := ResolveVersion(ResolveVersionParams{
		Versions:          versions,
		Range:             "99",
		DefaultMajor:      -1,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if result != nil {
		t.Errorf("expected nil for non-existent major, got %s", result.VersionString)
	}
}

func TestResolveVersion_EmptyVersions(t *testing.T) {
	result := ResolveVersion(ResolveVersionParams{
		Versions:          []VersionRecord{},
		Range:             "",
		DefaultMajor:      -1,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if result != nil {
		t.Errorf("expected nil for empty versions, got %v", result)
	}
}

func TestGetUniqueMajors(t *testing.T) {
	versions := makeVersions()

	majors := GetUniqueMajors(versions)

	if len(majors) != 3 {
		t.Fatalf("expected 3 unique majors, got %d", len(majors))
	}
	// Should be sorted descending: 3, 2, 1
	if majors[0] != 3 || majors[1] != 2 || majors[2] != 1 {
		t.Errorf("expected [3, 2, 1], got %v", majors)
	}
}

func TestToVersionString(t *testing.T) {
	tests := []struct {
		name       string
		major      int
		minor      int
		patch      int
		prerelease string
		want       string
	}{
		{"simple", 3, 4, 2, "", "3.4.2"},
		{"with prerelease", 3, 5, 0, "alpha.1", "3.5.0-alpha.1"},
		{"zeros", 0, 0, 0, "", "0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToVersionString(tt.major, tt.minor, tt.patch, tt.prerelease)
			if got != tt.want {
				t.Errorf("ToVersionString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSatisfiesRange(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		rangeStr string
		want     bool
	}{
		{"major-only match", "3.4.2", "3", true},
		{"major-only no match", "3.4.2", "2", false},
		{"caret match", "3.4.2", "^3.2.0", true},
		{"caret no match", "2.1.0", "^3.2.0", false},
		{"exact match", "3.4.2", "3.4.2", true},
		{"exact no match", "3.4.2", "3.4.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SatisfiesRange(tt.version, tt.rangeStr)
			if got != tt.want {
				t.Errorf("SatisfiesRange(%q, %q) = %v, want %v", tt.version, tt.rangeStr, got, tt.want)
			}
		})
	}
}
