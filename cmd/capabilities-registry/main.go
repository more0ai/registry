// Package main is the entrypoint for the capabilities-registry (binary name "registry" in Docker).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/morezero/capabilities-registry/internal/config"
	"github.com/morezero/capabilities-registry/internal/server"
	"github.com/morezero/capabilities-registry/pkg/db"
)

const usage = `Usage: registry [command]
       registry serve              Start the registry (NATS, HTTP, registry API).
       registry migrate up          Run database migrations.
       registry migrate down        Roll back one migration (optional; not all migrations support down).
       registry migrate status      Show migration status.
       registry clear               Truncate all registry tables; schema is preserved.
       registry seed [file]         Load capabilities from bootstrap JSON.

Commands:
  serve           (default) Start the capabilities registry.
  migrate up      Run database migrations only.
  migrate down    Roll back last migration (optional).
  migrate status  Show current migration status.
  clear           Truncate registry data; schema preserved.
  seed [file]     Seed from REGISTRY_BOOTSTRAP_FILE or optional file.

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

func runSeed(bootstrapFileOverride string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	if err := db.SeedBootstrap(ctx, pool, bootstrapPath); err != nil {
		return fmt.Errorf("seed bootstrap: %w", err)
	}
	return nil
}
