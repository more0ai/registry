package registry

import (
	"context"
	"testing"
)

const deprecateTestPrefix = "registry:deprecate_test"

func TestDeprecate_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	_, err := reg.Deprecate(ctx, &DeprecateInput{Cap: "more0.test"}, "test-user")
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", deprecateTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", deprecateTestPrefix, err)
	}
}
