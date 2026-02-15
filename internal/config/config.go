// Package config provides server configuration loaded from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

const logPrefix = "config:LoadConfig"

// Config holds capabilities-registry configuration.
type Config struct {
	// COMMS: connect to standalone NATS at COMMSURL.
	COMMSURL  string `envconfig:"COMMS_URL" default:"nats://127.0.0.1:4222"`
	COMMSName string `envconfig:"SERVICE_NAME" default:"capabilities-registry"`
	// NATSClientURL is the NATS URL returned to clients via GET /connection (e.g. from host: nats://127.0.0.1:4222).
	NATSClientURL string `envconfig:"NATS_CLIENT_URL"`

	// Registry subject overrides (empty = derive from bootstrap)
	RegistrySubject    string `envconfig:"REGISTRY_SUBJECT"`
	ChangeEventSubject string `envconfig:"REGISTRY_CHANGE_EVENT_SUBJECT"`

	// Timeouts
	RequestTimeout time.Duration `envconfig:"REGISTRY_REQUEST_TIMEOUT" default:"25s"`

	// Bootstrap
	BootstrapFile string `envconfig:"REGISTRY_BOOTSTRAP_FILE"`

	// Database
	DatabaseURL   string `envconfig:"DATABASE_URL" default:"postgres://morezero:morezero_secret@localhost:5432/morezero?sslmode=disable"`
	RunMigrations  bool   `envconfig:"RUN_MIGRATIONS" default:"false"`
	MigrationPath string `envconfig:"MIGRATION_PATH" default:"migrations"`

	// HTTP health endpoint (REGISTRY_HTTP_ADDR preferred, e.g. "0.0.0.0:8080")
	HTTPAddr          string        `envconfig:"REGISTRY_HTTP_ADDR"`
	HTTPPort          int           `envconfig:"HTTP_PORT" default:"8080"`
	HealthCheckTimeout time.Duration `envconfig:"HEALTH_CHECK_TIMEOUT" default:"5s"`

	// Logging
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ValidateForServe checks required config when running the registry server.
func (c *Config) ValidateForServe() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("%s - DATABASE_URL is required for serve", logPrefix)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("%s - REGISTRY_REQUEST_TIMEOUT must be positive", logPrefix)
	}
	if c.HealthCheckTimeout <= 0 {
		return fmt.Errorf("%s - HEALTH_CHECK_TIMEOUT must be positive", logPrefix)
	}
	return nil
}

// ValidateForDB checks required config when running DB-dependent commands (migrate, clear, seed).
func (c *Config) ValidateForDB() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("%s - DATABASE_URL is required", logPrefix)
	}
	return nil
}
