package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commsserver "github.com/nats-io/nats-server/v2/server"
	comms "github.com/nats-io/nats.go"
)

// startTestServer starts an in-process NATS server for testing.
func startTestServer(t *testing.T, port int) (*comms.Conn, func()) {
	t.Helper()

	opts := &commsserver.Options{
		Host:   "127.0.0.1",
		Port:   port,
		NoLog:  true,
		NoSigs: true,
	}

	ns, err := commsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - failed to create server: %v", err)
	}

	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("events:comms_publisher_integration_test - server failed to start")
	}

	nc, err := comms.Connect(ns.ClientURL(), comms.Timeout(5*time.Second))
	if err != nil {
		ns.Shutdown()
		t.Fatalf("events:comms_publisher_integration_test - failed to connect: %v", err)
	}

	cleanup := func() {
		nc.Close()
		ns.Shutdown()
		ns.WaitForShutdown()
	}

	return nc, cleanup
}

func TestCommsPublisher_PublishChanged_GranularSubject(t *testing.T) {
	nc, cleanup := startTestServer(t, 14230)
	defer cleanup()

	publisher := NewCommsPublisher(nc, nil)

	// Subscribe to granular subject
	received := make(chan *RegistryChangedEvent, 1)
	sub, err := nc.Subscribe("registry.changed.more0.doc-ingest", func(msg *comms.Msg) {
		var event RegistryChangedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			t.Errorf("events:comms_publisher_integration_test - failed to unmarshal: %v", err)
			return
		}
		received <- &event
	})
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	event := &RegistryChangedEvent{
		App:            "more0",
		Capability:     "doc-ingest",
		ChangedFields:  []string{"version"},
		AffectedMajors: []int{3},
		Revision:       5,
		Etag:           "cap-123-5",
		Timestamp:      "2025-01-01T00:00:00Z",
	}

	err = publisher.PublishChanged(context.Background(), event)
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - PublishChanged failed: %v", err)
	}
	nc.Flush()

	select {
	case got := <-received:
		if got.App != "more0" {
			t.Errorf("events:comms_publisher_integration_test - App = %q, want %q", got.App, "more0")
		}
		if got.Capability != "doc-ingest" {
			t.Errorf("events:comms_publisher_integration_test - Capability = %q, want %q", got.Capability, "doc-ingest")
		}
		if got.Revision != 5 {
			t.Errorf("events:comms_publisher_integration_test - Revision = %d, want 5", got.Revision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("events:comms_publisher_integration_test - timeout waiting for granular event")
	}
}

func TestCommsPublisher_PublishChanged_GlobalSubject(t *testing.T) {
	nc, cleanup := startTestServer(t, 14231)
	defer cleanup()

	publisher := NewCommsPublisher(nc, nil)

	// Subscribe to global change subject
	received := make(chan *RegistryChangedEvent, 1)
	sub, err := nc.Subscribe("registry.changed", func(msg *comms.Msg) {
		var event RegistryChangedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		received <- &event
	})
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	event := &RegistryChangedEvent{
		App:            "system",
		Capability:     "auth",
		ChangedFields:  []string{"status"},
		AffectedMajors: []int{1},
		Revision:       2,
		Etag:           "cap-456-2",
		Timestamp:      "2025-02-01T00:00:00Z",
	}

	err = publisher.PublishChanged(context.Background(), event)
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - PublishChanged failed: %v", err)
	}
	nc.Flush()

	select {
	case got := <-received:
		if got.App != "system" {
			t.Errorf("events:comms_publisher_integration_test - App = %q, want %q", got.App, "system")
		}
		if got.Capability != "auth" {
			t.Errorf("events:comms_publisher_integration_test - Capability = %q, want %q", got.Capability, "auth")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("events:comms_publisher_integration_test - timeout waiting for global event")
	}
}

func TestCommsPublisher_PublishChanged_BothSubjects(t *testing.T) {
	nc, cleanup := startTestServer(t, 14232)
	defer cleanup()

	publisher := NewCommsPublisher(nc, nil)

	granularReceived := make(chan bool, 1)
	globalReceived := make(chan bool, 1)

	sub1, err := nc.Subscribe("registry.changed.more0.test", func(msg *comms.Msg) {
		granularReceived <- true
	})
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - subscribe granular failed: %v", err)
	}
	defer sub1.Unsubscribe()

	sub2, err := nc.Subscribe("registry.changed", func(msg *comms.Msg) {
		globalReceived <- true
	})
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - subscribe global failed: %v", err)
	}
	defer sub2.Unsubscribe()

	event := &RegistryChangedEvent{
		App:            "more0",
		Capability:     "test",
		ChangedFields:  []string{"version"},
		AffectedMajors: []int{1},
		Revision:       1,
		Etag:           "test-1",
		Timestamp:      "2025-01-01T00:00:00Z",
	}

	err = publisher.PublishChanged(context.Background(), event)
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - PublishChanged failed: %v", err)
	}
	nc.Flush()

	// Both subjects should receive the event
	for _, ch := range []struct {
		name string
		ch   chan bool
	}{
		{"granular", granularReceived},
		{"global", globalReceived},
	} {
		select {
		case <-ch.ch:
			// OK
		case <-time.After(5 * time.Second):
			t.Errorf("events:comms_publisher_integration_test - timeout waiting for %s event", ch.name)
		}
	}
}

