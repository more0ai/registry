// Package registry implements the core registry business logic.
package registry

// ResolveInput holds parameters for the resolve method.
type ResolveInput struct {
	Cap            string             `json:"cap"`
	Ver            string             `json:"ver,omitempty"`
	Ctx            *ResolutionContext `json:"ctx,omitempty"`
	IncludeMethods bool               `json:"includeMethods,omitempty"`
	IncludeSchemas bool               `json:"includeSchemas,omitempty"`
}

// ResolveOutput holds the result of the resolve method.
type ResolveOutput struct {
	CanonicalIdentity string            `json:"canonicalIdentity"`
	NatsUrl           string            `json:"natsUrl"`
	Subject           string            `json:"subject"`
	Major             int               `json:"major"`
	ResolvedVersion   string            `json:"resolvedVersion"`
	Status            string            `json:"status"`
	TTLSeconds        int               `json:"ttlSeconds"`
	Etag              string            `json:"etag"`
	ExpiresAt         string            `json:"expiresAt,omitempty"`
	Methods           []MethodInfo      `json:"methods,omitempty"`
	Schemas           map[string]Schema `json:"schemas,omitempty"`
}

// MethodInfo holds basic method information.
type MethodInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Modes       []string `json:"modes"`
	Tags        []string `json:"tags"`
}

// Schema holds input/output schemas for a method.
type Schema struct {
	Input  map[string]interface{} `json:"input"`
	Output map[string]interface{} `json:"output"`
}

// DiscoverInput holds parameters for the discover method.
type DiscoverInput struct {
	App            string             `json:"app,omitempty"`
	Tags           []string           `json:"tags,omitempty"`
	Query          string             `json:"query,omitempty"`
	Status         string             `json:"status,omitempty"`
	SupportsMethod string             `json:"supportsMethod,omitempty"`
	Ctx            *ResolutionContext `json:"ctx,omitempty"`
	Page           int                `json:"page,omitempty"`
	Limit          int                `json:"limit,omitempty"`
}

// DiscoverOutput holds the result of the discover method.
type DiscoverOutput struct {
	Capabilities []DiscoveredCapability `json:"capabilities"`
	Pagination   Pagination            `json:"pagination"`
}

// DiscoveredCapability holds discovery result for a single capability.
type DiscoveredCapability struct {
	Cap           string   `json:"cap"`
	App           string   `json:"app"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Tags          []string `json:"tags"`
	DefaultMajor  int      `json:"defaultMajor"`
	LatestVersion string   `json:"latestVersion"`
	Majors        []int    `json:"majors"`
	Status        string   `json:"status"`
}

// Pagination holds pagination information.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// DescribeInput holds parameters for the describe method.
type DescribeInput struct {
	Cap     string `json:"cap"`
	Major   *int   `json:"major,omitempty"`
	Version string `json:"version,omitempty"`
}

// DescribeOutput holds the result of the describe method.
type DescribeOutput struct {
	Cap         string              `json:"cap"`
	App         string              `json:"app"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Version     string              `json:"version"`
	Major       int                 `json:"major"`
	Status      string              `json:"status"`
	Methods     []MethodDescription `json:"methods"`
	Tags        []string            `json:"tags"`
	Changelog   string              `json:"changelog,omitempty"`
}

// MethodDescription holds detailed method information.
type MethodDescription struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema"`
	OutputSchema map[string]interface{} `json:"outputSchema"`
	Modes        []string               `json:"modes"`
	Tags         []string               `json:"tags"`
	Examples     []interface{}          `json:"examples"`
}

