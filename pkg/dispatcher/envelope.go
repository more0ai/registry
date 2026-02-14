// Package dispatcher routes incoming COMMS messages to registry methods.
package dispatcher

import "encoding/json"

// RegistryRequest is the JSON envelope for incoming COMMS registry requests.
type RegistryRequest struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Cap    string                 `json:"cap"`
	Method string                 `json:"method"`
	Params json.RawMessage        `json:"params"`
	Ctx    *InvocationContext     `json:"ctx,omitempty"`
}

// RegistryResponse is the JSON envelope for COMMS registry responses.
type RegistryResponse struct {
	ID     string       `json:"id"`
	Ok     bool         `json:"ok"`
	Result interface{}  `json:"result,omitempty"`
	Error  *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail holds structured error information.
type ErrorDetail struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	Retryable bool        `json:"retryable"`
}

// InvocationContext holds context from the caller.
type InvocationContext struct {
	TenantID      string   `json:"tenantId,omitempty"`
	UserID        string   `json:"userId,omitempty"`
	RequestID     string   `json:"requestId,omitempty"`
	CorrelationID string   `json:"correlationId,omitempty"`
	Env           string   `json:"env,omitempty"`
	Aud           string   `json:"aud,omitempty"`
	Features      []string `json:"features,omitempty"`
	Roles         []string `json:"roles,omitempty"`
	DeadlineMs    int      `json:"deadlineMs,omitempty"`
	TimeoutMs     int      `json:"timeoutMs,omitempty"`
}
