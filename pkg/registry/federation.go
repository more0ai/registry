package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	comms "github.com/nats-io/nats.go"

	"github.com/morezero/capabilities-registry/pkg/db"
)

const federationLogPrefix = "registry:federation"

// FederationPool manages server-to-server NATS connections to remote registries.
// Keyed by alias (which maps to natsUrl). Connections are persistent.
type FederationPool struct {
	mu          sync.RWMutex
	connections map[string]*federatedConnection
	repo        *db.Repository
}

type federatedConnection struct {
	nc          *comms.Conn
	alias       string
	natsUrl     string
	registrySub string
	connectedAt time.Time
}

// NewFederationPool creates a new federation pool.
func NewFederationPool(repo *db.Repository) *FederationPool {
	return &FederationPool{
		connections: make(map[string]*federatedConnection),
		repo:        repo,
	}
}

// FederatedResolveInput holds parameters for a federated resolve call.
type FederatedResolveInput struct {
	Alias string
	Cap   string
	Ver   string
	Ctx   *ResolutionContext
}

// FederatedResolveOutput holds the result of a federated resolve call.
type FederatedResolveOutput struct {
	NatsUrl           string
	Subject           string
	CanonicalIdentity string
	ResolvedVersion   string
	Major             int
	Status            string
	TTLSeconds        int
	Etag              string
}

// Resolve performs a federated resolve call to a remote registry via NATS.
func (fp *FederationPool) Resolve(ctx context.Context, input *FederatedResolveInput) (*FederatedResolveOutput, error) {
	slog.Info(fmt.Sprintf("%s - Resolving alias=%s cap=%s", federationLogPrefix, input.Alias, input.Cap))

	// Look up alias in registries table
	entry, err := fp.repo.GetRegistryByAlias(ctx, input.Alias)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("Failed to look up alias %s: %v", input.Alias, err)}
	}
	if entry == nil {
		return nil, &RegistryError{Code: "UNKNOWN_ALIAS", Message: fmt.Sprintf("Unknown registry alias: %s", input.Alias)}
	}
	if entry.NatsUrl == nil || *entry.NatsUrl == "" {
		return nil, &RegistryError{Code: "REGISTRY_UNAVAILABLE", Message: fmt.Sprintf("Registry alias %s has no NATS URL configured", input.Alias)}
	}
	if entry.RegistrySubject == nil || *entry.RegistrySubject == "" {
		return nil, &RegistryError{Code: "REGISTRY_UNAVAILABLE", Message: fmt.Sprintf("Registry alias %s has no registry subject configured", input.Alias)}
	}

	// Get or create connection
	nc, err := fp.getOrConnect(input.Alias, *entry.NatsUrl)
	if err != nil {
		return nil, &RegistryError{Code: "REGISTRY_UNAVAILABLE", Message: fmt.Sprintf("Failed to connect to remote registry %s: %v", input.Alias, err)}
	}

	// Build remote resolve request
	remoteReq := map[string]interface{}{
		"id":     fmt.Sprintf("fed-%d", time.Now().UnixNano()),
		"type":   "invoke",
		"cap":    "system.registry",
		"method": "resolve",
		"params": map[string]interface{}{
			"cap": input.Cap,
			"ver": input.Ver,
		},
	}
	if input.Ctx != nil {
		remoteReq["params"].(map[string]interface{})["ctx"] = input.Ctx
	}

	payload, err := json.Marshal(remoteReq)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("Failed to marshal federated request: %v", err)}
	}

	// Send request to remote registry
	msg, err := nc.RequestWithContext(ctx, *entry.RegistrySubject, payload)
	if err != nil {
		return nil, &RegistryError{Code: "REGISTRY_UNAVAILABLE", Message: fmt.Sprintf("Remote registry %s did not respond: %v", input.Alias, err)}
	}

	// Decode response
	var resp struct {
		Ok     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("Failed to decode remote response from %s: %v", input.Alias, err)}
	}
	if !resp.Ok {
		code := "INTERNAL_ERROR"
		message := "Remote resolve failed"
		if resp.Error != nil {
			code = resp.Error.Code
			message = resp.Error.Message
		}
		return nil, &RegistryError{Code: code, Message: fmt.Sprintf("Remote registry %s: %s", input.Alias, message)}
	}

	// Decode the resolve result
	var remoteResult struct {
		Subject         string `json:"subject"`
		ResolvedVersion string `json:"resolvedVersion"`
		Major           int    `json:"major"`
		Status          string `json:"status"`
		TTLSeconds      int    `json:"ttlSeconds"`
		Etag            string `json:"etag"`
	}
	if err := json.Unmarshal(resp.Result, &remoteResult); err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("Failed to decode remote resolve result from %s: %v", input.Alias, err)}
	}

	return &FederatedResolveOutput{
		NatsUrl:           *entry.NatsUrl,
		Subject:           remoteResult.Subject,
		CanonicalIdentity: fmt.Sprintf("cap:@%s/%s@%s", input.Alias, input.Cap, remoteResult.ResolvedVersion),
		ResolvedVersion:   remoteResult.ResolvedVersion,
		Major:             remoteResult.Major,
		Status:            remoteResult.Status,
		TTLSeconds:        remoteResult.TTLSeconds,
		Etag:              remoteResult.Etag,
	}, nil
}

// getOrConnect gets an existing connection or creates a new one.
func (fp *FederationPool) getOrConnect(alias, natsUrl string) (*comms.Conn, error) {
	fp.mu.RLock()
	if fc, ok := fp.connections[alias]; ok && fc.nc.IsConnected() {
		fp.mu.RUnlock()
		return fc.nc, nil
	}
	fp.mu.RUnlock()

	fp.mu.Lock()
	defer fp.mu.Unlock()

	// Double-check after acquiring write lock
	if fc, ok := fp.connections[alias]; ok && fc.nc.IsConnected() {
		return fc.nc, nil
	}

	// Remove stale entry if exists
	if fc, ok := fp.connections[alias]; ok {
		fc.nc.Close()
		delete(fp.connections, alias)
	}

	slog.Info(fmt.Sprintf("%s - Connecting to remote NATS alias=%s url=%s", federationLogPrefix, alias, natsUrl))
	nc, err := comms.Connect(natsUrl,
		comms.Name(fmt.Sprintf("capabilities-registry-federation-%s", alias)),
		comms.MaxReconnects(5),
		comms.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, err
	}

	fp.connections[alias] = &federatedConnection{
		nc:          nc,
		alias:       alias,
		natsUrl:     natsUrl,
		connectedAt: time.Now(),
	}

	return nc, nil
}

// CloseAll closes all federated connections.
func (fp *FederationPool) CloseAll() {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	for alias, fc := range fp.connections {
		slog.Info(fmt.Sprintf("%s - Closing federated connection alias=%s", federationLogPrefix, alias))
		fc.nc.Close()
	}
	fp.connections = make(map[string]*federatedConnection)
}

// LoadRegistryAliases loads all registry aliases from the database.
// Returns a map of alias → natsUrl for inclusion in bootstrap response.
func (fp *FederationPool) LoadRegistryAliases(ctx context.Context) (map[string]string, string, error) {
	entries, err := fp.repo.ListRegistries(ctx)
	if err != nil {
		return nil, "", err
	}

	aliases := make(map[string]string)
	defaultAlias := "main"
	for _, e := range entries {
		if e.NatsUrl != nil && *e.NatsUrl != "" {
			aliases[e.Alias] = *e.NatsUrl
		}
		if e.IsDefault {
			defaultAlias = e.Alias
		}
	}
	return aliases, defaultAlias, nil
}
