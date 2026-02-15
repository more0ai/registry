package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/events"
)

const (
	defaultTTLSeconds    = 300
	defaultEnv           = "production"
	defaultSubjectPrefix = "cap"
	defaultAlias         = "main"
)

// Config holds registry configuration.
type Config struct {
	DefaultTTLSeconds int
	DefaultEnv        string
	SubjectPrefix     string
	DefaultAlias string
	// NatsUrl is the NATS server URL for the local/default registry.
	// Included in resolve responses so clients know which NATS to connect to.
	NatsUrl string
}

// DefaultConfig returns the default registry configuration.
func DefaultConfig() Config {
	return Config{
		DefaultTTLSeconds: defaultTTLSeconds,
		DefaultEnv:        defaultEnv,
		SubjectPrefix:     defaultSubjectPrefix,
		DefaultAlias:      defaultAlias,
	}
}

// Registry is the main registry service containing all business logic methods.
type Registry struct {
	repo           *db.Repository
	publisher      events.EventPublisher
	config         Config
	federationPool *FederationPool
}

// NewRegistry creates a new Registry instance.
func NewRegistry(params NewRegistryParams) *Registry {
	cfg := params.Config
	if cfg.DefaultTTLSeconds == 0 {
		cfg.DefaultTTLSeconds = defaultTTLSeconds
	}
	if cfg.DefaultEnv == "" {
		cfg.DefaultEnv = defaultEnv
	}
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = defaultSubjectPrefix
	}
	if cfg.DefaultAlias == "" {
		cfg.DefaultAlias = defaultAlias
	}

	pub := params.Publisher
	if pub == nil {
		pub = &events.NoOpPublisher{}
	}

	var fedPool *FederationPool
	if params.Repo != nil {
		fedPool = NewFederationPool(params.Repo)
	}

	return &Registry{
		repo:           params.Repo,
		publisher:      pub,
		config:         cfg,
		federationPool: fedPool,
	}
}

// NewRegistryParams holds parameters for NewRegistry.
type NewRegistryParams struct {
	Repo      *db.Repository
	Publisher events.EventPublisher
	Config    Config
}

// buildSubject builds a COMMS subject from components.
func (r *Registry) buildSubject(app, name string, major int) string {
	// Normalize: replace dots in name with underscores for subject
	safeName := strings.ReplaceAll(name, ".", "_")
	return fmt.Sprintf("%s.%s.%s.v%d", r.config.SubjectPrefix, app, safeName, major)
}

// getEnv returns the environment from context or default.
func (r *Registry) getEnv(ctx *ResolutionContext) string {
	if ctx != nil && ctx.Env != "" {
		return ctx.Env
	}
	return r.config.DefaultEnv
}

// requireRepo returns an error if the repository is not configured (e.g. in tests with nil repo).
func (r *Registry) requireRepo() *RegistryError {
	if r.repo == nil {
		return &RegistryError{Code: "INTERNAL_ERROR", Message: "repository not configured"}
	}
	return nil
}

// Close cleans up resources (e.g., federated connections).
func (r *Registry) Close() {
	if r.federationPool != nil {
		r.federationPool.CloseAll()
	}
}

// LoadRegistryAliases loads all registry aliases from the database.
// Returns a map of alias → natsUrl and the default alias name.
func (r *Registry) LoadRegistryAliases(ctx context.Context) (map[string]string, string, error) {
	if r.federationPool == nil {
		return nil, r.config.DefaultAlias, nil
	}
	return r.federationPool.LoadRegistryAliases(ctx)
}

// GetBootstrapCapabilities returns capabilities from the database in the same shape as resolve:
// ResolveOutput per capability (canonicalIdentity, natsUrl, subject, major, resolvedVersion, status, ttlSeconds=0, etag, methods, optional schemas).
func (r *Registry) GetBootstrapCapabilities(ctx context.Context, env string, includeMethods, includeSchemas bool) (map[string]*ResolveOutput, error) {
	if r.repo == nil {
		return map[string]*ResolveOutput{}, nil
	}
	entries, err := r.repo.ListBootstrapEntries(ctx, env)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*ResolveOutput, len(entries))
	natsUrl := r.config.NatsUrl
	if natsUrl == "" {
		natsUrl = "nats://127.0.0.1:4222"
	}
	alias := r.config.DefaultAlias
	if alias == "" {
		alias = defaultAlias
	}
	for _, e := range entries {
		capRef := e.App + "." + e.Name
		subject := r.buildSubject(e.App, e.Name, e.DefaultMajor)
		canonicalIdentity := fmt.Sprintf("cap:@%s/%s/%s@%s", alias, e.App, e.Name, e.VersionString)
		ro := &ResolveOutput{
			CanonicalIdentity: canonicalIdentity,
			NatsUrl:           natsUrl,
			Subject:           subject,
			Major:             e.DefaultMajor,
			ResolvedVersion:   e.VersionString,
			Status:            e.VersionStatus,
			TTLSeconds:        0,
			Etag:              "bootstrap",
		}
		if includeMethods || includeSchemas {
			methods, err := r.repo.GetMethods(ctx, e.VersionID)
			if err == nil {
				if includeMethods {
					ro.Methods = make([]MethodInfo, len(methods))
					for i, m := range methods {
						ro.Methods[i] = MethodInfo{
							Name:        m.Name,
							Description: ptrStringOr(m.Description, ""),
							Modes:       m.Modes,
							Tags:        m.Tags,
						}
					}
				}
				if includeSchemas {
					ro.Schemas = make(map[string]Schema, len(methods))
					for _, m := range methods {
						ro.Schemas[m.Name] = Schema{
							Input:  jsonBytesToMap(m.InputSchema),
							Output: jsonBytesToMap(m.OutputSchema),
						}
					}
				}
			}
		}
		out[capRef] = ro
	}
	return out, nil
}
