package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/morezero/capabilities-registry/pkg/registry"
)

const logPrefix = "dispatcher:dispatch"

// Dispatcher routes COMMS requests to registry methods.
type Dispatcher struct {
	registry *registry.Registry
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(reg *registry.Registry) *Dispatcher {
	return &Dispatcher{registry: reg}
}

// Dispatch routes a request to the appropriate registry method and returns a response.
func (d *Dispatcher) Dispatch(ctx context.Context, req *RegistryRequest) *RegistryResponse {
	slog.Debug(fmt.Sprintf("%s - method=%s id=%s", logPrefix, req.Method, req.ID))

	// Extract userID from context
	userID := "system"
	if req.Ctx != nil && req.Ctx.UserID != "" {
		userID = req.Ctx.UserID
	}

	switch req.Method {
	case "resolve":
		return d.handleResolve(ctx, req)
	case "discover":
		return d.handleDiscover(ctx, req)
	case "describe":
		return d.handleDescribe(ctx, req)
	case "upsert":
		return d.handleUpsert(ctx, req, userID)
	case "setDefaultMajor":
		return d.handleSetDefaultMajor(ctx, req, userID)
	case "deprecate":
		return d.handleDeprecate(ctx, req, userID)
	case "disable":
		return d.handleDisable(ctx, req, userID)
	case "listMajors":
		return d.handleListMajors(ctx, req)
	case "health":
		return d.handleHealth(ctx, req)
	default:
		return &RegistryResponse{
			ID: req.ID,
			Ok: false,
			Error: &ErrorDetail{
				Code:      "METHOD_NOT_FOUND",
				Message:   fmt.Sprintf("Unknown method: %s", req.Method),
				Retryable: false,
			},
		}
	}
}

func (d *Dispatcher) handleResolve(ctx context.Context, req *RegistryRequest) *RegistryResponse {
	var input registry.ResolveInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse resolve params", false)
	}
	// Inject context
	if input.Ctx == nil && req.Ctx != nil {
		input.Ctx = invCtxToResCtx(req.Ctx)
	}

	result, err := d.registry.Resolve(ctx, &input)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleDiscover(ctx context.Context, req *RegistryRequest) *RegistryResponse {
	var input registry.DiscoverInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse discover params", false)
	}
	if input.Ctx == nil && req.Ctx != nil {
		input.Ctx = invCtxToResCtx(req.Ctx)
	}

	result, err := d.registry.Discover(ctx, &input)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleDescribe(ctx context.Context, req *RegistryRequest) *RegistryResponse {
	var input registry.DescribeInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse describe params", false)
	}

	result, err := d.registry.Describe(ctx, &input)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleUpsert(ctx context.Context, req *RegistryRequest, userID string) *RegistryResponse {
	var input registry.UpsertInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse upsert params", false)
	}

	result, err := d.registry.Upsert(ctx, &input, userID)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleSetDefaultMajor(ctx context.Context, req *RegistryRequest, userID string) *RegistryResponse {
	var input registry.SetDefaultMajorInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse setDefaultMajor params", false)
	}

	result, err := d.registry.SetDefaultMajor(ctx, &input, userID)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleDeprecate(ctx context.Context, req *RegistryRequest, userID string) *RegistryResponse {
	var input registry.DeprecateInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse deprecate params", false)
	}

	result, err := d.registry.Deprecate(ctx, &input, userID)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleDisable(ctx context.Context, req *RegistryRequest, userID string) *RegistryResponse {
	var input registry.DisableInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse disable params", false)
	}

	result, err := d.registry.Disable(ctx, &input, userID)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleListMajors(ctx context.Context, req *RegistryRequest) *RegistryResponse {
	var input registry.ListMajorsInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return errorResponse(req.ID, "INVALID_ARGUMENT", "Failed to parse listMajors params", false)
	}

	result, err := d.registry.ListMajors(ctx, &input)
	if err != nil {
		return registryErrorToResponse(req.ID, err)
	}
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

func (d *Dispatcher) handleHealth(ctx context.Context, req *RegistryRequest) *RegistryResponse {
	result := d.registry.Health(ctx)
	return &RegistryResponse{ID: req.ID, Ok: true, Result: result}
}

// --- helpers ---

func errorResponse(id, code, message string, retryable bool) *RegistryResponse {
	return &RegistryResponse{
		ID: id,
		Ok: false,
		Error: &ErrorDetail{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	}
}

func registryErrorToResponse(id string, err error) *RegistryResponse {
	if regErr, ok := err.(*registry.RegistryError); ok {
		retryable := regErr.Code == "INTERNAL_ERROR"
		return &RegistryResponse{
			ID: id,
			Ok: false,
			Error: &ErrorDetail{
				Code:      regErr.Code,
				Message:   regErr.Message,
				Details:   regErr.Details,
				Retryable: retryable,
			},
		}
	}
	return errorResponse(id, "INTERNAL_ERROR", err.Error(), true)
}

func invCtxToResCtx(invCtx *InvocationContext) *registry.ResolutionContext {
	if invCtx == nil {
		return nil
	}
	return &registry.ResolutionContext{
		TenantID: invCtx.TenantID,
		Env:      invCtx.Env,
		Aud:      invCtx.Aud,
		Features: invCtx.Features,
	}
}
