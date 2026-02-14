package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all environment variables that might interfere
	envVars := []string{
		"COMMS_URL", "SERVICE_NAME",
		"REGISTRY_SUBJECT", "REGISTRY_CHANGE_EVENT_SUBJECT",
		"REGISTRY_REQUEST_TIMEOUT", "REGISTRY_BOOTSTRAP_FILE",
		"DATABASE_URL", "RUN_MIGRATIONS", "MIGRATION_PATH",
		"HTTP_PORT", "HEALTH_CHECK_TIMEOUT", "LOG_LEVEL",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("config:config_test - unexpected error: %v", err)
	}

	// Verify defaults
	if cfg.COMMSURL != "nats://127.0.0.1:4222" {
		t.Errorf("config:config_test - COMMSURL = %q, want %q", cfg.COMMSURL, "nats://127.0.0.1:4222")
	}
	if cfg.COMMSName != "capabilities-registry" {
		t.Errorf("config:config_test - COMMSName = %q, want %q", cfg.COMMSName, "capabilities-registry")
	}
	if cfg.RegistrySubject != "" {
		t.Errorf("config:config_test - RegistrySubject = %q, want empty", cfg.RegistrySubject)
	}
	if cfg.ChangeEventSubject != "" {
		t.Errorf("config:config_test - ChangeEventSubject = %q, want empty", cfg.ChangeEventSubject)
	}
	if cfg.RequestTimeout != 25*time.Second {
		t.Errorf("config:config_test - RequestTimeout = %v, want 25s", cfg.RequestTimeout)
	}
	if cfg.BootstrapFile != "" {
		t.Errorf("config:config_test - BootstrapFile = %q, want empty", cfg.BootstrapFile)
	}
	if cfg.DatabaseURL != "postgres://morezero:morezero_secret@localhost:5432/morezero?sslmode=disable" {
		t.Errorf("config:config_test - DatabaseURL = %q, unexpected default", cfg.DatabaseURL)
	}
	if cfg.RunMigrations {
		t.Error("config:config_test - expected RunMigrations=false by default")
	}
	if cfg.MigrationPath != "migrations" {
		t.Errorf("config:config_test - MigrationPath = %q, want %q", cfg.MigrationPath, "migrations")
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("config:config_test - HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.HealthCheckTimeout != 5*time.Second {
		t.Errorf("config:config_test - HealthCheckTimeout = %v, want 5s", cfg.HealthCheckTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("config:config_test - LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoadConfig_EnvironmentOverrides(t *testing.T) {
	// Set environment variables
	overrides := map[string]string{
		"COMMS_URL":                       "nats://custom:4222",
		"SERVICE_NAME":                    "test-server",
		"REGISTRY_SUBJECT":               "custom.registry",
		"REGISTRY_CHANGE_EVENT_SUBJECT":   "custom.changed",
		"REGISTRY_REQUEST_TIMEOUT":        "10s",
		"REGISTRY_BOOTSTRAP_FILE":         "/tmp/bootstrap.json",
		"DATABASE_URL":                    "postgres://test@localhost/test",
		"RUN_MIGRATIONS":                  "true",
		"MIGRATION_PATH":                  "/tmp/migrations",
		"HTTP_PORT":                       "9090",
		"HEALTH_CHECK_TIMEOUT":            "10s",
		"LOG_LEVEL":                       "debug",
	}

	for key, val := range overrides {
		os.Setenv(key, val)
	}
	defer func() {
		for key := range overrides {
			os.Unsetenv(key)
		}
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("config:config_test - unexpected error: %v", err)
	}

	if cfg.COMMSURL != "nats://custom:4222" {
		t.Errorf("config:config_test - COMMSURL = %q, want %q", cfg.COMMSURL, "nats://custom:4222")
	}
	if cfg.COMMSName != "test-server" {
		t.Errorf("config:config_test - COMMSName = %q, want %q", cfg.COMMSName, "test-server")
	}
	if cfg.RegistrySubject != "custom.registry" {
		t.Errorf("config:config_test - RegistrySubject = %q, want %q", cfg.RegistrySubject, "custom.registry")
	}
	if cfg.ChangeEventSubject != "custom.changed" {
		t.Errorf("config:config_test - ChangeEventSubject = %q, want %q", cfg.ChangeEventSubject, "custom.changed")
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("config:config_test - RequestTimeout = %v, want 10s", cfg.RequestTimeout)
	}
	if cfg.BootstrapFile != "/tmp/bootstrap.json" {
		t.Errorf("config:config_test - BootstrapFile = %q, want %q", cfg.BootstrapFile, "/tmp/bootstrap.json")
	}
	if cfg.DatabaseURL != "postgres://test@localhost/test" {
		t.Errorf("config:config_test - DatabaseURL = %q, unexpected", cfg.DatabaseURL)
	}
	if !cfg.RunMigrations {
		t.Error("config:config_test - expected RunMigrations=true")
	}
	if cfg.MigrationPath != "/tmp/migrations" {
		t.Errorf("config:config_test - MigrationPath = %q, want %q", cfg.MigrationPath, "/tmp/migrations")
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("config:config_test - HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
	if cfg.HealthCheckTimeout != 10*time.Second {
		t.Errorf("config:config_test - HealthCheckTimeout = %v, want 10s", cfg.HealthCheckTimeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("config:config_test - LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadConfig_LogLevels(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, level := range validLevels {
		os.Setenv("LOG_LEVEL", level)
		cfg, err := LoadConfig()
		os.Unsetenv("LOG_LEVEL")

		if err != nil {
			t.Fatalf("config:config_test - unexpected error for level %q: %v", level, err)
		}
		if cfg.LogLevel != level {
			t.Errorf("config:config_test - LogLevel = %q, want %q", cfg.LogLevel, level)
		}
	}
}
