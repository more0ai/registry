package dispatcher

import (
	"encoding/json"
	"testing"
)

func TestRegistryRequest_Unmarshal(t *testing.T) {
	raw := `{
		"id": "req-1",
		"type": "invoke",
		"cap": "more0.registry",
		"method": "resolve",
		"params": {"cap": "more0.doc.ingest@^3.0.0"},
		"ctx": {"tenantId": "tenant-1", "env": "production"}
	}`

	var req RegistryRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.ID != "req-1" {
		t.Errorf("expected id req-1, got %s", req.ID)
	}
	if req.Method != "resolve" {
		t.Errorf("expected method resolve, got %s", req.Method)
	}
	if req.Ctx == nil {
		t.Fatal("expected ctx, got nil")
	}
	if req.Ctx.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", req.Ctx.TenantID)
	}
}

func TestRegistryResponse_Marshal(t *testing.T) {
	resp := &RegistryResponse{
		ID: "req-1",
		Ok: true,
		Result: map[string]interface{}{
			"subject":         "cap.more0.doc_ingest.v3",
			"resolvedVersion": "3.4.2",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded["ok"] != true {
		t.Errorf("expected ok=true, got %v", decoded["ok"])
	}
	if decoded["id"] != "req-1" {
		t.Errorf("expected id=req-1, got %v", decoded["id"])
	}
}

func TestRegistryResponse_Error(t *testing.T) {
	resp := &RegistryResponse{
		ID: "req-2",
		Ok: false,
		Error: &ErrorDetail{
			Code:      "NOT_FOUND",
			Message:   "Capability not found",
			Retryable: false,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded RegistryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Ok {
		t.Error("expected ok=false")
	}
	if decoded.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if decoded.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", decoded.Error.Code)
	}
}

func TestInvCtxToResCtx(t *testing.T) {
	invCtx := &InvocationContext{
		TenantID: "tenant-1",
		Env:      "production",
		Aud:      "api",
		Features: []string{"beta"},
	}

	resCtx := invCtxToResCtx(invCtx)
	if resCtx == nil {
		t.Fatal("expected resCtx, got nil")
	}
	if resCtx.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", resCtx.TenantID)
	}
	if resCtx.Env != "production" {
		t.Errorf("expected production, got %s", resCtx.Env)
	}
	if resCtx.Aud != "api" {
		t.Errorf("expected api, got %s", resCtx.Aud)
	}
	if len(resCtx.Features) != 1 || resCtx.Features[0] != "beta" {
		t.Errorf("expected [beta], got %v", resCtx.Features)
	}

	// nil input
	if invCtxToResCtx(nil) != nil {
		t.Error("expected nil for nil input")
	}
}
