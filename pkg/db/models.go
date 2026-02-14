package db

import "time"

// Capability represents a row in the capabilities table.
type Capability struct {
	ID          string    `json:"id"`
	App         string    `json:"app"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Tags        []string  `json:"tags"`
	Status      string    `json:"status"`
	Object      string    `json:"object"`
	Revision    int       `json:"revision"`
	Created     time.Time `json:"created"`
	CreatedBy   string    `json:"created_by"`
	Modified    time.Time `json:"modified"`
	ModifiedBy  string    `json:"modified_by"`
	Config      []byte    `json:"config,omitempty"`
	Ext         []byte    `json:"ext,omitempty"`
}

// CapabilityVersion represents a row in the capability_versions table.
type CapabilityVersion struct {
	ID                string     `json:"id"`
	CapabilityID      string     `json:"capability_id"`
	Major             int        `json:"major"`
	Minor             int        `json:"minor"`
	Patch             int        `json:"patch"`
	Prerelease        *string    `json:"prerelease,omitempty"`
	BuildMetadata     *string    `json:"build_metadata,omitempty"`
	VersionString     *string    `json:"version_string,omitempty"`
	Status            string     `json:"status"`
	DeprecationReason *string    `json:"deprecation_reason,omitempty"`
	DeprecatedAt      *time.Time `json:"deprecated_at,omitempty"`
	DisabledAt        *time.Time `json:"disabled_at,omitempty"`
	Description       *string    `json:"description,omitempty"`
	Changelog         *string    `json:"changelog,omitempty"`
	Metadata          []byte     `json:"metadata,omitempty"`
	Object            string     `json:"object"`
	Created           time.Time  `json:"created"`
	CreatedBy         string     `json:"created_by"`
	Modified          time.Time  `json:"modified"`
	ModifiedBy        string     `json:"modified_by"`
	Config            []byte     `json:"config,omitempty"`
	Ext               []byte     `json:"ext,omitempty"`
}

// CapabilityMethod represents a row in the capability_methods table.
type CapabilityMethod struct {
	ID           string    `json:"id"`
	VersionID    string    `json:"version_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	InputSchema  []byte    `json:"input_schema,omitempty"`
	OutputSchema []byte    `json:"output_schema,omitempty"`
	Tags         []string  `json:"tags"`
	Policies     []byte    `json:"policies,omitempty"`
	Examples     []byte    `json:"examples,omitempty"`
	Modes        []string  `json:"modes"`
	Object       string    `json:"object"`
	Created      time.Time `json:"created"`
	CreatedBy    string    `json:"created_by"`
	Modified     time.Time `json:"modified"`
	ModifiedBy   string    `json:"modified_by"`
	Config       []byte    `json:"config,omitempty"`
	Ext          []byte    `json:"ext,omitempty"`
}

// CapabilityDefault represents a row in the capability_defaults table.
type CapabilityDefault struct {
	ID           string    `json:"id"`
	CapabilityID string    `json:"capability_id"`
	DefaultMajor int       `json:"default_major"`
	Env          string    `json:"env"`
	Object       string    `json:"object"`
	Created      time.Time `json:"created"`
	CreatedBy    string    `json:"created_by"`
	Modified     time.Time `json:"modified"`
	ModifiedBy   string    `json:"modified_by"`
	Config       []byte    `json:"config,omitempty"`
	Ext          []byte    `json:"ext,omitempty"`
}

// CapabilityTenantRule represents a row in the capability_tenant_rules table.
type CapabilityTenantRule struct {
	ID               string   `json:"id"`
	CapabilityID     string   `json:"capability_id"`
	TenantID         *string  `json:"tenant_id,omitempty"`
	Env              *string  `json:"env,omitempty"`
	Aud              *string  `json:"aud,omitempty"`
	RuleType         string   `json:"rule_type"`
	AllowedMajors    []int    `json:"allowed_majors"`
	DeniedMajors     []int    `json:"denied_majors"`
	RequiredFeatures []string `json:"required_features"`
	Priority         int      `json:"priority"`
	Object           string   `json:"object"`
	Status           string   `json:"status"`
	CreatedBy        string   `json:"created_by"`
	ModifiedBy       string   `json:"modified_by"`
}

// RegistryEntry represents a row in the registries table.
// Used for alias-based resolution and federation.
type RegistryEntry struct {
	ID              string    `json:"id"`
	Alias           string    `json:"alias"`
	NatsUrl         *string   `json:"nats_url,omitempty"`
	RegistrySubject *string   `json:"registry_subject,omitempty"`
	IsDefault       bool      `json:"is_default"`
	Config          []byte    `json:"config,omitempty"`
	Created         time.Time `json:"created"`
	Modified        time.Time `json:"modified"`
}

// ResolutionContext provides multi-tenant context for resolution.
type ResolutionContext struct {
	TenantID string
	Env      string
	Aud      string
	Features []string
}
