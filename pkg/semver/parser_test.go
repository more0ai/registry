package semver

import (
	"testing"
)

func TestParseCapabilityRef_BasicFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantApp   string
		wantName  string
		wantRange string
		wantFull  string
		wantErr   bool
	}{
		{
			name:      "no version",
			input:     "more0.doc.ingest",
			wantApp:   "more0",
			wantName:  "doc.ingest",
			wantRange: "",
			wantFull:  "more0.doc.ingest",
		},
		{
			name:      "major only",
			input:     "more0.doc.ingest@3",
			wantApp:   "more0",
			wantName:  "doc.ingest",
			wantRange: "3",
			wantFull:  "more0.doc.ingest",
		},
		{
			name:      "exact version",
			input:     "more0.doc.ingest@3.2.1",
			wantApp:   "more0",
			wantName:  "doc.ingest",
			wantRange: "3.2.1",
			wantFull:  "more0.doc.ingest",
		},
		{
			name:      "caret range",
			input:     "more0.doc.ingest@^3.2.0",
			wantApp:   "more0",
			wantName:  "doc.ingest",
			wantRange: "^3.2.0",
			wantFull:  "more0.doc.ingest",
		},
		{
			name:      "tilde range",
			input:     "more0.doc.ingest@~3.2.0",
			wantApp:   "more0",
			wantName:  "doc.ingest",
			wantRange: "~3.2.0",
			wantFull:  "more0.doc.ingest",
		},
		{
			name:      "simple name",
			input:     "more0.registry",
			wantApp:   "more0",
			wantName:  "registry",
			wantRange: "",
			wantFull:  "more0.registry",
		},
		{
			name:      "system capability",
			input:     "system.registry@1",
			wantApp:   "system",
			wantName:  "registry",
			wantRange: "1",
			wantFull:  "system.registry",
		},
		{
			name:    "missing app",
			input:   "registry",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:      "trimmed whitespace",
			input:     "  more0.doc.ingest@3  ",
			wantApp:   "more0",
			wantName:  "doc.ingest",
			wantRange: "3",
			wantFull:  "more0.doc.ingest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCapabilityRef(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.App != tt.wantApp {
				t.Errorf("App = %q, want %q", result.App, tt.wantApp)
			}
			if result.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", result.Name, tt.wantName)
			}
			if result.Range != tt.wantRange {
				t.Errorf("Range = %q, want %q", result.Range, tt.wantRange)
			}
			if result.Full != tt.wantFull {
				t.Errorf("Full = %q, want %q", result.Full, tt.wantFull)
			}
		})
	}
}

func TestIsMajorOnly(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"3", true},
		{"10", true},
		{"0", true},
		{"3.2.0", false},
		{"^3.2.0", false},
		{"~3.2.0", false},
		{"", false},
		{"abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsMajorOnly(tt.input)
			if got != tt.want {
				t.Errorf("IsMajorOnly(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsExactVersion(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"3.2.1", true},
		{"0.0.0", true},
		{"1.2.3-alpha.1", true},
		{"1.2.3+build.123", true},
		{"3", false},
		{"3.2", false},
		{"^3.2.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsExactVersion(tt.input)
			if got != tt.want {
				t.Errorf("IsExactVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateCapabilityName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "registry", true},
		{"dotted", "doc.ingest", true},
		{"hyphen", "my-cap", true},
		{"underscore", "my_cap", true},
		{"starts with digit", "3cap", false},
		{"special chars", "cap@1", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateCapabilityName(tt.input)
			if got != tt.want {
				t.Errorf("ValidateCapabilityName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateAppName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "more0", true},
		{"hyphen", "my-app", true},
		{"uppercase", "More0", false},
		{"underscore", "my_app", false},
		{"starts with digit", "3app", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateAppName(tt.input)
			if got != tt.want {
				t.Errorf("ValidateAppName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildCapabilityString(t *testing.T) {
	tests := []struct {
		name   string
		params BuildCapabilityParams
		want   string
	}{
		{
			name:   "no version",
			params: BuildCapabilityParams{App: "more0", Name: "doc.ingest"},
			want:   "more0.doc.ingest",
		},
		{
			name:   "with version",
			params: BuildCapabilityParams{App: "more0", Name: "doc.ingest", Version: "3.2.1"},
			want:   "more0.doc.ingest@3.2.1",
		},
		{
			name:   "with range",
			params: BuildCapabilityParams{App: "more0", Name: "registry", Version: "^1.0.0"},
			want:   "more0.registry@^1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCapabilityString(tt.params)
			if got != tt.want {
				t.Errorf("BuildCapabilityString() = %q, want %q", got, tt.want)
			}
		})
	}
}
