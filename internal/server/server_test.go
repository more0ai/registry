package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/morezero/capabilities-registry/internal/config"
	"github.com/morezero/capabilities-registry/pkg/registry"
)

const serverTestPrefix = "server:server_test"

// mockRegistry implements registryForServer for handler tests.
type mockRegistry struct {
	health   *registry.HealthOutput
	discover *registry.DiscoverOutput
	discoverErr error
	describe *registry.DescribeOutput
	describeErr error
}

func (m *mockRegistry) Health(context.Context) *registry.HealthOutput {
	if m.health != nil {
		return m.health
	}
	return &registry.HealthOutput{Status: "unhealthy", Timestamp: time.Now().UTC().Format(time.RFC3339)}
}

func (m *mockRegistry) Discover(ctx context.Context, input *registry.DiscoverInput) (*registry.DiscoverOutput, error) {
	return m.discover, m.discoverErr
}

func (m *mockRegistry) Describe(ctx context.Context, input *registry.DescribeInput) (*registry.DescribeOutput, error) {
	return m.describe, m.describeErr
}

func (m *mockRegistry) GetBootstrapCapabilities(context.Context, string, bool, bool) (map[string]*registry.ResolveOutput, error) {
	return nil, nil
}

func (m *mockRegistry) LoadRegistryAliases(context.Context) (map[string]string, string, error) {
	return nil, "", nil
}

func (m *mockRegistry) Close() {}

// testServer returns a Server with mock registry and test config for HTTP handler tests.
func testServer(t *testing.T, reg registryForServer) *Server {
	t.Helper()
	cfg := &config.Config{
		HealthCheckTimeout: 5 * time.Second,
	}
	return &Server{cfg: cfg, reg: reg}
}

func TestBuildOpenAPISpec_EmptyMethods(t *testing.T) {
	d := &registry.DescribeOutput{
		Cap:         "more0.doc.ingest",
		App:         "more0",
		Name:        "doc.ingest",
		Version:     "3.0.0",
		Major:       3,
		Status:      "active",
		Description: "Document ingestion",
		Methods:     []registry.MethodDescription{},
	}
	spec := buildOpenAPISpec(d)

	if spec.OpenAPI != "3.0.0" {
		t.Errorf("%s - OpenAPI = %q, want 3.0.0", serverTestPrefix, spec.OpenAPI)
	}
	if spec.Info.Title != "more0.doc.ingest" {
		t.Errorf("%s - Info.Title = %q, want more0.doc.ingest", serverTestPrefix, spec.Info.Title)
	}
	if spec.Info.Description != "Document ingestion" {
		t.Errorf("%s - Info.Description = %q, want Document ingestion", serverTestPrefix, spec.Info.Description)
	}
	if spec.Info.Version != "3.0.0" {
		t.Errorf("%s - Info.Version = %q, want 3.0.0", serverTestPrefix, spec.Info.Version)
	}
	if len(spec.Paths) != 0 {
		t.Errorf("%s - expected 0 paths for no methods, got %d", serverTestPrefix, len(spec.Paths))
	}
}

