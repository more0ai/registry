package registry

import (
	"context"
	"testing"
)

const listMajorsTestPrefix = "registry:list_majors_test"

func TestListMajors_RequireRepo(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()
	_, err := reg.ListMajors(ctx, &ListMajorsInput{Cap: "more0.test"})
	if err == nil {
		t.Fatalf("%s - expected error when repo is nil", listMajorsTestPrefix)
	}
	if regErr, ok := err.(*RegistryError); !ok || regErr.Code != "INTERNAL_ERROR" {
		t.Errorf("%s - expected INTERNAL_ERROR, got %v", listMajorsTestPrefix, err)
	}
}
