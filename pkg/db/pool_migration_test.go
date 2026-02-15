package db

import (
	"context"
	"testing"
)

const poolMigrationTestPrefix = "db:pool_migration_test"

// TestMigrationDown_ReturnsNil verifies MigrationDown is a no-op and returns nil.
// The function does not use the pool; it only prints a message.
func TestMigrationDown_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	err := MigrationDown(ctx, nil, "")
	if err != nil {
		t.Errorf("%s - MigrationDown returned %v, want nil", poolMigrationTestPrefix, err)
	}
}
