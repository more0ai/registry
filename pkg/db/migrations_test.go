package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigrationFiles_ValidDir(t *testing.T) {
	// Create a temp directory with SQL files
	dir := t.TempDir()

	files := map[string]string{
		"0001_create_table.sql": "CREATE TABLE test (id SERIAL PRIMARY KEY);",
		"0002_add_column.sql":  "ALTER TABLE test ADD COLUMN name TEXT;",
		"0003_add_index.sql":   "CREATE INDEX idx_name ON test(name);",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("db:migrations_test - failed to write test file %s: %v", name, err)
		}
	}

	result, err := LoadMigrationFiles(dir)
	if err != nil {
		t.Fatalf("db:migrations_test - unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("db:migrations_test - expected 3 migrations, got %d", len(result))
	}

	// Verify order (should be sorted by filename)
	if result[0] != "CREATE TABLE test (id SERIAL PRIMARY KEY);" {
		t.Errorf("db:migrations_test - first migration content mismatch")
	}
	if result[1] != "ALTER TABLE test ADD COLUMN name TEXT;" {
		t.Errorf("db:migrations_test - second migration content mismatch")
	}
	if result[2] != "CREATE INDEX idx_name ON test(name);" {
		t.Errorf("db:migrations_test - third migration content mismatch")
	}
}

func TestLoadMigrationFiles_SkipsNonSQLFiles(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"0001_create.sql":   "CREATE TABLE t1;",
		"README.md":         "# Migrations",
		"notes.txt":         "some notes",
		"0002_alter.sql":    "ALTER TABLE t1;",
		"config.json":       "{}",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("db:migrations_test - failed to write test file: %v", err)
		}
	}

	result, err := LoadMigrationFiles(dir)
	if err != nil {
		t.Fatalf("db:migrations_test - unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("db:migrations_test - expected 2 SQL files, got %d", len(result))
	}
}

func TestLoadMigrationFiles_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(dir, "subdir.sql") // tricky name ending with .sql
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("db:migrations_test - failed to create subdir: %v", err)
	}

	// Create actual SQL file
	sqlFile := filepath.Join(dir, "0001_create.sql")
	if err := os.WriteFile(sqlFile, []byte("CREATE TABLE x;"), 0644); err != nil {
		t.Fatalf("db:migrations_test - failed to write file: %v", err)
	}

	result, err := LoadMigrationFiles(dir)
	if err != nil {
		t.Fatalf("db:migrations_test - unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("db:migrations_test - expected 1 migration (skipping dir), got %d", len(result))
	}
}

func TestLoadMigrationFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := LoadMigrationFiles(dir)
	if err != nil {
		t.Fatalf("db:migrations_test - unexpected error: %v", err)
	}

	if result != nil && len(result) != 0 {
		t.Errorf("db:migrations_test - expected empty result, got %d items", len(result))
	}
}

func TestLoadMigrationFiles_NonExistentDir(t *testing.T) {
	_, err := LoadMigrationFiles(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Error("db:migrations_test - expected error for non-existent directory")
	}
}

func TestLoadMigrationFiles_SortOrder(t *testing.T) {
	dir := t.TempDir()

	// Write files in reverse order to ensure sorting works
	files := []struct {
		name    string
		content string
	}{
		{"0003_third.sql", "THIRD"},
		{"0001_first.sql", "FIRST"},
		{"0002_second.sql", "SECOND"},
	}

	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("db:migrations_test - failed to write file: %v", err)
		}
	}

	result, err := LoadMigrationFiles(dir)
	if err != nil {
		t.Fatalf("db:migrations_test - unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("db:migrations_test - expected 3, got %d", len(result))
	}
	if result[0] != "FIRST" {
		t.Errorf("db:migrations_test - expected FIRST at index 0, got %s", result[0])
	}
	if result[1] != "SECOND" {
		t.Errorf("db:migrations_test - expected SECOND at index 1, got %s", result[1])
	}
	if result[2] != "THIRD" {
		t.Errorf("db:migrations_test - expected THIRD at index 2, got %s", result[2])
	}
}

func TestContainsInt(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		val   int
		want  bool
	}{
		{"found at start", []int{1, 2, 3}, 1, true},
		{"found at end", []int{1, 2, 3}, 3, true},
		{"found in middle", []int{1, 2, 3}, 2, true},
		{"not found", []int{1, 2, 3}, 4, false},
		{"empty slice", []int{}, 1, false},
		{"nil slice", nil, 1, false},
		{"single element found", []int{5}, 5, true},
		{"single element not found", []int{5}, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsInt(tt.slice, tt.val)
			if got != tt.want {
				t.Errorf("db:migrations_test - containsInt(%v, %d) = %v, want %v",
					tt.slice, tt.val, got, tt.want)
			}
		})
	}
}