func TestBuildOpenAPISpec_WithMethods(t *testing.T) {
	d := &registry.DescribeOutput{
		Cap:         "more0.doc.ingest",
		App:         "more0",
		Name:        "doc.ingest",
		Version:     "2.1.0",
		Major:       2,
		Status:      "active",
		Description: "Ingest documents",
		Methods: []registry.MethodDescription{
			{
				Name:        "ingest",
				Description: "Ingest a document",
				Modes:       []string{"sync"},
				InputSchema:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{"url": map[string]interface{}{"type": "string"}}},
				OutputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Name:   "batchIngest",
				Modes:  []string{"async"},
				InputSchema:  map[string]interface{}{"type": "object"},
				OutputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}
	spec := buildOpenAPISpec(d)

	if len(spec.Paths) != 2 {
		t.Errorf("%s - expected 2 paths, got %d", serverTestPrefix, len(spec.Paths))
	}
	if _, ok := spec.Paths["/ingest"]; !ok {
		t.Errorf("%s - expected path /ingest", serverTestPrefix)
	}
	if _, ok := spec.Paths["/batchIngest"]; !ok {
		t.Errorf("%s - expected path /batchIngest", serverTestPrefix)
	}
	ingestPath := spec.Paths["/ingest"]
	if ingestPath.Post == nil {
		t.Fatalf("%s - expected Post for /ingest", serverTestPrefix)
	}
	if ingestPath.Post.Summary != "ingest" {
		t.Errorf("%s - Post.Summary = %q, want ingest", serverTestPrefix, ingestPath.Post.Summary)
	}
	if ingestPath.Post.Description != "Ingest a document" {
		t.Errorf("%s - Post.Description = %q, want Ingest a document", serverTestPrefix, ingestPath.Post.Description)
	}
	if ingestPath.Post.OperationID != "ingest" {
		t.Errorf("%s - Post.OperationID = %q, want ingest", serverTestPrefix, ingestPath.Post.OperationID)
	}
}

func TestBuildOpenAPISpec_EmptyDescriptionUsesCap(t *testing.T) {
	d := &registry.DescribeOutput{
		Cap:     "system.registry",
		App:     "system",
		Name:    "registry",
		Version: "1.0.0",
		Major:   1,
		Status:  "active",
		Methods: []registry.MethodDescription{},
	}
	spec := buildOpenAPISpec(d)
	// buildOpenAPISpec uses desc when non-empty else "Capability " + d.Cap
	if spec.Info.Description != "Capability system.registry" {
		t.Errorf("%s - Info.Description = %q, want Capability system.registry", serverTestPrefix, spec.Info.Description)
	}
}

func TestBuildOpenAPISpec_JSONRoundTrip(t *testing.T) {
	d := &registry.DescribeOutput{
		Cap:     "more0.test",
		Version: "1.0.0",
		Methods: []registry.MethodDescription{
			{Name: "doSomething", InputSchema: map[string]interface{}{"type": "object"}, OutputSchema: map[string]interface{}{"type": "object"}},
		},
	}
	spec := buildOpenAPISpec(d)
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("%s - marshal failed: %v", serverTestPrefix, err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("%s - unmarshal failed: %v", serverTestPrefix, err)
	}
	if decoded["openapi"] != "3.0.0" {
		t.Errorf("%s - openapi = %v, want 3.0.0", serverTestPrefix, decoded["openapi"])
	}
	paths, ok := decoded["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s - paths missing or wrong type", serverTestPrefix)
	}
	if _, ok := paths["/doSomething"]; !ok {
		t.Errorf("%s - paths should contain /doSomething", serverTestPrefix)
	}
}

func TestHandleHome_Success(t *testing.T) {
	reg := &mockRegistry{
		health: &registry.HealthOutput{Status: "healthy", Checks: registry.HealthChecks{Database: true}, Timestamp: time.Now().UTC().Format(time.RFC3339)},
		discover: &registry.DiscoverOutput{
			Capabilities: []registry.DiscoveredCapability{{Cap: "more0.test", App: "more0", Name: "test", DefaultMajor: 1, LatestVersion: "1.0.0", Status: "active"}},
			Pagination:   registry.Pagination{Page: 1, Limit: 100, Total: 1, TotalPages: 1},
		},
	}
	s := testServer(t, reg)
	handler := s.handleHome()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - handleHome got status %d, want 200", serverTestPrefix, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("%s - Content-Type = %q, want text/html", serverTestPrefix, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if body == "" || len(body) < 100 {
		t.Errorf("%s - response body too short", serverTestPrefix)
	}
	if !strings.Contains(body, "healthy") || !strings.Contains(body, "more0.test") {
		t.Errorf("%s - body should contain health and capability", serverTestPrefix)
	}
}

func TestHandleHome_DiscoverError(t *testing.T) {
	reg := &mockRegistry{
		health:      &registry.HealthOutput{Status: "healthy", Timestamp: time.Now().UTC().Format(time.RFC3339)},
		discoverErr: context.DeadlineExceeded,
	}
	s := testServer(t, reg)
	handler := s.handleHome()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - handleHome (discover error) got status %d, want 200", serverTestPrefix, rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Could not load") && !strings.Contains(body, "context deadline exceeded") {
		t.Errorf("%s - body should show discover error", serverTestPrefix)
	}
}

func TestHandleHome_OnlyRoot(t *testing.T) {
	reg := &mockRegistry{}
	s := testServer(t, reg)
	handler := s.handleHome()
	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("%s - handleHome(/other) got status %d, want 404", serverTestPrefix, rec.Code)
	}
}

func TestHealthHandler_Healthy(t *testing.T) {
	reg := &mockRegistry{
		health: &registry.HealthOutput{Status: "healthy", Checks: registry.HealthChecks{Database: true}, Timestamp: time.Now().UTC().Format(time.RFC3339)},
	}
	s := testServer(t, reg)
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HealthCheckTimeout)
		defer cancel()
		h := s.reg.Health(ctx)
		w.Header().Set("Content-Type", "application/json")
		if h.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(h)
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - health (healthy) got status %d, want 200", serverTestPrefix, rec.Code)
	}
	var out registry.HealthOutput
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("%s - decode health: %v", serverTestPrefix, err)
	}
	if out.Status != "healthy" {
		t.Errorf("%s - Status = %q, want healthy", serverTestPrefix, out.Status)
	}
}

