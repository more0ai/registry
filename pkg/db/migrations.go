// Package db provides migration loading from directory.
package db

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

const migrationsLogPrefix = "db:migrations"

// LoadMigrationFiles reads all .sql files from dir, sorted by name, and returns their contents.
func LoadMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%s - failed to read migration dir %s: %w", migrationsLogPrefix, dir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var out []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s - failed to read %s: %w", migrationsLogPrefix, path, err)
		}
		out = append(out, string(data))
	}
	slog.Info(fmt.Sprintf("%s - Loaded %d migration files from %s", migrationsLogPrefix, len(out), dir))
	return out, nil
}
