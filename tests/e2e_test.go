// Package tests contains end-to-end tests for the capabilities-registry.
// These tests start an embedded NATS server and test the full request/response
// flow through the dispatcher, simulating real client interactions.
package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commsserver "github.com/nats-io/nats-server/v2/server"
	comms "github.com/nats-io/nats.go"

	"github.com/morezero/capabilities-registry/pkg/dispatcher"
	"github.com/morezero/capabilities-registry/pkg/events"
	"github.com/morezero/capabilities-registry/pkg/registry"
)

const (
	testRegistrySubject = "cap.test.registry.v1"
	testPort            = 14240
)

// testEnv holds the test environment for E2E tests.
type testEnv struct {
	nc       *comms.Conn
	ns       *commsserver.Server
	disp     *dispatcher.Dispatcher
	reg      *registry.Registry
	captured []*events.RegistryChangedEvent
}

// setupE2E starts an embedded NATS server and sets up the dispatcher pipeline.
// Note: These tests use a NoOp publisher and nil repo to test the NATS transport
// and dispatch routing without requiring a database.
func setupE2E(t *testing.T) *testEnv {
	t.Helper()

	// Start embedded NATS
	opts := &commsserver.Options{
		Host:   "127.0.0.1",
		Port:   testPort,
		NoLog:  true,
		NoSigs: true,
	}

	ns, err := commsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("e2e_test - failed to create NATS server: %v", err)
	}

	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("e2e_test - NATS server failed to start")
	}

	nc, err := comms.Connect(ns.ClientURL(), comms.Timeout(5*time.Second))
	if err != nil {
		ns.Shutdown()
		t.Fatalf("e2e_test - failed to connect: %v", err)
	}

	env := &testEnv{
		nc: nc,
		ns: ns,
	}

	// Create registry with callback publisher (captures events)
	pub := events.NewCallbackPublisher(func(_ context.Context, event *events.RegistryChangedEvent) error {
		env.captured = append(env.captured, event)
		return nil
	})

	// Registry with nil repo (can't do DB ops, but can test dispatch routing)
	reg := registry.NewRegistry(registry.NewRegistryParams{
		Repo:      nil,
		Publisher: pub,
		Config:    registry.DefaultConfig(),
	})
	env.reg = reg

	disp := dispatcher.NewDispatcher(reg)
	env.disp = disp

	// Subscribe to registry subject (simulates the server subscription)
	_, err = nc.Subscribe(testRegistrySubject, func(msg *comms.Msg) {
		var req dispatcher.RegistryRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp := &dispatcher.RegistryResponse{
				Ok: false,
				Error: &dispatcher.ErrorDetail{
					Code:    "INVALID_REQUEST",
					Message: "Failed to decode request",
				},
			}
			data, _ := json.Marshal(resp)
			msg.Respond(data)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp := disp.Dispatch(ctx, &req)
		data, _ := json.Marshal(resp)
		msg.Respond(data)
	})
	if err != nil {
		nc.Close()
		ns.Shutdown()
		t.Fatalf("e2e_test - failed to subscribe: %v", err)
	}

	t.Cleanup(func() {
		nc.Close()
		ns.Shutdown()
		ns.WaitForShutdown()
	})

	return env
}

// sendRequest sends a registry request over NATS and returns the response.
func sendRequest(t *testing.T, nc *comms.Conn, req *dispatcher.RegistryRequest) *dispatcher.RegistryResponse {
	t.Helper()

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("e2e_test - failed to marshal request: %v", err)
	}

	msg, err := nc.Request(testRegistrySubject, data, 10*time.Second)
	if err != nil {
		t.Fatalf("e2e_test - request failed: %v", err)
	}

	var resp dispatcher.RegistryResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		t.Fatalf("e2e_test - failed to unmarshal response: %v", err)
	}

	return &resp
}

func TestE2E_UnknownMethod(t *testing.T) {
	env := setupE2E(t)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-1",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "nonexistent",
		Params: json.RawMessage(`{}`),
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false for unknown method")
	}
	if resp.ID != "e2e-1" {
		t.Errorf("e2e_test - ID = %q, want %q", resp.ID, "e2e-1")
	}
	if resp.Error == nil {
		t.Fatal("e2e_test - expected error, got nil")
	}
	if resp.Error.Code != "METHOD_NOT_FOUND" {
		t.Errorf("e2e_test - error code = %q, want %q", resp.Error.Code, "METHOD_NOT_FOUND")
	}
	if resp.Error.Retryable {
		t.Error("e2e_test - METHOD_NOT_FOUND should not be retryable")
	}
}

