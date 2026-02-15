package registry

import (
	"context"
	"testing"
)

const describeTestPrefix = "registry:describe_test"

func TestDescribe_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})
	ctx := context.Background()

	_, err := reg.Describe(ctx, &DescribeInput{Cap: "more0.doc.ingest"})
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", describeTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", describeTestPrefix, err)
	}
}
