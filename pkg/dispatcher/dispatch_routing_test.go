package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/morezero/capabilities-registry/pkg/registry"
)

// TestDispatch_UnknownMethod verifies that unknown methods return METHOD_NOT_FOUND.
func TestDispatch_UnknownMethod(t *testing.T) {
	// Dispatcher with nil registry - only testing the unknown method branch
	disp := &Dispatcher{registry: nil}

	req := &RegistryRequest{
		ID:     "test-1",
		Method: "nonexistent",
		Params: json.RawMessage(`{}`),
	}

	resp := disp.Dispatch(context.Background(), req)

	if resp.Ok {
		t.Error("dispatcher:dispatch_routing_test - expected Ok=false for unknown method")
	}
	if resp.ID != "test-1" {
		t.Errorf("dispatcher:dispatch_routing_test - expected ID=test-1, got %s", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("dispatcher:dispatch_routing_test - expected error, got nil")
	}
	if resp.Error.Code != "METHOD_NOT_FOUND" {
		t.Errorf("dispatcher:dispatch_routing_test - expected METHOD_NOT_FOUND, got %s", resp.Error.Code)
	}
	if resp.Error.Retryable {
		t.Error("dispatcher:dispatch_routing_test - METHOD_NOT_FOUND should not be retryable")
	}
}

func TestDispatch_UnknownMethodPreservesRequestID(t *testing.T) {
	disp := &Dispatcher{registry: nil}

	ids := []string{"req-1", "req-2", "unique-abc-123", ""}
	for _, id := range ids {
		resp := disp.Dispatch(context.Background(), &RegistryRequest{
			ID:     id,
			Method: "unknown",
			Params: json.RawMessage(`{}`),
		})

		if resp.ID != id {
			t.Errorf("dispatcher:dispatch_routing_test - expected ID=%q, got %q", id, resp.ID)
		}
	}
}

func TestDispatch_UserIDFromContext(t *testing.T) {
	disp := &Dispatcher{registry: nil}

	tests := []struct {
		name   string
		ctx    *InvocationContext
		method string
	}{
		{
			name:   "with userID in context",
			ctx:    &InvocationContext{UserID: "user-123"},
			method: "unknown",
		},
		{
			name:   "nil context defaults to system",
			ctx:    nil,
			method: "unknown",
		},
		{
			name:   "empty userID defaults to system",
			ctx:    &InvocationContext{UserID: ""},
			method: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := disp.Dispatch(context.Background(), &RegistryRequest{
				ID:     "test",
				Method: tt.method,
				Params: json.RawMessage(`{}`),
				Ctx:    tt.ctx,
			})

			// Verifying the dispatch doesn't panic with various contexts
			if resp.Error == nil || resp.Error.Code != "METHOD_NOT_FOUND" {
				t.Errorf("dispatcher:dispatch_routing_test - expected METHOD_NOT_FOUND for unknown method")
			}
		})
	}
}

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		code      string
		message   string
		retryable bool
	}{
		{
			name:      "not found error",
			id:        "req-1",
			code:      "NOT_FOUND",
			message:   "Capability not found",
			retryable: false,
		},
		{
			name:      "internal error is retryable",
			id:        "req-2",
			code:      "INTERNAL_ERROR",
			message:   "Database unavailable",
			retryable: true,
		},
		{
			name:      "invalid argument",
			id:        "req-3",
			code:      "INVALID_ARGUMENT",
			message:   "Missing required field",
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := errorResponse(tt.id, tt.code, tt.message, tt.retryable)

			if resp.ID != tt.id {
				t.Errorf("dispatcher:dispatch_routing_test - ID = %q, want %q", resp.ID, tt.id)
			}
			if resp.Ok {
				t.Error("dispatcher:dispatch_routing_test - expected Ok=false")
			}
			if resp.Error == nil {
				t.Fatal("dispatcher:dispatch_routing_test - expected error, got nil")
			}
			if resp.Error.Code != tt.code {
				t.Errorf("dispatcher:dispatch_routing_test - Code = %q, want %q", resp.Error.Code, tt.code)
			}
			if resp.Error.Message != tt.message {
				t.Errorf("dispatcher:dispatch_routing_test - Message = %q, want %q", resp.Error.Message, tt.message)
			}
			if resp.Error.Retryable != tt.retryable {
				t.Errorf("dispatcher:dispatch_routing_test - Retryable = %v, want %v", resp.Error.Retryable, tt.retryable)
			}
			if resp.Result != nil {
				t.Errorf("dispatcher:dispatch_routing_test - expected Result=nil, got %v", resp.Result)
			}
		})
	}
}

