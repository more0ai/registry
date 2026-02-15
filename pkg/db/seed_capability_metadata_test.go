package db

import (
	"context"
	"path/filepath"
	"testing"
)

const seedCapTestPrefix = "db:seed_capability_metadata_test"

func TestSeedFromCapabilityMetadataFile_EmptyPath(t *testing.T) {
	ctx := context.Background()
	// No pool needed - function returns nil for empty path
	err := SeedFromCapabilityMetadataFile(ctx, nil, "", "")
	if err != nil {
		t.Errorf("%s - expected nil for empty path, got %v", seedCapTestPrefix, err)
	}
}

func TestSeedFromCapabilityMetadataFile_PathTraversal_Rejected(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	// Path outside baseDir (e.g. ../etc/passwd style) should be rejected
	pathOutside := filepath.Clean(filepath.Join(baseDir, "..", "outside"))
	err := SeedFromCapabilityMetadataFile(ctx, nil, pathOutside, baseDir)
	if err == nil {
		t.Fatalf("%s - expected error for path outside baseDir", seedCapTestPrefix)
	}
}
