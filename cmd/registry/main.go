// Package main is the entrypoint for the capabilities-registry (binary name "registry" in Docker).
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/morezero/capabilities-registry/internal/config"
	"github.com/morezero/capabilities-registry/internal/server"
	"github.com/morezero/capabilities-registry/pkg/db"
)

const usage = `Usage: registry [command]
       registry serve              Start the registry (NATS, HTTP, registry API).
       registry migrate up          Run database migrations.
       registry migrate down        Roll back one migration (optional; not all migrations support down).
       registry migrate status      Show migration status.
       registry ensure-db [name]    Create database if missing (default name: registry_test). Uses DATABASE_URL host/user.
       registry clear               Truncate all registry tables; schema is preserved.
       registry seed [file]         Seed from capabilities metadata (e.g. registry/capabilities/metadata.json).

Commands:
  serve           (default) Start the capabilities registry.
  migrate up      Run database migrations only.
  migrate down    Roll back last migration (optional).
  migrate status  Show current migration status.
  ensure-db [name] Create database (e.g. registry_test) on same host as DATABASE_URL; then run tests with that URL.
  clear           Truncate registry data; schema preserved.
  seed [file]     Seed from capabilities metadata (path derived from bootstrap file or REGISTRY_BOOTSTRAP_FILE).

Environment: DATABASE_URL (required), MIGRATION_PATH, REGISTRY_HTTP_ADDR (default 0.0.0.0:8080), REGISTRY_BOOTSTRAP_FILE. See README.
`

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 && args[0] != "" {
		cmd = args[0]
	}

	switch cmd {
	case "migrate":
		if len(args) < 2 {
			log.Fatalf("registry migrate: require subcommand (up, down, status)")
		}
		sub := args[1]
		switch sub {
		case "up":
			if err := runMigrateUp(); err != nil {
				log.Fatalf("registry migrate up: %v", err)
			}
		case "status":
			if err := runMigrateStatus(); err != nil {
				log.Fatalf("registry migrate status: %v", err)
			}
		case "down":
			if err := runMigrateDown(); err != nil {
				log.Fatalf("registry migrate down: %v", err)
			}
		default:
			log.Fatalf("registry migrate: unknown subcommand %q (use up, down, status)", sub)
		}
		return
	case "clear":
		if err := runClear(); err != nil {
			log.Fatalf("registry clear: %v", err)
		}
		return
	case "seed":
		bootstrapFile := ""
		if len(args) > 1 {
			bootstrapFile = args[1]
		}
		if err := runSeed(bootstrapFile); err != nil {
			log.Fatalf("registry seed: %v", err)
		}
		return
	case "ensure-db":
		dbName := "registry_test"
		if len(args) > 1 && args[1] != "" {
			dbName = args[1]
		}
		if err := runEnsureDB(dbName); err != nil {
			log.Fatalf("registry ensure-db: %v", err)
		}
		return
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	case "serve", "":
		// serve (explicit or default)
		break
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q.\n%s", cmd, usage)
		os.Exit(1)
	}

	if err := server.Run(); err != nil {
		log.Fatalf("registry: %v", err)
	}
}

func runMigrateUp() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.ValidateForDB(); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	migrationSQL, err := db.LoadMigrationFiles(cfg.MigrationPath)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if err := db.RunMigrations(ctx, pool, migrationSQL); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func runMigrateStatus() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.ValidateForDB(); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	return db.MigrationStatus(ctx, pool, cfg.MigrationPath)
}

func runMigrateDown() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.ValidateForDB(); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	return db.MigrationDown(ctx, pool, cfg.MigrationPath)
}

func runClear() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.ValidateForDB(); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if err := db.ClearRegistry(ctx, pool); err != nil {
		return fmt.Errorf("clear registry: %w", err)
	}
	return nil
}

func runEnsureDB(dbName string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	// Replace path with target database name; query (e.g. sslmode) is kept on u.RawQuery.
	u.Path = "/" + dbName
	targetURL := u.String()
	ctx := context.Background()
	if err := db.EnsureDatabase(ctx, targetURL); err != nil {
		return err
	}
	fmt.Printf("Database %q is ready.\n", dbName)
	return nil
}

func runSeed(bootstrapFileOverride string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.ValidateForDB(); err != nil {
		return err
	}
	bootstrapPath := bootstrapFileOverride
	if bootstrapPath == "" {
		bootstrapPath = cfg.BootstrapFile
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	// Seed capability metadata from registry/capabilities/metadata.json (e.g. system.registry).
	// Bootstrap file is not used for seeding; bootstrap response is built from DB.
	metadataPath := filepath.Clean(filepath.Join(filepath.Dir(bootstrapPath), "..", "capabilities", "metadata.json"))
	baseDir := ""
	if bootstrapFileOverride != "" {
		baseDir, _ = os.Getwd()
	}
	if err := db.SeedFromCapabilityMetadataFile(ctx, pool, metadataPath, baseDir); err != nil {
		return fmt.Errorf("seed capability metadata: %w", err)
	}
	return nil
}
