package registry

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

const healthTestPrefix = "registry:health_test"

func TestHealth_NilRepo_ReturnsUnhealthy(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{
		Repo:      nil,
		Publisher: nil,
		Config:    DefaultConfig(),
	})
	ctx := context.Background()

	out := reg.Health(ctx)

	if out.Status != "unhealthy" {
		t.Errorf("%s - Status = %q, want unhealthy", healthTestPrefix, out.Status)
	}
	if out.Checks.Database {
		t.Errorf("%s - expected Database check false when repo is nil", healthTestPrefix)
	}
	if out.Timestamp == "" {
		t.Errorf("%s - expected non-empty Timestamp", healthTestPrefix)
	}
	// Timestamp should be RFC3339
	if _, err := time.Parse(time.RFC3339, out.Timestamp); err != nil {
		t.Errorf("%s - Timestamp not RFC3339: %v", healthTestPrefix, err)
	}
}

func TestHealth_OutputShape(t *testing.T) {
	reg := NewRegistry(NewRegistryParams{Repo: nil, Publisher: nil, Config: DefaultConfig()})
	ctx := context.Background()

	out := reg.Health(ctx)

	// Ensure JSON marshalling works and shape is stable
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("%s - marshal failed: %v", healthTestPrefix, err)
	}
	var decoded HealthOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("%s - unmarshal failed: %v", healthTestPrefix, err)
	}
	if decoded.Status != out.Status {
		t.Errorf("%s - round-trip Status = %q, want %q", healthTestPrefix, decoded.Status, out.Status)
	}
	if decoded.Checks.Database != out.Checks.Database {
		t.Errorf("%s - round-trip Database = %v, want %v", healthTestPrefix, decoded.Checks.Database, out.Checks.Database)
	}
}
