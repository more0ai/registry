package registry

import (
	"context"
	"testing"
)

const disableTestPrefix = "registry:disable_test"

func TestDisable_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	_, err := reg.Disable(ctx, &DisableInput{Cap: "more0.test"}, "test-user")
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", disableTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", disableTestPrefix, err)
	}
}