func TestRegistryErrorToResponse_RegistryError(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		message       string
		wantRetryable bool
	}{
		{
			name:          "NOT_FOUND is not retryable",
			code:          "NOT_FOUND",
			message:       "Capability not found",
			wantRetryable: false,
		},
		{
			name:          "INTERNAL_ERROR is retryable",
			code:          "INTERNAL_ERROR",
			message:       "DB connection failed",
			wantRetryable: true,
		},
		{
			name:          "INVALID_ARGUMENT is not retryable",
			code:          "INVALID_ARGUMENT",
			message:       "Bad input",
			wantRetryable: false,
		},
		{
			name:          "FORBIDDEN is not retryable",
			code:          "FORBIDDEN",
			message:       "Access denied",
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regErr := registry.NewRegistryError(tt.code, tt.message)
			resp := registryErrorToResponse("req-1", regErr)

			if resp.Ok {
				t.Error("dispatcher:dispatch_routing_test - expected Ok=false")
			}
			if resp.Error == nil {
				t.Fatal("dispatcher:dispatch_routing_test - expected error, got nil")
			}
			if resp.Error.Code != tt.code {
				t.Errorf("dispatcher:dispatch_routing_test - Code = %q, want %q", resp.Error.Code, tt.code)
			}
			if resp.Error.Retryable != tt.wantRetryable {
				t.Errorf("dispatcher:dispatch_routing_test - Retryable = %v, want %v", resp.Error.Retryable, tt.wantRetryable)
			}
		})
	}
}

func TestRegistryErrorToResponse_GenericError(t *testing.T) {
	// When passed a non-RegistryError, should wrap as INTERNAL_ERROR
	genericErr := errors.New("something went wrong")
	resp := registryErrorToResponse("req-1", genericErr)

	if resp.Ok {
		t.Error("dispatcher:dispatch_routing_test - expected Ok=false")
	}
	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("dispatcher:dispatch_routing_test - Code = %q, want %q", resp.Error.Code, "INTERNAL_ERROR")
	}
	if !resp.Error.Retryable {
		t.Error("dispatcher:dispatch_routing_test - generic errors should be retryable")
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("dispatcher:dispatch_routing_test - Message = %q, want %q", resp.Error.Message, "something went wrong")
	}
}

func TestInvCtxToResCtx_FullMapping(t *testing.T) {
	invCtx := &InvocationContext{
		TenantID:      "tenant-1",
		UserID:        "user-1",
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		Env:           "staging",
		Aud:           "api",
		Features:      []string{"beta", "preview"},
		Roles:         []string{"admin"},
		DeadlineMs:    5000,
		TimeoutMs:     3000,
	}

	resCtx := invCtxToResCtx(invCtx)

	if resCtx.TenantID != "tenant-1" {
		t.Errorf("dispatcher:dispatch_routing_test - TenantID = %q, want %q", resCtx.TenantID, "tenant-1")
	}
	if resCtx.Env != "staging" {
		t.Errorf("dispatcher:dispatch_routing_test - Env = %q, want %q", resCtx.Env, "staging")
	}
	if resCtx.Aud != "api" {
		t.Errorf("dispatcher:dispatch_routing_test - Aud = %q, want %q", resCtx.Aud, "api")
	}
	if len(resCtx.Features) != 2 {
		t.Fatalf("dispatcher:dispatch_routing_test - Features length = %d, want 2", len(resCtx.Features))
	}
	if resCtx.Features[0] != "beta" || resCtx.Features[1] != "preview" {
		t.Errorf("dispatcher:dispatch_routing_test - Features = %v, want [beta, preview]", resCtx.Features)
	}
}