func TestE2E_HealthCheck(t *testing.T) {
	env := setupE2E(t)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-health-1",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "health",
		Params: json.RawMessage(`{}`),
	}

	resp := sendRequest(t, env.nc, req)

	if !resp.Ok {
		t.Errorf("e2e_test - expected Ok=true for health, got error: %v", resp.Error)
	}
	if resp.ID != "e2e-health-1" {
		t.Errorf("e2e_test - ID = %q, want %q", resp.ID, "e2e-health-1")
	}
	if resp.Result == nil {
		t.Fatal("e2e_test - expected result, got nil")
	}

	// The health check with nil repo will fail DB check but still return a result
	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("e2e_test - failed to marshal result: %v", err)
	}

	var health registry.HealthOutput
	if err := json.Unmarshal(resultJSON, &health); err != nil {
		t.Fatalf("e2e_test - failed to unmarshal health: %v", err)
	}

	if health.Timestamp == "" {
		t.Error("e2e_test - expected non-empty timestamp")
	}
}

func TestE2E_ResolveWithoutDB(t *testing.T) {
	env := setupE2E(t)

	// This should fail because there's no DB, but we should get a structured error
	req := &dispatcher.RegistryRequest{
		ID:     "e2e-resolve-1",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "resolve",
		Params: json.RawMessage(`{"cap": "more0.doc.ingest@^3.0.0"}`),
		Ctx: &dispatcher.InvocationContext{
			TenantID: "tenant-1",
			Env:      "production",
		},
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB)")
	}
	if resp.Error == nil {
		t.Fatal("e2e_test - expected error, got nil")
	}
	// Should be INTERNAL_ERROR since repo is nil
	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("e2e_test - error code = %q, want %q", resp.Error.Code, "INTERNAL_ERROR")
	}
}

func TestE2E_DiscoverWithoutDB(t *testing.T) {
	env := setupE2E(t)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-discover-1",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "discover",
		Params: json.RawMessage(`{"app": "more0", "page": 1, "limit": 10}`),
	}

	resp := sendRequest(t, env.nc, req)

	// Should fail because DB is nil
	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB)")
	}
	if resp.ID != "e2e-discover-1" {
		t.Errorf("e2e_test - ID = %q, want %q", resp.ID, "e2e-discover-1")
	}
}

func TestE2E_DescribeWithoutDB(t *testing.T) {
	env := setupE2E(t)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-describe-1",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "describe",
		Params: json.RawMessage(`{"cap": "more0.doc.ingest"}`),
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB)")
	}
}

func TestE2E_InvalidJSON(t *testing.T) {
	env := setupE2E(t)

	// Send invalid JSON directly
	msg, err := env.nc.Request(testRegistrySubject, []byte(`{invalid json`), 10*time.Second)
	if err != nil {
		t.Fatalf("e2e_test - request failed: %v", err)
	}

	var resp dispatcher.RegistryResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		t.Fatalf("e2e_test - failed to unmarshal response: %v", err)
	}

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false for invalid JSON")
	}
	if resp.Error == nil {
		t.Fatal("e2e_test - expected error for invalid JSON")
	}
	if resp.Error.Code != "INVALID_REQUEST" {
		t.Errorf("e2e_test - error code = %q, want %q", resp.Error.Code, "INVALID_REQUEST")
	}
}

func TestE2E_InvalidMethodParams(t *testing.T) {
	env := setupE2E(t)

	// Valid request envelope but invalid params for the method
	req := &dispatcher.RegistryRequest{
		ID:     "e2e-invalid-params",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "resolve",
		Params: json.RawMessage(`"not-an-object"`),
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false for invalid params")
	}
	if resp.Error == nil {
		t.Fatal("e2e_test - expected error for invalid params")
	}
	if resp.Error.Code != "INVALID_ARGUMENT" {
		t.Errorf("e2e_test - error code = %q, want %q", resp.Error.Code, "INVALID_ARGUMENT")
	}
}

func TestE2E_RequestIDPreservation(t *testing.T) {
	env := setupE2E(t)

	ids := []string{"req-001", "req-002", "unique-xyz-789", ""}
	for _, id := range ids {
		req := &dispatcher.RegistryRequest{
			ID:     id,
			Method: "nonexistent",
			Params: json.RawMessage(`{}`),
		}

		resp := sendRequest(t, env.nc, req)

		if resp.ID != id {
			t.Errorf("e2e_test - ID = %q, want %q", resp.ID, id)
		}
	}
}

