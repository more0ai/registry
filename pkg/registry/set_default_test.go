package registry

import (
	"context"
	"testing"
)

const setDefaultTestPrefix = "registry:set_default_test"

func TestSetDefaultMajor_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	_, err := reg.SetDefaultMajor(ctx, &SetDefaultMajorInput{Cap: "more0.test", Major: 1}, "test-user")
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", setDefaultTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", setDefaultTestPrefix, err)
	}
}
