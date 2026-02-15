package registry

import (
	"context"
	"testing"

	"github.com/morezero/capabilities-registry/pkg/events"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTTLSeconds != 300 {
		t.Errorf("registry:registry_test - DefaultTTLSeconds = %d, want 300", cfg.DefaultTTLSeconds)
	}
	if cfg.DefaultEnv != "production" {
		t.Errorf("registry:registry_test - DefaultEnv = %q, want %q", cfg.DefaultEnv, "production")
	}
	if cfg.SubjectPrefix != "cap" {
		t.Errorf("registry:registry_test - SubjectPrefix = %q, want %q", cfg.SubjectPrefix, "cap")
	}
}

func TestNewRegistry_DefaultConfig(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil, // not testing DB operations
		Publisher: nil,
		Config:    Config{},
	})

	if reg.config.DefaultTTLSeconds != defaultTTLSeconds {
		t.Errorf("registry:registry_test - DefaultTTLSeconds = %d, want %d", reg.config.DefaultTTLSeconds, defaultTTLSeconds)
	}
	if reg.config.DefaultEnv != defaultEnv {
		t.Errorf("registry:registry_test - DefaultEnv = %q, want %q", reg.config.DefaultEnv, defaultEnv)
	}
	if reg.config.SubjectPrefix != defaultSubjectPrefix {
		t.Errorf("registry:registry_test - SubjectPrefix = %q, want %q", reg.config.SubjectPrefix, defaultSubjectPrefix)
	}
}

func TestNewRegistry_CustomConfig(t *testing.T) {
	customCfg := Config{
		DefaultTTLSeconds: 600,
		DefaultEnv:        "staging",
		SubjectPrefix:     "custom",
	}

	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    customCfg,
	})

	if reg.config.DefaultTTLSeconds != 600 {
		t.Errorf("registry:registry_test - DefaultTTLSeconds = %d, want 600", reg.config.DefaultTTLSeconds)
	}
	if reg.config.DefaultEnv != "staging" {
		t.Errorf("registry:registry_test - DefaultEnv = %q, want %q", reg.config.DefaultEnv, "staging")
	}
	if reg.config.SubjectPrefix != "custom" {
		t.Errorf("registry:registry_test - SubjectPrefix = %q, want %q", reg.config.SubjectPrefix, "custom")
	}
}

func TestNewRegistry_NilPublisherDefaultsToNoOp(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})

	_, isNoOp := reg.publisher.(*events.NoOpPublisher)
	if !isNoOp {
		t.Errorf("registry:registry_test - expected NoOpPublisher when Publisher is nil, got %T", reg.publisher)
	}
}

func TestNewRegistry_CustomPublisher(t *testing.T) {
	pub := &events.NoOpPublisher{}
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: pub,
		Config:    DefaultConfig(),
	})

	if reg.publisher != pub {
		t.Errorf("registry:registry_test - expected provided publisher to be used")
	}
}

func TestBuildSubject(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})

	tests := []struct {
		name  string
		app   string
		capN  string
		major int
		want  string
	}{
		{
			name:  "simple name",
			app:   "more0",
			capN:  "registry",
			major: 1,
			want:  "cap.more0.registry.v1",
		},
		{
			name:  "dotted name is replaced",
			app:   "more0",
			capN:  "doc.ingest",
			major: 3,
			want:  "cap.more0.doc_ingest.v3",
		},
		{
			name:  "system capability",
			app:   "system",
			capN:  "auth",
			major: 2,
			want:  "cap.system.auth.v2",
		},
		{
			name:  "multiple dots replaced",
			app:   "more0",
			capN:  "a.b.c",
			major: 1,
			want:  "cap.more0.a_b_c.v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reg.buildSubject(tt.app, tt.capN, tt.major)
			if got != tt.want {
				t.Errorf("registry:registry_test - buildSubject(%q, %q, %d) = %q, want %q",
					tt.app, tt.capN, tt.major, got, tt.want)
			}
		})
	}
}

func TestBuildSubject_CustomPrefix(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config: Config{
			DefaultTTLSeconds: 300,
			DefaultEnv:        "production",
			SubjectPrefix:     "svc",
		},
	})

	got := reg.buildSubject("more0", "registry", 1)
	want := "svc.more0.registry.v1"
	if got != want {
		t.Errorf("registry:registry_test - buildSubject with custom prefix = %q, want %q", got, want)
	}
}

func TestGetEnv(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})

	tests := []struct {
		name string
		ctx  *ResolutionContext
		want string
	}{
		{
			name: "nil context returns default",
			ctx:  nil,
			want: "production",
		},
		{
			name: "empty env returns default",
			ctx:  &ResolutionContext{Env: ""},
			want: "production",
		},
		{
			name: "custom env returned",
			ctx:  &ResolutionContext{Env: "staging"},
			want: "staging",
		},
		{
			name: "context with other fields but no env",
			ctx:  &ResolutionContext{TenantID: "tenant-1"},
			want: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reg.getEnv(tt.ctx)
			if got != tt.want {
				t.Errorf("registry:registry_test - getEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetEnv_CustomDefaultEnv(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config: Config{
			DefaultTTLSeconds: 300,
			DefaultEnv:        "development",
			SubjectPrefix:     "cap",
		},
	})

	got := reg.getEnv(nil)
	if got != "development" {
		t.Errorf("registry:registry_test - getEnv(nil) with custom default = %q, want %q", got, "development")
	}

	got = reg.getEnv(&ResolutionContext{Env: "staging"})
	if got != "staging" {
		t.Errorf("registry:registry_test - getEnv(staging) = %q, want %q", got, "staging")
	}
}

func TestClose_NoPanic(t *testing.T) {
	// Registry with nil repo has nil federationPool
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})
	reg.Close()
	// No panic
}

func TestRequireRepo_ReturnsErrorWhenNil(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})
	err := reg.requireRepo()
	if err == nil {
		t.Fatal("registry:registry_test - expected error when repo is nil")
	}
	if err.Code != "INTERNAL_ERROR" {
		t.Errorf("registry:registry_test - Code = %q, want INTERNAL_ERROR", err.Code)
	}
}

func TestGetBootstrapCapabilities_NilRepo_ReturnsEmpty(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	out, err := reg.GetBootstrapCapabilities(ctx, "production", false, false)
	if err != nil {
		t.Fatalf("registry:registry_test - unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("registry:registry_test - expected non-nil map")
	}
	if len(out) != 0 {
		t.Errorf("registry:registry_test - expected empty map, got %d entries", len(out))
	}
}

func TestLoadRegistryAliases_NilFederationPool_ReturnsDefaultAlias(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	aliases, defaultAlias, err := reg.LoadRegistryAliases(ctx)
	if err != nil {
		t.Fatalf("registry:registry_test - unexpected error: %v", err)
	}
	if aliases != nil {
		t.Errorf("registry:registry_test - expected nil aliases, got %v", aliases)
	}
	if defaultAlias != "main" {
		t.Errorf("registry:registry_test - defaultAlias = %q, want main", defaultAlias)
	}
}