func TestE2E_ContextPropagation(t *testing.T) {
	env := setupE2E(t)

	// Test that context fields are passed through
	req := &dispatcher.RegistryRequest{
		ID:     "e2e-ctx-1",
		Method: "resolve",
		Params: json.RawMessage(`{"cap": "more0.test"}`),
		Ctx: &dispatcher.InvocationContext{
			TenantID:      "tenant-123",
			UserID:        "user-456",
			Env:           "staging",
			Features:      []string{"beta"},
			DeadlineMs:    5000,
		},
	}

	resp := sendRequest(t, env.nc, req)

	// We can't test the result itself (no DB), but we verify the request/response cycle works
	if resp.ID != "e2e-ctx-1" {
		t.Errorf("e2e_test - ID = %q, want %q", resp.ID, "e2e-ctx-1")
	}
}

func TestE2E_ConcurrentRequests(t *testing.T) {
	env := setupE2E(t)

	const numRequests = 20
	results := make(chan *dispatcher.RegistryResponse, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			req := &dispatcher.RegistryRequest{
				ID:     "concurrent-" + string(rune('a'+idx%26)),
				Method: "health",
				Params: json.RawMessage(`{}`),
			}
			resp := sendRequest(t, env.nc, req)
			results <- resp
		}(i)
	}

	for i := 0; i < numRequests; i++ {
		select {
		case resp := <-results:
			if !resp.Ok {
				t.Errorf("e2e_test - concurrent request failed: %v", resp.Error)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("e2e_test - timeout waiting for concurrent request %d", i)
		}
	}
}

func TestE2E_AllDispatchMethods_InvalidParams(t *testing.T) {
	env := setupE2E(t)

	methods := []string{
		"resolve", "discover", "describe", "upsert",
		"setDefaultMajor", "deprecate", "disable", "listMajors",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := &dispatcher.RegistryRequest{
				ID:     "e2e-" + method,
				Method: method,
				Params: json.RawMessage(`"invalid"`),
			}

			resp := sendRequest(t, env.nc, req)

			if resp.Ok {
				t.Errorf("e2e_test - expected Ok=false for invalid params on %s", method)
			}
			if resp.Error == nil {
				t.Fatalf("e2e_test - expected error for %s, got nil", method)
			}
			if resp.Error.Code != "INVALID_ARGUMENT" {
				t.Errorf("e2e_test - %s error code = %q, want %q", method, resp.Error.Code, "INVALID_ARGUMENT")
			}
		})
	}
}

func TestE2E_UpsertWithoutDB(t *testing.T) {
	env := setupE2E(t)

	params := map[string]interface{}{
		"app":         "more0",
		"name":        "test-cap",
		"description": "A test capability",
		"tags":        []string{"test"},
		"version": map[string]interface{}{
			"major": 1,
			"minor": 0,
			"patch": 0,
		},
		"methods": []map[string]interface{}{
			{
				"name":        "doSomething",
				"description": "Does something",
				"modes":       []string{"sync"},
			},
		},
		"setAsDefault": true,
	}

	paramsJSON, _ := json.Marshal(params)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-upsert-1",
		Type:   "invoke",
		Cap:    "system.registry",
		Method: "upsert",
		Params: json.RawMessage(paramsJSON),
		Ctx: &dispatcher.InvocationContext{
			UserID: "test-user",
		},
	}

	resp := sendRequest(t, env.nc, req)

	// Should fail because no DB, but should be a structured error
	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB for upsert)")
	}
	if resp.Error == nil {
		t.Fatal("e2e_test - expected error")
	}
}

func TestE2E_SetDefaultMajorWithoutDB(t *testing.T) {
	env := setupE2E(t)

	params := map[string]interface{}{
		"cap":   "more0.doc.ingest",
		"major": 3,
		"env":   "production",
	}

	paramsJSON, _ := json.Marshal(params)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-setdefault-1",
		Method: "setDefaultMajor",
		Params: json.RawMessage(paramsJSON),
		Ctx:    &dispatcher.InvocationContext{UserID: "admin"},
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB)")
	}
}

func TestE2E_DeprecateWithoutDB(t *testing.T) {
	env := setupE2E(t)

	params := map[string]interface{}{
		"cap":    "more0.doc.ingest",
		"reason": "Replaced by v2",
	}

	paramsJSON, _ := json.Marshal(params)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-deprecate-1",
		Method: "deprecate",
		Params: json.RawMessage(paramsJSON),
		Ctx:    &dispatcher.InvocationContext{UserID: "admin"},
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB)")
	}
}

func TestE2E_ListMajorsWithoutDB(t *testing.T) {
	env := setupE2E(t)

	params := map[string]interface{}{
		"cap":             "more0.doc.ingest",
		"includeInactive": true,
	}

	paramsJSON, _ := json.Marshal(params)

	req := &dispatcher.RegistryRequest{
		ID:     "e2e-listmajors-1",
		Method: "listMajors",
		Params: json.RawMessage(paramsJSON),
	}

	resp := sendRequest(t, env.nc, req)

	if resp.Ok {
		t.Error("e2e_test - expected Ok=false (no DB)")
	}
}
