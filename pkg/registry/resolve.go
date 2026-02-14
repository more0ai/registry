package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/semver"
)

const resolveLogPrefix = "registry:resolve"

// Resolve finds the best matching version for a capability reference.
// Supports both local resolution and federated resolution (cross-sandbox).
//
// Federation logic:
//  1. Parse the incoming capability reference
//  2. If the reference contains an @alias prefix (e.g. "@partner/my.app/cap"),
//     check if that alias is different from the default
//  3. If the alias is remote, delegate to the federation pool
//  4. If local or no alias, resolve normally from the local database
//  5. All responses include natsUrl — the NATS server where the subject lives
func (r *Registry) Resolve(ctx context.Context, input *ResolveInput) (*ResolveOutput, error) {
	slog.Info(fmt.Sprintf("%s - cap=%s ver=%s", resolveLogPrefix, input.Cap, input.Ver))

	if err := r.requireRepo(); err != nil {
		return nil, err
	}

	// Check for alias prefix (e.g. "@partner/my.app/my.cap")
	alias, capRef := extractAlias(input.Cap)
	defaultAlias := "main"
	if r.config.DefaultAlias != "" {
		defaultAlias = r.config.DefaultAlias
	}

	// If alias is present and different from default, try federated resolution
	if alias != "" && alias != defaultAlias {
		return r.resolveRemote(ctx, input, alias, capRef)
	}

	// Local resolution (alias is default or empty)
	return r.resolveLocal(ctx, input, defaultAlias, capRef)
}

// resolveLocal resolves a capability from the local database.
func (r *Registry) resolveLocal(ctx context.Context, input *ResolveInput, defaultAlias string, capRef string) (*ResolveOutput, error) {
	// Use the original cap if capRef is empty (no alias was extracted)
	resolveRef := capRef
	if resolveRef == "" {
		resolveRef = input.Cap
	}

	parsed, err := semver.ParseCapabilityRef(resolveRef)
	if err != nil {
		return nil, &RegistryError{Code: "INVALID_ARGUMENT", Message: err.Error()}
	}

	rangeStr := input.Ver
	if rangeStr == "" {
		rangeStr = parsed.Range
	}

	// Get capability
	cap, err := r.repo.GetCapability(ctx, parsed.App, parsed.Name)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}
	if cap == nil {
		return nil, &RegistryError{Code: "NOT_FOUND", Message: fmt.Sprintf("Capability not found: %s", parsed.Full)}
	}

	// Get versions
	versions, err := r.repo.GetVersions(ctx, cap.ID)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}
	if len(versions) == 0 {
		return nil, &RegistryError{Code: "NOT_FOUND", Message: fmt.Sprintf("No versions found for capability: %s", parsed.Full)}
	}

	// Get default major
	env := r.getEnv(input.Ctx)
	defaultEntry, _ := r.repo.GetDefault(ctx, cap.ID, env)
	defaultMajor := -1
	if defaultEntry != nil {
		defaultMajor = defaultEntry.DefaultMajor
	}

	// Convert to VersionRecords
	records := dbVersionsToRecords(versions)

	// Resolve
	resolved := semver.ResolveVersion(semver.ResolveVersionParams{
		Versions:          records,
		Range:             rangeStr,
		DefaultMajor:      defaultMajor,
		IncludeDeprecated: true,
		ExcludeDisabled:   true,
	})

	if resolved == nil {
		return nil, &RegistryError{
			Code:    "NOT_FOUND",
			Message: fmt.Sprintf("No matching version for %s@%s", parsed.Full, orDefault(rangeStr, "default")),
		}
	}

	// Check tenant access
	if input.Ctx != nil && input.Ctx.TenantID != "" {
		allowed, reason := r.repo.CheckTenantAccess(ctx, cap.ID, resolved.Major, db.ResolutionContext{
			TenantID: input.Ctx.TenantID,
			Env:      input.Ctx.Env,
			Aud:      input.Ctx.Aud,
			Features: input.Ctx.Features,
		})
		if !allowed {
			return nil, &RegistryError{Code: "FORBIDDEN", Message: reason}
		}
	}

	// Build response — always include natsUrl (local server URL)
	subject := r.buildSubject(parsed.App, parsed.Name, resolved.Major)
	canonicalIdentity := fmt.Sprintf("cap:@%s/%s/%s@%s", defaultAlias, parsed.App, parsed.Name, resolved.VersionString)
	natsUrl := r.config.NatsUrl
	if natsUrl == "" {
		natsUrl = "nats://127.0.0.1:4222"
	}

	result := &ResolveOutput{
		CanonicalIdentity: canonicalIdentity,
		NatsUrl:           natsUrl,
		Subject:           subject,
		Major:             resolved.Major,
		ResolvedVersion:   resolved.VersionString,
		Status:            resolved.Status,
		TTLSeconds:        r.config.DefaultTTLSeconds,
		Etag:              fmt.Sprintf("%s-%d", cap.ID, cap.Revision),
	}

	// Include methods if requested
	if input.IncludeMethods || input.IncludeSchemas {
		versionID := resolved.ID
		methods, err := r.repo.GetMethods(ctx, versionID)
		if err == nil {
			if input.IncludeMethods {
				result.Methods = make([]MethodInfo, len(methods))
				for i, m := range methods {
					result.Methods[i] = MethodInfo{
						Name:        m.Name,
						Description: ptrStringOr(m.Description, ""),
						Modes:       m.Modes,
						Tags:        m.Tags,
					}
				}
			}
			if input.IncludeSchemas {
				result.Schemas = make(map[string]Schema, len(methods))
				for _, m := range methods {
					result.Schemas[m.Name] = Schema{
						Input:  jsonBytesToMap(m.InputSchema),
						Output: jsonBytesToMap(m.OutputSchema),
					}
				}
			}
		}
	}

	return result, nil
}

