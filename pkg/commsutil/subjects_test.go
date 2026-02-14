package commsutil

import "testing"

func TestBuildChangeSubject(t *testing.T) {
	tests := []struct {
		name       string
		app        string
		capability string
		want       string
	}{
		{"basic", "more0", "doc.ingest", "registry.changed.more0.doc.ingest"},
		{"system", "system", "registry", "registry.changed.system.registry"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildChangeSubject(tt.app, tt.capability)
			if got != tt.want {
				t.Errorf("BuildChangeSubject(%q, %q) = %q, want %q", tt.app, tt.capability, got, tt.want)
			}
		})
	}
}

func TestBuildCapabilitySubject(t *testing.T) {
	tests := []struct {
		name  string
		app   string
		capN  string
		major int
		want  string
	}{
		{"simple", "more0", "registry", 1, "cap.more0.registry.v1"},
		{"dotted name", "more0", "doc.ingest", 3, "cap.more0.doc_ingest.v3"},
		{"system", "system", "auth", 2, "cap.system.auth.v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCapabilitySubject(tt.app, tt.capN, tt.major)
			if got != tt.want {
				t.Errorf("BuildCapabilitySubject(%q, %q, %d) = %q, want %q", tt.app, tt.capN, tt.major, got, tt.want)
			}
		})
	}
}