func TestCommsPublisher_CustomGlobalSubject(t *testing.T) {
	nc, cleanup := startTestServer(t, 14233)
	defer cleanup()

	customSubject := "custom.events.changed"
	publisher := NewCommsPublisher(nc, &CommsPublisherOpts{
		GlobalChangeSubject: customSubject,
	})

	received := make(chan *RegistryChangedEvent, 1)
	sub, err := nc.Subscribe(customSubject, func(msg *comms.Msg) {
		var event RegistryChangedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		received <- &event
	})
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	event := &RegistryChangedEvent{
		App:            "more0",
		Capability:     "custom",
		ChangedFields:  []string{"version"},
		AffectedMajors: []int{1},
		Revision:       1,
		Etag:           "custom-1",
		Timestamp:      "2025-01-01T00:00:00Z",
	}

	err = publisher.PublishChanged(context.Background(), event)
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - PublishChanged failed: %v", err)
	}
	nc.Flush()

	select {
	case got := <-received:
		if got.App != "more0" {
			t.Errorf("events:comms_publisher_integration_test - App = %q, want %q", got.App, "more0")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("events:comms_publisher_integration_test - timeout waiting for custom subject event")
	}
}

func TestCommsPublisher_EventFieldsPreserved(t *testing.T) {
	nc, cleanup := startTestServer(t, 14234)
	defer cleanup()

	publisher := NewCommsPublisher(nc, nil)

	received := make(chan *RegistryChangedEvent, 1)
	sub, err := nc.Subscribe("registry.changed", func(msg *comms.Msg) {
		var event RegistryChangedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		received <- &event
	})
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	major := 3
	event := &RegistryChangedEvent{
		App:             "more0",
		Capability:      "doc.ingest",
		ChangedFields:   []string{"version", "methods", "defaultMajor"},
		NewDefaultMajor: &major,
		AffectedMajors:  []int{2, 3},
		Revision:        10,
		Etag:            "cap-abc-10",
		Timestamp:       "2025-06-15T12:30:00Z",
		Env:             "production",
	}

	err = publisher.PublishChanged(context.Background(), event)
	if err != nil {
		t.Fatalf("events:comms_publisher_integration_test - PublishChanged failed: %v", err)
	}
	nc.Flush()

	select {
	case got := <-received:
		if got.App != "more0" {
			t.Errorf("events:comms_publisher_integration_test - App = %q, want %q", got.App, "more0")
		}
		if got.Capability != "doc.ingest" {
			t.Errorf("events:comms_publisher_integration_test - Capability = %q, want %q", got.Capability, "doc.ingest")
		}
		if len(got.ChangedFields) != 3 {
			t.Errorf("events:comms_publisher_integration_test - ChangedFields len = %d, want 3", len(got.ChangedFields))
		}
		if got.NewDefaultMajor == nil || *got.NewDefaultMajor != 3 {
			t.Errorf("events:comms_publisher_integration_test - NewDefaultMajor unexpected")
		}
		if len(got.AffectedMajors) != 2 {
			t.Errorf("events:comms_publisher_integration_test - AffectedMajors len = %d, want 2", len(got.AffectedMajors))
		}
		if got.Revision != 10 {
			t.Errorf("events:comms_publisher_integration_test - Revision = %d, want 10", got.Revision)
		}
		if got.Etag != "cap-abc-10" {
			t.Errorf("events:comms_publisher_integration_test - Etag = %q, want %q", got.Etag, "cap-abc-10")
		}
		if got.Env != "production" {
			t.Errorf("events:comms_publisher_integration_test - Env = %q, want %q", got.Env, "production")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("events:comms_publisher_integration_test - timeout waiting for event")
	}
}

func TestNewCommsPublisher_NilOpts(t *testing.T) {
	nc, cleanup := startTestServer(t, 14235)
	defer cleanup()

	publisher := NewCommsPublisher(nc, nil)
	if publisher == nil {
		t.Fatal("events:comms_publisher_integration_test - expected non-nil publisher")
	}
	// Default global subject should be used
	if publisher.globalChangeSubject != "registry.changed" {
		t.Errorf("events:comms_publisher_integration_test - globalChangeSubject = %q, want %q",
			publisher.globalChangeSubject, "registry.changed")
	}
}

func TestNewCommsPublisher_EmptyGlobalSubject(t *testing.T) {
	nc, cleanup := startTestServer(t, 14236)
	defer cleanup()

	publisher := NewCommsPublisher(nc, &CommsPublisherOpts{
		GlobalChangeSubject: "",
	})

	// Empty string should use default
	if publisher.globalChangeSubject != "registry.changed" {
		t.Errorf("events:comms_publisher_integration_test - globalChangeSubject = %q, want %q",
			publisher.globalChangeSubject, "registry.changed")
	}
}
