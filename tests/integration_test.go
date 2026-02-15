//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	commsserver "github.com/nats-io/nats-server/v2/server"
	comms "github.com/nats-io/nats.go"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/dispatcher"
	"github.com/morezero/capabilities-registry/pkg/events"
	"github.com/morezero/capabilities-registry/pkg/registry"
)

const integrationTestPrefix = "tests:integration_test"
const integrationNatsPort = 14241

// Integration tests use DATABASE_URL (e.g. .../registry_test on platform Postgres).
// Create DBs once: scripts/ensure-databases.ps1

func TestIntegration_RegistryWithDB_ResolveDiscoverDescribe(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skipf("%s - DATABASE_URL not set (e.g. .../registry_test; create with scripts/ensure-databases.ps1), skipping", integrationTestPrefix)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("%s - NewPool failed: %v", integrationTestPrefix, err)
	}
	defer pool.Close()

	migrationPath := "migrations"
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		migrationPath = filepath.Join("..", "migrations")
	}
	migrationSQL, err := db.LoadMigrationFiles(migrationPath)
	if err != nil {
		t.Fatalf("%s - LoadMigrationFiles failed: %v", integrationTestPrefix, err)
	}
	if err := db.RunMigrations(ctx, pool, migrationSQL); err != nil {
		t.Fatalf("%s - RunMigrations failed: %v", integrationTestPrefix, err)
	}

	opts := &commsserver.Options{
		Host:   "127.0.0.1",
		Port:   integrationNatsPort,
		NoLog:  true,
		NoSigs: true,
	}
	ns, err := commsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("%s - failed to create NATS server: %v", integrationTestPrefix, err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatalf("%s - NATS server failed to start", integrationTestPrefix)
	}
	defer func() {
		ns.Shutdown()
		ns.WaitForShutdown()
	}()

	nc, err := comms.Connect(ns.ClientURL(), comms.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("%s - failed to connect to NATS: %v", integrationTestPrefix, err)
	}
	defer nc.Close()

	repo := db.NewRepository(pool)
	pub := events.NewCallbackPublisher(func(_ context.Context, _ *events.RegistryChangedEvent) error { return nil })
	reg := registry.NewRegistry(registry.NewRegistryParams{
		Repo:      repo,
		Publisher: pub,
		Config:    registry.DefaultConfig(),
	})
	defer reg.Close()
	disp := dispatcher.NewDispatcher(reg)

	subject := "cap.test.registry.integration.v1"
	_, err = nc.Subscribe(subject, func(msg *comms.Msg) {
		var req dispatcher.RegistryRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp := &dispatcher.RegistryResponse{
				Ok: false,
				Error: &dispatcher.ErrorDetail{Code: "INVALID_REQUEST", Message: "Failed to decode request"},
			}
			data, _ := json.Marshal(resp)
			msg.Respond(data)
			return
		}
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer reqCancel()
		resp := disp.Dispatch(reqCtx, &req)
		data, _ := json.Marshal(resp)
		msg.Respond(data)
	})
	if err != nil {
		t.Fatalf("%s - subscribe failed: %v", integrationTestPrefix, err)
	}

	send := func(req *dispatcher.RegistryRequest) *dispatcher.RegistryResponse {
		data, _ := json.Marshal(req)
		msg, err := nc.Request(subject, data, 10*time.Second)
		if err != nil {
			t.Fatalf("%s - request failed: %v", integrationTestPrefix, err)
		}
		var resp dispatcher.RegistryResponse
		if err := json.Unmarshal(msg.Data, &resp); err != nil {
			t.Fatalf("%s - unmarshal response: %v", integrationTestPrefix, err)
		}
		return &resp
	}

	// 1. Upsert a capability
	upsertParams := map[string]interface{}{
		"app":         "intapp",
		"name":        "integration.cap",
		"description": "Integration test capability",
		"tags":        []string{"integration"},
		"version":     map[string]interface{}{"major": 1, "minor": 0, "patch": 0},
		"methods": []map[string]interface{}{
			{"name": "doWork", "description": "Does work", "modes": []string{"sync"}},
		},
		"setAsDefault": true,
	}
	upsertJSON, _ := json.Marshal(upsertParams)
	resp := send(&dispatcher.RegistryRequest{
		ID:     "int-upsert-1",
		Method: "upsert",
		Params: upsertJSON,
		Ctx:    &dispatcher.InvocationContext{UserID: "00000000-0000-0000-0000-000000000001"},
	})
	if !resp.Ok {
		t.Fatalf("%s - upsert failed: %v", integrationTestPrefix, resp.Error)
	}

	// 2. Resolve
	resp = send(&dispatcher.RegistryRequest{
		ID:     "int-resolve-1",
		Method: "resolve",
		Params: json.RawMessage(`{"cap": "intapp.integration.cap@^1.0.0"}`),
		Ctx:    &dispatcher.InvocationContext{Env: "production"},
	})
	if !resp.Ok {
		t.Fatalf("%s - resolve failed: %v", integrationTestPrefix, resp.Error)
	}
	result, _ := json.Marshal(resp.Result)
	var resolveOut registry.ResolveOutput
	if err := json.Unmarshal(result, &resolveOut); err != nil {
		t.Fatalf("%s - resolve result unmarshal: %v", integrationTestPrefix, err)
	}
	if resolveOut.ResolvedVersion != "1.0.0" {
		t.Errorf("%s - ResolvedVersion = %q, want 1.0.0", integrationTestPrefix, resolveOut.ResolvedVersion)
	}
	if resolveOut.Subject == "" {
		t.Errorf("%s - Subject empty", integrationTestPrefix)
	}

	// 3. Discover
	resp = send(&dispatcher.RegistryRequest{
		ID:     "int-discover-1",
		Method: "discover",
		Params: json.RawMessage(`{"app": "intapp", "page": 1, "limit": 10}`),
	})
	if !resp.Ok {
		t.Fatalf("%s - discover failed: %v", integrationTestPrefix, resp.Error)
	}
	var discoverOut registry.DiscoverOutput
	result, _ = json.Marshal(resp.Result)
	if err := json.Unmarshal(result, &discoverOut); err != nil {
		t.Fatalf("%s - discover result unmarshal: %v", integrationTestPrefix, err)
	}
	if discoverOut.Pagination.Total < 1 {
		t.Errorf("%s - discover total = %d, want >= 1", integrationTestPrefix, discoverOut.Pagination.Total)
	}

	// 4. Describe
	resp = send(&dispatcher.RegistryRequest{
		ID:     "int-describe-1",
		Method: "describe",
		Params: json.RawMessage(`{"cap": "intapp.integration.cap"}`),
	})
	if !resp.Ok {
		t.Fatalf("%s - describe failed: %v", integrationTestPrefix, resp.Error)
	}
	var describeOut registry.DescribeOutput
	result, _ = json.Marshal(resp.Result)
	if err := json.Unmarshal(result, &describeOut); err != nil {
		t.Fatalf("%s - describe result unmarshal: %v", integrationTestPrefix, err)
	}
	if describeOut.Cap != "intapp.integration.cap" {
		t.Errorf("%s - Describe.Cap = %q, want intapp.integration.cap", integrationTestPrefix, describeOut.Cap)
	}
	if len(describeOut.Methods) < 1 {
		t.Errorf("%s - expected at least 1 method", integrationTestPrefix)
	}

	// 5. Health
	resp = send(&dispatcher.RegistryRequest{
		ID:     "int-health-1",
		Method: "health",
		Params: json.RawMessage(`{}`),
	})
	if !resp.Ok {
		t.Fatalf("%s - health failed: %v", integrationTestPrefix, resp.Error)
	}
	var healthOut registry.HealthOutput
	result, _ = json.Marshal(resp.Result)
	if err := json.Unmarshal(result, &healthOut); err != nil {
		t.Fatalf("%s - health result unmarshal: %v", integrationTestPrefix, err)
	}
	if healthOut.Status != "healthy" {
		t.Errorf("%s - health status = %q, want healthy", integrationTestPrefix, healthOut.Status)
	}
	if !healthOut.Checks.Database {
		t.Errorf("%s - health database check should be true", integrationTestPrefix)
	}

	// 6. ListMajors
	resp = send(&dispatcher.RegistryRequest{
		ID:     "int-listmajors-1",
		Method: "listMajors",
		Params: json.RawMessage(`{"cap": "intapp.integration.cap", "includeInactive": true}`),
	})
	if !resp.Ok {
		t.Fatalf("%s - listMajors failed: %v", integrationTestPrefix, resp.Error)
	}
	var listOut registry.ListMajorsOutput
	result, _ = json.Marshal(resp.Result)
	if err := json.Unmarshal(result, &listOut); err != nil {
		t.Fatalf("%s - listMajors result unmarshal: %v", integrationTestPrefix, err)
	}
	if len(listOut.Majors) < 1 {
		t.Errorf("%s - listMajors expected >= 1 major", integrationTestPrefix)
	}
}
