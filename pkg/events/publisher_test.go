package events

import (
	"context"
	"testing"
)

func TestNoOpPublisher(t *testing.T) {
	pub := &NoOpPublisher{}
	err := pub.PublishChanged(context.Background(), &RegistryChangedEvent{
		App:        "more0",
		Capability: "test",
		Revision:   1,
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCallbackPublisher(t *testing.T) {
	var captured *RegistryChangedEvent

	pub := NewCallbackPublisher(func(_ context.Context, event *RegistryChangedEvent) error {
		captured = event
		return nil
	})

	event := &RegistryChangedEvent{
		App:            "more0",
		Capability:     "test",
		ChangedFields:  []string{"version"},
		AffectedMajors: []int{1},
		Revision:       5,
		Etag:           "abc-5",
		Timestamp:      "2025-01-01T00:00:00Z",
	}

	err := pub.PublishChanged(context.Background(), event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if captured == nil {
		t.Fatal("expected callback to be called")
	}
	if captured.App != "more0" {
		t.Errorf("expected app more0, got %s", captured.App)
	}
	if captured.Revision != 5 {
		t.Errorf("expected revision 5, got %d", captured.Revision)
	}
}
