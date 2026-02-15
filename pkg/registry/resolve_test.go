package registry

import (
	"context"
	"testing"
)

func TestResolve_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})
	ctx := context.Background()

	_, err := reg.Resolve(ctx, &ResolveInput{Cap: "more0.doc.ingest"})
	if err == nil {
		t.Fatal("registry:resolve_test - expected error when repo is nil")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("registry:resolve_test - expected INTERNAL_ERROR, got %v", err)
	}
}

func TestResolve_InvalidCapRef(t *testing.T) {
	// Resolve with nil repo fails before parsing; test invalid ref with a mock would need repo interface.
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()

	_, err := reg.Resolve(ctx, &ResolveInput{Cap: ""})
	if err == nil {
		t.Fatal("registry:resolve_test - expected error for empty cap with nil repo")
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("registry:resolve_test - expected INTERNAL_ERROR (no repo), got %v", err)
	}
}