// resolveRemote resolves a capability via a remote/federated registry.
func (r *Registry) resolveRemote(ctx context.Context, input *ResolveInput, alias string, capRef string) (*ResolveOutput, error) {
	slog.Info(fmt.Sprintf("%s - Federated resolve alias=%s cap=%s", resolveLogPrefix, alias, capRef))

	if r.federationPool == nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: "Federation pool not initialized"}
	}

	fedResult, err := r.federationPool.Resolve(ctx, &FederatedResolveInput{
		Alias: alias,
		Cap:   capRef,
		Ver:   input.Ver,
		Ctx:   input.Ctx,
	})
	if err != nil {
		return nil, err
	}

	return &ResolveOutput{
		CanonicalIdentity: fedResult.CanonicalIdentity,
		NatsUrl:           fedResult.NatsUrl,
		Subject:           fedResult.Subject,
		Major:             fedResult.Major,
		ResolvedVersion:   fedResult.ResolvedVersion,
		Status:            fedResult.Status,
		TTLSeconds:        fedResult.TTLSeconds,
		Etag:              fedResult.Etag,
	}, nil
}

// extractAlias extracts an @alias prefix from a capability reference.
// Returns (alias, remaining) or ("", original) if no alias found.
// Examples:
//
//	"@partner/my.app/my.cap" → ("partner", "my.app/my.cap")
//	"my.app/my.cap"          → ("", "my.app/my.cap")
func extractAlias(capRef string) (string, string) {
	if !strings.HasPrefix(capRef, "@") {
		return "", capRef
	}
	// Find the first "/" after the @
	rest := capRef[1:] // strip @
	idx := strings.Index(rest, "/")
	if idx < 0 {
		// Just "@alias" with no slash — treat as alias with no cap
		return rest, ""
	}
	return rest[:idx], rest[idx+1:]
}

func dbVersionsToRecords(versions []db.CapabilityVersion) []semver.VersionRecord {
	records := make([]semver.VersionRecord, len(versions))
	for i, v := range versions {
		pre := ""
		if v.Prerelease != nil {
			pre = *v.Prerelease
		}
		records[i] = semver.VersionRecord{
			ID:            v.ID,
			Major:         v.Major,
			Minor:         v.Minor,
			Patch:         v.Patch,
			Prerelease:    pre,
			Status:        v.Status,
			VersionString: semver.ToVersionString(v.Major, v.Minor, v.Patch, pre),
		}
	}
	return records
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func ptrStringOr(p *string, def string) string {
	if p != nil {
		return *p
	}
	return def
}

func jsonBytesToMap(data []byte) map[string]interface{} {
	if data == nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}
