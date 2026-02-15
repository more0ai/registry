package db

import (
	"context"
	"testing"
)

const poolTestPrefix = "db:pool_test"

func TestNewPool_InvalidURL(t *testing.T) {
	ctx := context.Background()
	pool, err := NewPool(ctx, "invalid://not-a-valid-database-url")
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatalf("%s - expected error for invalid URL", poolTestPrefix)
	}
	if pool != nil {
		t.Errorf("%s - expected nil pool on error", poolTestPrefix)
	}
}

func TestNewPool_EmptyURL(t *testing.T) {
	ctx := context.Background()
	pool, err := NewPool(ctx, "")
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatalf("%s - expected error for empty URL", poolTestPrefix)
	}
}
