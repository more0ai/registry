package registry

import "testing"

func TestRegistryError(t *testing.T) {
	err := NewRegistryError("NOT_FOUND", "Capability not found")

	if err.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", err.Code)
	}
	if err.Message != "Capability not found" {
		t.Errorf("expected 'Capability not found', got %s", err.Message)
	}
	if err.Error() != "NOT_FOUND: Capability not found" {
		t.Errorf("expected 'NOT_FOUND: Capability not found', got %s", err.Error())
	}
}
