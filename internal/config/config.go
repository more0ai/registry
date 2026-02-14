// Package config provides server configuration loaded from environment variables.
package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

const logPrefix = "config:LoadConfig"

// Config holds capabilities-registry configuration.
type Config struct {
	// COMMS: connect to standalone NATS at COMMSURL.
	COMMSURL  string `envconfig:"COMMS_URL" default:"nats://127.0.0.1:4222"`
	COMMSName string `envconfig:"SERVICE_NAME" default:"capabilities-registry"`

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

	// HTTP health endpoint
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