// UpsertInput holds parameters for the upsert method.
type UpsertInput struct {
	App         string             `json:"app"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Version     VersionInput       `json:"version"`
	Methods     []MethodDefinition `json:"methods"`
	SetAsDefault bool              `json:"setAsDefault,omitempty"`
	Env          string            `json:"env,omitempty"`
}

// VersionInput holds version parameters for upsert.
type VersionInput struct {
	Major       int                    `json:"major"`
	Minor       int                    `json:"minor"`
	Patch       int                    `json:"patch"`
	Prerelease  string                 `json:"prerelease,omitempty"`
	Description string                 `json:"description,omitempty"`
	Changelog   string                 `json:"changelog,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// MethodDefinition holds method parameters for upsert.
type MethodDefinition struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"`
	Modes        []string               `json:"modes,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Policies     map[string]interface{} `json:"policies,omitempty"`
	Examples     []interface{}          `json:"examples,omitempty"`
}

// UpsertOutput holds the result of the upsert method.
type UpsertOutput struct {
	Action       string `json:"action"`
	CapabilityID string `json:"capabilityId"`
	VersionID    string `json:"versionId"`
	Cap          string `json:"cap"`
	Version      string `json:"version"`
	Subject      string `json:"subject"`
}

// SetDefaultMajorInput holds parameters for the setDefaultMajor method.
type SetDefaultMajorInput struct {
	Cap   string `json:"cap"`
	Major int    `json:"major"`
	Env   string `json:"env,omitempty"`
}

// SetDefaultMajorOutput holds the result of the setDefaultMajor method.
type SetDefaultMajorOutput struct {
	Success       bool `json:"success"`
	PreviousMajor *int `json:"previousMajor,omitempty"`
	NewMajor      int  `json:"newMajor"`
}

// DeprecateInput holds parameters for the deprecate method.
type DeprecateInput struct {
	Cap     string `json:"cap"`
	Version string `json:"version,omitempty"`
	Major   *int   `json:"major,omitempty"`
	Reason  string `json:"reason"`
}

// DeprecateOutput holds the result of the deprecate method.
type DeprecateOutput struct {
	Success          bool     `json:"success"`
	AffectedVersions []string `json:"affectedVersions"`
}

// DisableInput holds parameters for the disable method.
type DisableInput struct {
	Cap     string `json:"cap"`
	Version string `json:"version,omitempty"`
	Major   *int   `json:"major,omitempty"`
	Reason  string `json:"reason"`
}

// DisableOutput holds the result of the disable method.
type DisableOutput struct {
	Success          bool     `json:"success"`
	AffectedVersions []string `json:"affectedVersions"`
}

// ListMajorsInput holds parameters for the listMajors method.
type ListMajorsInput struct {
	Cap             string `json:"cap"`
	IncludeInactive bool   `json:"includeInactive,omitempty"`
}

// ListMajorsOutput holds the result of the listMajors method.
type ListMajorsOutput struct {
	Majors []MajorInfo `json:"majors"`
}

// MajorInfo holds information about a major version.
type MajorInfo struct {
	Major         int    `json:"major"`
	LatestVersion string `json:"latestVersion"`
	Status        string `json:"status"`
	VersionCount  int    `json:"versionCount"`
	IsDefault     bool   `json:"isDefault"`
}

// HealthOutput holds the result of the health method.
type HealthOutput struct {
	Status    string       `json:"status"`
	Checks    HealthChecks `json:"checks"`
	Timestamp string       `json:"timestamp"`
}

// HealthChecks holds individual health check results.
type HealthChecks struct {
	Database bool `json:"database"`
	COMMS    bool `json:"comms,omitempty"`
}

// ResolutionContext provides multi-tenant context for resolution.
type ResolutionContext struct {
	TenantID string   `json:"tenantId,omitempty"`
	Env      string   `json:"env,omitempty"`
	Aud      string   `json:"aud,omitempty"`
	Features []string `json:"features,omitempty"`
}

// RegistryError is a structured error from the registry.
type RegistryError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func (e *RegistryError) Error() string {
	return e.Code + ": " + e.Message
}

// NewRegistryError creates a new RegistryError.
func NewRegistryError(code, message string) *RegistryError {
	return &RegistryError{Code: code, Message: message}
}
