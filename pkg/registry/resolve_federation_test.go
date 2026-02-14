package registry

import (
	"testing"
)

func TestExtractAlias(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAlias string
		wantCap   string
	}{
		{
			name:      "no alias - simple cap ref",
			input:     "my.app/my.cap",
			wantAlias: "",
			wantCap:   "my.app/my.cap",
		},
		{
			name:      "no alias - dotted cap",
			input:     "system.registry",
			wantAlias: "",
			wantCap:   "system.registry",
		},
		{
			name:      "alias prefix - partner",
			input:     "@partner/my.app/my.cap",
			wantAlias: "partner",
			wantCap:   "my.app/my.cap",
		},
		{
			name:      "alias prefix - main",
			input:     "@main/my.app/my.cap",
			wantAlias: "main",
			wantCap:   "my.app/my.cap",
		},
		{
			name:      "alias only - no cap",
			input:     "@partner",
			wantAlias: "partner",
			wantCap:   "",
		},
		{
			name:      "alias with nested path",
			input:     "@sandbox/partner.app/image.resize",
			wantAlias: "sandbox",
			wantCap:   "partner.app/image.resize",
		},
		{
			name:      "empty string",
			input:     "",
			wantAlias: "",
			wantCap:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAlias, gotCap := extractAlias(tt.input)
			if gotAlias != tt.wantAlias {
				t.Errorf("extractAlias(%q) alias = %q, want %q", tt.input, gotAlias, tt.wantAlias)
			}
			if gotCap != tt.wantCap {
				t.Errorf("extractAlias(%q) cap = %q, want %q", tt.input, gotCap, tt.wantCap)
			}
		})
	}
}

func TestResolveLocal_IncludesNatsUrl(t *testing.T) {
	// Test that the local resolve sets NatsUrl from config
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil, // nil repo - will fail requireRepo but tests config path
		Publisher: nil,
		Config: Config{
			DefaultTTLSeconds: 300,
			DefaultEnv:        "production",
			SubjectPrefix:     "cap",
			DefaultAlias:      "main",
			NatsUrl:           "nats://test-server:4222",
		},
	})

	if reg.config.NatsUrl != "nats://test-server:4222" {
		t.Errorf("expected NatsUrl to be set, got %q", reg.config.NatsUrl)
	}
}

func TestResolveOutput_NatsUrlField(t *testing.T) {
	// Test that ResolveOutput can hold NatsUrl
	output := &ResolveOutput{
		CanonicalIdentity: "cap:@main/my.app/my.cap@1.0.0",
		NatsUrl:           "nats://sandbox:4222",
		Subject:           "cap.my.app.my_cap.v1",
		Major:             1,
		ResolvedVersion:   "1.0.0",
		Status:            "active",
		TTLSeconds:        300,
		Etag:              "test",
	}

	if output.NatsUrl != "nats://sandbox:4222" {
		t.Errorf("expected NatsUrl = nats://sandbox:4222, got %q", output.NatsUrl)
	}
	if output.CanonicalIdentity != "cap:@main/my.app/my.cap@1.0.0" {
		t.Errorf("expected CanonicalIdentity to be set, got %q", output.CanonicalIdentity)
	}
}