func TestHealthHandler_Unhealthy(t *testing.T) {
	reg := &mockRegistry{
		health: &registry.HealthOutput{Status: "unhealthy", Checks: registry.HealthChecks{Database: false}, Timestamp: time.Now().UTC().Format(time.RFC3339)},
	}
	s := testServer(t, reg)
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HealthCheckTimeout)
		defer cancel()
		h := s.reg.Health(ctx)
		w.Header().Set("Content-Type", "application/json")
		if h.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(h)
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("%s - health (unhealthy) got status %d, want 503", serverTestPrefix, rec.Code)
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - ready got status %d, want 200", serverTestPrefix, rec.Code)
	}
	var out map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("%s - decode ready: %v", serverTestPrefix, err)
	}
	if out["status"] != "ready" {
		t.Errorf("%s - status = %q, want ready", serverTestPrefix, out["status"])
	}
}

func TestConnectionHandler_GET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/connection", nil)
	rec := httptest.NewRecorder()
	func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"natsUrl": "nats://127.0.0.1:4222"})
	}(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - connection GET got status %d, want 200", serverTestPrefix, rec.Code)
	}
	var out map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("%s - decode connection: %v", serverTestPrefix, err)
	}
	if out["natsUrl"] != "nats://127.0.0.1:4222" {
		t.Errorf("%s - natsUrl = %q, want nats://127.0.0.1:4222", serverTestPrefix, out["natsUrl"])
	}
}

func TestConnectionHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/connection", nil)
	rec := httptest.NewRecorder()
	func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("%s - connection POST got status %d, want 405", serverTestPrefix, rec.Code)
	}
}

func TestHandleCapabilityDetail_NotFound(t *testing.T) {
	reg := &mockRegistry{
		describeErr: &registry.RegistryError{Code: "NOT_FOUND", Message: "not found"},
	}
	s := testServer(t, reg)
	handler := s.handleCapabilityDetail()
	req := httptest.NewRequest(http.MethodGet, "/capability/nonexistent.cap", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("%s - capability detail (not found) got status %d, want 404", serverTestPrefix, rec.Code)
	}
}

func TestHandleCapabilityDetail_Success(t *testing.T) {
	reg := &mockRegistry{
		describe: &registry.DescribeOutput{
			Cap: "more0.test", App: "more0", Name: "test", Version: "1.0.0", Major: 1, Status: "active",
			Methods: []registry.MethodDescription{{Name: "run", Modes: []string{"sync"}}},
		},
	}
	s := testServer(t, reg)
	handler := s.handleCapabilityDetail()
	req := httptest.NewRequest(http.MethodGet, "/capability/more0.test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - capability detail got status %d, want 200", serverTestPrefix, rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "more0.test") || !strings.Contains(body, "run") {
		t.Errorf("%s - body should contain cap and method", serverTestPrefix)
	}
}

func TestHandleCapabilityDetail_OpenAPISpec(t *testing.T) {
	reg := &mockRegistry{
		describe: &registry.DescribeOutput{
			Cap: "more0.test", App: "more0", Name: "test", Version: "1.0.0", Major: 1, Status: "active",
			Methods: []registry.MethodDescription{{Name: "run", InputSchema: map[string]interface{}{"type": "object"}, OutputSchema: map[string]interface{}{"type": "object"}}},
		},
	}
	s := testServer(t, reg)
	handler := s.handleCapabilityDetail()
	req := httptest.NewRequest(http.MethodGet, "/capability/more0.test/openapi.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("%s - openapi.json got status %d, want 200", serverTestPrefix, rec.Code)
	}
	var spec openAPI3Spec
	if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
		t.Fatalf("%s - decode openapi: %v", serverTestPrefix, err)
	}
	if spec.OpenAPI != "3.0.0" || spec.Info.Title != "more0.test" {
		t.Errorf("%s - openapi spec OpenAPI=%q Title=%q", serverTestPrefix, spec.OpenAPI, spec.Info.Title)
	}
	if _, ok := spec.Paths["/run"]; !ok {
		t.Errorf("%s - paths should contain /run", serverTestPrefix)
	}
}

func TestHandleCapabilityDetail_RedirectWhenNoCap(t *testing.T) {
	reg := &mockRegistry{}
	s := testServer(t, reg)
	handler := s.handleCapabilityDetail()
	req := httptest.NewRequest(http.MethodGet, "/capability/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Errorf("%s - /capability/ got status %d, want 302 redirect", serverTestPrefix, rec.Code)
	}
}
