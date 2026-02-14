package bootstrap

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

const logPrefix = "bootstrap:loader"

// LoadBootstrapConfig loads bootstrap config from file paths or environment.
// It tries paths in order: first any paths passed in, then REGISTRY_BOOTSTRAP_FILE env, then defaults.
// So an explicit path (e.g. from "seed my.json") is tried before the env var.
func LoadBootstrapConfig(paths ...string) (*BootstrapConfig, error) {
	// Build path list: passed paths first, then env, then defaults
	all := make([]string, 0, len(paths)+4)
	for _, p := range paths {
		if p != "" {
			all = append(all, p)
		}
	}
	if envPath := os.Getenv("REGISTRY_BOOTSTRAP_FILE"); envPath != "" {
		all = append(all, envPath)
	}
	all = append(all, "config/bootstrap.json", "bootstrap.json")
	paths = all

	for _, p := range paths {
		if p == "" {
			continue
		}

		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var cfg BootstrapConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			slog.Warn(fmt.Sprintf("%s - Failed to parse bootstrap file %s: %v", logPrefix, p, err))
			continue
		}

		slog.Info(fmt.Sprintf("%s - Loaded bootstrap config from %s", logPrefix, p))
		return &cfg, nil
	}

	slog.Info(fmt.Sprintf("%s - Using default bootstrap config", logPrefix))
	return GetDefaultBootstrapConfig(), nil
}

// GetDefaultBootstrapConfig returns the embedded fallback bootstrap configuration.
func GetDefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		Name:                 "morezero-bootstrap",
		Version:              "1.0.0",
		Description:          "Default system capability bootstrap configuration",
		MinimumCapabilities:  []string{"system.registry", "system.auth", "system.config", "system.health", "system.events"},
		Capabilities: map[string]BootstrapCapability{
			"system.registry": {
				Subject:     "cap.system.registry.v1",
				Major:       1,
				Version:     "1.0.0",
				Status:      "active",
				Description: "Core registry service for capability resolution",
				Methods:     []string{"resolve", "discover", "describe", "upsert", "deprecate", "disable", "setDefaultMajor", "listMajors", "health"},
				IsSystem:    true,
				TTLSeconds:  0,
			},
			"system.auth": {
				Subject:     "cap.system.auth.v1",
				Major:       1,
				Version:     "1.0.0",
				Status:      "active",
				Description: "Authentication and authorization service",
				Methods:     []string{"authenticate", "authorize", "validate", "refresh"},
				IsSystem:    true,
				TTLSeconds:  0,
			},
			"system.config": {
				Subject:     "cap.system.config.v1",
				Major:       1,
				Version:     "1.0.0",
				Status:      "active",
				Description: "Configuration service",
				Methods:     []string{"get", "set", "list", "watch"},
				IsSystem:    true,
				TTLSeconds:  0,
			},
			"system.health": {
				Subject:     "cap.system.health.v1",
				Major:       1,
				Version:     "1.0.0",
				Status:      "active",
				Description: "System health monitoring",
				Methods:     []string{"check", "status", "metrics"},
				IsSystem:    true,
				TTLSeconds:  0,
			},
			"system.events": {
				Subject:     "cap.system.events.v1",
				Major:       1,
				Version:     "1.0.0",
				Status:      "active",
				Description: "Event bus service",
				Methods:     []string{"publish", "subscribe", "unsubscribe"},
				IsSystem:    true,
				TTLSeconds:  0,
			},
		},
		Aliases: map[string]string{
			"registry": "system.registry",
			"auth":     "system.auth",
			"config":   "system.config",
		},
		ChangeEvents: ChangeEventSubjects{
			Global:  "registry.changed",
			Pattern: "registry.changed.{app}.{capability}",
		},
	}
}

// CreateResolvedBootstrap builds a ResolvedBootstrap for fast lookups.
func CreateResolvedBootstrap(cfg *BootstrapConfig) *ResolvedBootstrap {
	caps := make(map[string]*BootstrapCapability, len(cfg.Capabilities))
	for ref, cap := range cfg.Capabilities {
		c := cap // copy to avoid pointer aliasing
		caps[ref] = &c
	}

	aliases := make(map[string]string, len(cfg.Aliases))
	for alias, target := range cfg.Aliases {
		aliases[alias] = target
	}

	minCaps := make([]string, len(cfg.MinimumCapabilities))
	copy(minCaps, cfg.MinimumCapabilities)

	return &ResolvedBootstrap{
		name:         cfg.Name,
		version:      cfg.Version,
		minCaps:      minCaps,
		capabilities: caps,
		aliases:      aliases,
		changeEvents: cfg.ChangeEvents,
	}
}

// MergeBootstrapConfigs merges an override config into a base config.
func MergeBootstrapConfigs(base, override *BootstrapConfig) *BootstrapConfig {
	merged := *base

	// Merge capabilities
	if merged.Capabilities == nil {
		merged.Capabilities = make(map[string]BootstrapCapability)
	}
	for ref, cap := range override.Capabilities {
		merged.Capabilities[ref] = cap
	}

	// Merge aliases
	if merged.Aliases == nil {
		merged.Aliases = make(map[string]string)
	}
	for alias, target := range override.Aliases {
		merged.Aliases[alias] = target
	}

	// Override change events if set
	if override.ChangeEvents.Global != "" {
		merged.ChangeEvents.Global = override.ChangeEvents.Global
	}
	if override.ChangeEvents.Pattern != "" {
		merged.ChangeEvents.Pattern = override.ChangeEvents.Pattern
	}

	return &merged
}
