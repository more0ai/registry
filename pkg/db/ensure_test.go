package db

import (
	"context"
	"net/url"
	"testing"
)

const ensureTestPrefix = "db:ensure_test"

func TestBuildPostgresURL(t *testing.T) {
	u, _ := url.Parse("postgres://user:pass@localhost:5432/mydb?sslmode=disable")
	got := buildPostgresURL(u)
	if got != "postgres://user:pass@localhost:5432/postgres?sslmode=disable" {
		t.Errorf("%s - buildPostgresURL = %q, want path /postgres", ensureTestPrefix, got)
	}
}

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"mydb", `"mydb"`},
		{"registry_test", `"registry_test"`},
		{`db"name`, `"db""name"`},
	}
	for _, tt := range tests {
		got := quoteIdent(tt.name)
		if got != tt.want {
			t.Errorf("%s - quoteIdent(%q) = %q, want %q", ensureTestPrefix, tt.name, got, tt.want)
		}
	}
}

func TestEnsureDatabase_InvalidURL(t *testing.T) {
	ctx := context.Background()
	err := EnsureDatabase(ctx, "://invalid")
	if err == nil {
		t.Fatalf("%s - expected error for invalid URL", ensureTestPrefix)
	}
}

func TestEnsureDatabase_EmptyDbname(t *testing.T) {
	ctx := context.Background()
	// URL with empty path (e.g. postgres://host/)
	err := EnsureDatabase(ctx, "postgres://localhost:5432/?sslmode=disable")
	if err == nil {
		t.Fatalf("%s - expected error for empty dbname", ensureTestPrefix)
	}
}

func TestEnsureDatabase_InvalidDbnameChars(t *testing.T) {
	ctx := context.Background()
	err := EnsureDatabase(ctx, "postgres://localhost:5432/my-db?sslmode=disable")
	if err == nil {
		t.Fatalf("%s - expected error for invalid dbname (hyphen)", ensureTestPrefix)
	}
}