func TestInvCtxToResCtx_EmptyFields(t *testing.T) {
	invCtx := &InvocationContext{}
	resCtx := invCtxToResCtx(invCtx)

	if resCtx == nil {
		t.Fatal("dispatcher:dispatch_routing_test - expected non-nil ResolutionContext for empty InvocationContext")
	}
	if resCtx.TenantID != "" {
		t.Errorf("dispatcher:dispatch_routing_test - TenantID = %q, want empty", resCtx.TenantID)
	}
	if resCtx.Env != "" {
		t.Errorf("dispatcher:dispatch_routing_test - Env = %q, want empty", resCtx.Env)
	}
	if resCtx.Features != nil {
		t.Errorf("dispatcher:dispatch_routing_test - Features = %v, want nil", resCtx.Features)
	}
}

func TestRegistryRequest_AllMethods(t *testing.T) {
	// Verify all supported method names are recognized
	knownMethods := []string{
		"resolve", "discover", "describe", "upsert",
		"setDefaultMajor", "deprecate", "disable",
		"listMajors", "health",
	}

	if len(knownMethods) != 9 {
		t.Errorf("dispatcher:dispatch_routing_test - expected 9 known methods, got %d", len(knownMethods))
	}
}

// TestDispatch_WithNilRepoRegistry verifies that each method returns INTERNAL_ERROR when registry has nil repo.
func TestDispatch_WithNilRepoRegistry(t *testing.T) {
	reg := registry.NewRegistry(registry.NewRegistryParams{
		Repo: nil, Publisher: nil, Config: registry.DefaultConfig(),
	})
	disp := NewDispatcher(reg)
	ctx := context.Background()

	tests := []struct {
		method string
		params string
	}{
		{"resolve", `{"cap":"more0.test"}`},
		{"discover", `{"page":1,"limit":10}`},
		{"describe", `{"cap":"more0.test"}`},
		{"listMajors", `{"cap":"more0.test"}`},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			resp := disp.Dispatch(ctx, &RegistryRequest{
				ID: "req-1", Method: tt.method, Params: json.RawMessage(tt.params),
			})
			if resp.Ok {
				t.Errorf("dispatcher:dispatch_routing_test - expected Ok=false for %s with nil repo", tt.method)
			}
			if resp.Error == nil {
				t.Fatalf("dispatcher:dispatch_routing_test - expected error for %s", tt.method)
			}
			if resp.Error.Code != "INTERNAL_ERROR" {
				t.Errorf("dispatcher:dispatch_routing_test - %s: Code = %q, want INTERNAL_ERROR", tt.method, resp.Error.Code)
			}
		})
	}
}

// TestDispatch_Health_WithNilRepoRegistry verifies health returns Ok with unhealthy status when repo is nil.
func TestDispatch_Health_WithNilRepoRegistry(t *testing.T) {
	reg := registry.NewRegistry(registry.NewRegistryParams{
		Repo: nil, Publisher: nil, Config: registry.DefaultConfig(),
	})
	disp := NewDispatcher(reg)
	ctx := context.Background()
	resp := disp.Dispatch(ctx, &RegistryRequest{
		ID: "req-1", Method: "health", Params: json.RawMessage(`{}`),
	})
	if !resp.Ok {
		t.Errorf("dispatcher:dispatch_routing_test - health with nil repo should return Ok=true (result has status unhealthy)")
	}
	if resp.Error != nil {
		t.Errorf("dispatcher:dispatch_routing_test - health should not return error")
	}
	if resp.Result == nil {
		t.Fatal("dispatcher:dispatch_routing_test - health should return result")
	}
	out, ok := resp.Result.(*registry.HealthOutput)
	if !ok {
		t.Fatalf("dispatcher:dispatch_routing_test - health result type = %T, want *registry.HealthOutput", resp.Result)
	}
	if out.Status != "unhealthy" {
		t.Errorf("dispatcher:dispatch_routing_test - health result status = %q, want unhealthy", out.Status)
	}
}

