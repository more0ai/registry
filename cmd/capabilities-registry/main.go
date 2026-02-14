// Package main is the entrypoint for the capabilities-registry.
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

const usage = `Usage: capabilities-registry [command]

Commands:
  (default)   Start the capabilities registry (NATS, HTTP, registry).
  migrate     Run database migrations only (does not start the server).
  clear       Truncate all registry tables; schema is preserved.
  seed [file] Load capabilities from bootstrap JSON. Optional file overrides REGISTRY_BOOTSTRAP_FILE.

Environment: DATABASE_URL, MIGRATION_PATH (migrate), REGISTRY_BOOTSTRAP_FILE (seed default). See README for full list.
`

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 && args[0] != "" {
		cmd = args[0]
	}

	switch cmd {
	case "migrate":
		if err := runMigrate(); err != nil {
			log.Fatalf("capabilities-registry migrate: %v", err)
		}
		return
	case "clear":
		if err := runClear(); err != nil {
			log.Fatalf("capabilities-registry clear: %v", err)
		}
		return
	case "seed":
		bootstrapFile := ""
		if len(args) > 1 {
			bootstrapFile = args[1]
		}
		if err := runSeed(bootstrapFile); err != nil {
			log.Fatalf("capabilities-registry seed: %v", err)
		}
		return
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	case "":
		// fall through to server
	default:
		// unknown subcommand
		fmt.Fprintf(os.Stderr, "Unknown command %q.\n%s", cmd, usage)
		os.Exit(1)
	}

	if err := server.Run(); err != nil {
		log.Fatalf("capabilities-registry: fatal error: %v", err)
	}
}

func runMigrate() error {
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
