// Package bootstrap provides bootstrap configuration loading for system capabilities.
package bootstrap

// BootstrapMethodMetadata holds optional per-method metadata (description, schemas, modes, tags, examples).
// When present in bootstrap, this metadata is persisted so describe returns full method details.
type BootstrapMethodMetadata struct {
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"`
	Modes        []string               `json:"modes,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Examples     []interface{}          `json:"examples,omitempty"`
}

// BootstrapCapability is a system capability entry in the bootstrap config.
type BootstrapCapability struct {
	Subject          string                           `json:"subject"`
	NatsUrl          string                           `json:"natsUrl,omitempty"`
	Major            int                              `json:"major"`
	Version          string                           `json:"version"`
	Status           string                           `json:"status"`
	Description      string                           `json:"description,omitempty"`
	Methods          []string                         `json:"methods"`
	MethodsMetadata  map[string]BootstrapMethodMetadata `json:"methodsMetadata,omitempty"`
	IsSystem         bool                             `json:"isSystem"`
	TTLSeconds       int                              `json:"ttlSeconds"`
}

// BootstrapAliasEntry provides NATS URL information for a known registry alias.
type BootstrapAliasEntry struct {
	NatsUrl string `json:"natsUrl"`
}

// BootstrapConfig is the root bootstrap configuration.
// Name and MinimumCapabilities support versioning and cache invalidation (see Docs/registry/09 §2.2).
type BootstrapConfig struct {
	Name                 string                           `json:"name"`
	Version              string                           `json:"version"`
	Description          string                           `json:"description,omitempty"`
	MinimumCapabilities  []string                         `json:"minimum_capabilities,omitempty"`
	Capabilities         map[string]BootstrapCapability    `json:"capabilities"`
	Aliases              map[string]string                `json:"aliases"`
	RegistryAliases      map[string]BootstrapAliasEntry   `json:"registryAliases,omitempty"`
	DefaultAlias         string                           `json:"defaultAlias,omitempty"`
	ChangeEvents         ChangeEventSubjects              `json:"changeEventSubjects"`
}

// ChangeEventSubjects defines event subject patterns.
type ChangeEventSubjects struct {
	Global  string `json:"global"`
	Pattern string `json:"pattern"`
}

// ResolvedBootstrap provides fast lookup of bootstrap capabilities.
type ResolvedBootstrap struct {
	name         string
	version      string
	minCaps      []string
	capabilities map[string]*BootstrapCapability
	aliases      map[string]string
	changeEvents ChangeEventSubjects
}

// Get returns a bootstrap capability by reference (e.g., "system.registry").
func (rb *ResolvedBootstrap) Get(capRef string) *BootstrapCapability {
	// Try direct lookup first
	if cap, ok := rb.capabilities[capRef]; ok {
		return cap
	}
	// Try alias resolution
	if resolved, ok := rb.aliases[capRef]; ok {
		if cap, ok := rb.capabilities[resolved]; ok {
			return cap
		}
	}
	return nil
}

// IsSystem checks if a capability reference is a system capability.
func (rb *ResolvedBootstrap) IsSystem(capRef string) bool {
	cap := rb.Get(capRef)
	return cap != nil && cap.IsSystem
}

// GetSubject returns the COMMS subject for a bootstrap capability.
func (rb *ResolvedBootstrap) GetSubject(capRef string) string {
	cap := rb.Get(capRef)
	if cap != nil {
		return cap.Subject
	}
	return ""
}

// List returns all bootstrap capabilities.
func (rb *ResolvedBootstrap) List() map[string]*BootstrapCapability {
	return rb.capabilities
}

// ResolveAlias resolves an alias to the full capability reference.
func (rb *ResolvedBootstrap) ResolveAlias(alias string) string {
	if resolved, ok := rb.aliases[alias]; ok {
		return resolved
	}
	return alias
}

// GlobalChangeSubject returns the global change event subject.
func (rb *ResolvedBootstrap) GlobalChangeSubject() string {
	return rb.changeEvents.Global
}

// Name returns the bootstrap config name (for versioning/cache invalidation).
func (rb *ResolvedBootstrap) Name() string {
	return rb.name
}

// Version returns the bootstrap config version.
func (rb *ResolvedBootstrap) Version() string {
	return rb.version
}

// MinimumCapabilities returns the list of capability refs that must always be present.
func (rb *ResolvedBootstrap) MinimumCapabilities() []string {
	return rb.minCaps
}