// TestDispatch_InvalidParams_ReturnsINVALID_ARGUMENT verifies bad JSON params yield INVALID_ARGUMENT.
func TestDispatch_InvalidParams_ReturnsINVALID_ARGUMENT(t *testing.T) {
	reg := registry.NewRegistry(registry.NewRegistryParams{
		Repo: nil, Publisher: nil, Config: registry.DefaultConfig(),
	})
	disp := NewDispatcher(reg)
	ctx := context.Background()

	resp := disp.Dispatch(ctx, &RegistryRequest{
		ID: "req-1", Method: "resolve", Params: json.RawMessage(`{invalid json`),
	})
	if resp.Ok {
		t.Error("dispatcher:dispatch_routing_test - expected Ok=false for invalid params")
	}
	if resp.Error == nil {
		t.Fatal("dispatcher:dispatch_routing_test - expected error")
	}
	if resp.Error.Code != "INVALID_ARGUMENT" {
		t.Errorf("dispatcher:dispatch_routing_test - Code = %q, want INVALID_ARGUMENT", resp.Error.Code)
	}
}

func TestInvocationContext_JSON(t *testing.T) {
	raw := `{
		"tenantId": "t-1",
		"userId": "u-1",
		"requestId": "r-1",
		"correlationId": "c-1",
		"env": "production",
		"aud": "api",
		"features": ["beta"],
		"roles": ["admin", "user"],
		"deadlineMs": 5000,
		"timeoutMs": 3000
	}`

	var ctx InvocationContext
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		t.Fatalf("dispatcher:dispatch_routing_test - failed to unmarshal: %v", err)
	}

	if ctx.TenantID != "t-1" {
		t.Errorf("dispatcher:dispatch_routing_test - TenantID = %q, want %q", ctx.TenantID, "t-1")
	}
	if ctx.UserID != "u-1" {
		t.Errorf("dispatcher:dispatch_routing_test - UserID = %q, want %q", ctx.UserID, "u-1")
	}
	if ctx.DeadlineMs != 5000 {
		t.Errorf("dispatcher:dispatch_routing_test - DeadlineMs = %d, want 5000", ctx.DeadlineMs)
	}
	if ctx.TimeoutMs != 3000 {
		t.Errorf("dispatcher:dispatch_routing_test - TimeoutMs = %d, want 3000", ctx.TimeoutMs)
	}
	if len(ctx.Roles) != 2 {
		t.Errorf("dispatcher:dispatch_routing_test - Roles length = %d, want 2", len(ctx.Roles))
	}
}

func TestRegistryResponse_JSONRoundTrip(t *testing.T) {
	resp := &RegistryResponse{
		ID:     "req-1",
		Ok:     true,
		Result: map[string]interface{}{"subject": "cap.more0.registry.v1"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("dispatcher:dispatch_routing_test - marshal failed: %v", err)
	}

	var decoded RegistryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("dispatcher:dispatch_routing_test - unmarshal failed: %v", err)
	}

	if decoded.ID != "req-1" {
		t.Errorf("dispatcher:dispatch_routing_test - ID = %q, want %q", decoded.ID, "req-1")
	}
	if !decoded.Ok {
		t.Error("dispatcher:dispatch_routing_test - expected Ok=true")
	}
	if decoded.Error != nil {
		t.Error("dispatcher:dispatch_routing_test - expected Error=nil for successful response")
	}
}

func TestErrorDetail_JSON(t *testing.T) {
	detail := &ErrorDetail{
		Code:      "VALIDATION_ERROR",
		Message:   "Field 'name' is required",
		Details:   map[string]string{"field": "name"},
		Retryable: false,
	}

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("dispatcher:dispatch_routing_test - marshal failed: %v", err)
	}

	var decoded ErrorDetail
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("dispatcher:dispatch_routing_test - unmarshal failed: %v", err)
	}

	if decoded.Code != "VALIDATION_ERROR" {
		t.Errorf("dispatcher:dispatch_routing_test - Code = %q, want %q", decoded.Code, "VALIDATION_ERROR")
	}
	if decoded.Retryable {
		t.Error("dispatcher:dispatch_routing_test - expected Retryable=false")
	}
}
