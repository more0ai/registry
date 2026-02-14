package bootstrap

import (
	"testing"
)

func TestGetDefaultBootstrapConfig(t *testing.T) {
	cfg := GetDefaultBootstrapConfig()

	if cfg.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", cfg.Version)
	}

	if len(cfg.Capabilities) == 0 {
		t.Fatal("expected capabilities, got none")
	}

	registry, ok := cfg.Capabilities["system.registry"]
	if !ok {
		t.Fatal("expected system.registry capability")
	}

	if registry.Subject != "cap.system.registry.v1" {
		t.Errorf("expected subject cap.system.registry.v1, got %s", registry.Subject)
	}

	if !registry.IsSystem {
		t.Error("expected system.registry to be a system capability")
	}

	if len(registry.Methods) == 0 {
		t.Error("expected methods on system.registry")
	}
}

func TestCreateResolvedBootstrap(t *testing.T) {
	cfg := GetDefaultBootstrapConfig()
	resolved := CreateResolvedBootstrap(cfg)

	// Direct lookup
	cap := resolved.Get("system.registry")
	if cap == nil {
		t.Fatal("expected system.registry, got nil")
	}
	if cap.Subject != "cap.system.registry.v1" {
		t.Errorf("expected cap.system.registry.v1, got %s", cap.Subject)
	}

	// Alias lookup
	cap = resolved.Get("registry")
	if cap == nil {
		t.Fatal("expected alias 'registry' to resolve, got nil")
	}
	if cap.Subject != "cap.system.registry.v1" {
		t.Errorf("expected cap.system.registry.v1 via alias, got %s", cap.Subject)
	}

	// Non-existent
	cap = resolved.Get("nonexistent")
	if cap != nil {
		t.Errorf("expected nil for non-existent capability, got %v", cap)
	}

	// IsSystem
	if !resolved.IsSystem("system.registry") {
		t.Error("expected system.registry to be system")
	}
	if resolved.IsSystem("nonexistent") {
		t.Error("expected nonexistent to not be system")
	}

	// GetSubject
	subject := resolved.GetSubject("system.registry")
	if subject != "cap.system.registry.v1" {
		t.Errorf("expected cap.system.registry.v1, got %s", subject)
	}
	subject = resolved.GetSubject("auth")
	if subject != "cap.system.auth.v1" {
		t.Errorf("expected cap.system.auth.v1 via alias, got %s", subject)
	}
	subject = resolved.GetSubject("nonexistent")
	if subject != "" {
		t.Errorf("expected empty string for non-existent, got %s", subject)
	}
}

func TestResolveAlias(t *testing.T) {
	cfg := GetDefaultBootstrapConfig()
	resolved := CreateResolvedBootstrap(cfg)

	got := resolved.ResolveAlias("registry")
	if got != "system.registry" {
		t.Errorf("expected system.registry, got %s", got)
	}

	got = resolved.ResolveAlias("nonexistent")
	if got != "nonexistent" {
		t.Errorf("expected passthrough for unknown alias, got %s", got)
	}
}

func TestMergeBootstrapConfigs(t *testing.T) {
	base := GetDefaultBootstrapConfig()
	override := &BootstrapConfig{
		Capabilities: map[string]BootstrapCapability{
			"system.custom": {
				Subject:  "cap.system.custom.v1",
				Major:    1,
				Version:  "1.0.0",
				Status:   "active",
				Methods:  []string{"doStuff"},
				IsSystem: true,
			},
		},
		Aliases: map[string]string{
			"custom": "system.custom",
		},
	}

	merged := MergeBootstrapConfigs(base, override)

	// Should have all base capabilities plus the override
	if _, ok := merged.Capabilities["system.registry"]; !ok {
		t.Error("expected system.registry from base to remain")
	}
	if _, ok := merged.Capabilities["system.custom"]; !ok {
		t.Error("expected system.custom from override to be added")
	}

	// Should have all aliases
	if merged.Aliases["registry"] != "system.registry" {
		t.Error("expected base alias to remain")
	}
	if merged.Aliases["custom"] != "system.custom" {
		t.Error("expected override alias to be added")
	}
}

func TestList(t *testing.T) {
	cfg := GetDefaultBootstrapConfig()
	resolved := CreateResolvedBootstrap(cfg)

	caps := resolved.List()
	if len(caps) != len(cfg.Capabilities) {
		t.Errorf("expected %d capabilities, got %d", len(cfg.Capabilities), len(caps))
	}
}
