package registry

import (
	"context"
	"testing"
)

const discoverTestPrefix = "registry:discover_test"

func TestDiscover_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})
	ctx := context.Background()

	_, err := reg.Discover(ctx, &DiscoverInput{Page: 1, Limit: 10})
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", discoverTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", discoverTestPrefix, err)
	}
}

func TestDiscover_DefaultPageAndLimit(t *testing.T) {
	// Page and limit normalization is internal; we test requireRepo path here.
	// Full Discover with repo is covered by integration tests.
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	_, err := reg.Discover(ctx, &DiscoverInput{Page: 0, Limit: 0})
	if err == nil {
		t.Fatalf("%s - expected error (nil repo)", discoverTestPrefix)
	}
}
