package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/morezero/capabilities-registry/pkg/semver"
)

const describeLogPrefix = "registry:describe"

// Describe returns full details for a capability version, including capability-level
// metadata (cap, app, name, description, version, major, status, tags, changelog) and
// for each method full metadata: name, description, inputSchema, outputSchema (JSON schemas),
// modes, tags, and examples.
func (r *Registry) Describe(ctx context.Context, input *DescribeInput) (*DescribeOutput, error) {
	slog.Info(fmt.Sprintf("%s - cap=%s", describeLogPrefix, input.Cap))

	if err := r.requireRepo(); err != nil {
		return nil, err
	}

	parsed, err := semver.ParseCapabilityRef(input.Cap)
	if err != nil {
		return nil, &RegistryError{Code: "INVALID_ARGUMENT", Message: err.Error()}
	}

	cap, err := r.repo.GetCapability(ctx, parsed.App, parsed.Name)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}
	if cap == nil {
		return nil, &RegistryError{Code: "NOT_FOUND", Message: fmt.Sprintf("Capability not found: %s", parsed.Full)}
	}

	versions, err := r.repo.GetVersions(ctx, cap.ID)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}
	if len(versions) == 0 {
		return nil, &RegistryError{Code: "NOT_FOUND", Message: fmt.Sprintf("No versions found for: %s", parsed.Full)}
	}

	// Find target version: by exact Version string, or by Major, or latest
	targetVersion := versions[0]
	if input.Version != "" {
		found := false
		for _, v := range versions {
			pre := ""
			if v.Prerelease != nil {
				pre = *v.Prerelease
			}
			if semver.ToVersionString(v.Major, v.Minor, v.Patch, pre) == input.Version {
				targetVersion = v
				found = true
				break
			}
		}
		if !found {
			return nil, &RegistryError{
				Code:    "NOT_FOUND",
				Message: fmt.Sprintf("Version %s not found for: %s", input.Version, parsed.Full),
			}
		}
	} else if input.Major != nil {
		found := false
		for _, v := range versions {
			if v.Major == *input.Major {
				targetVersion = v
				found = true
				break
			}
		}
		if !found {
			return nil, &RegistryError{
				Code:    "NOT_FOUND",
				Message: fmt.Sprintf("Major version %d not found for: %s", *input.Major, parsed.Full),
			}
		}
	}

	methods, err := r.repo.GetMethods(ctx, targetVersion.ID)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	pre := ""
	if targetVersion.Prerelease != nil {
		pre = *targetVersion.Prerelease
	}

	desc := ""
	if cap.Description != nil {
		desc = *cap.Description
	}

	changelog := ""
	if targetVersion.Changelog != nil {
		changelog = *targetVersion.Changelog
	}

	methodDescs := make([]MethodDescription, len(methods))
	for i, m := range methods {
		mDesc := ""
		if m.Description != nil {
			mDesc = *m.Description
		}
		methodDescs[i] = MethodDescription{
			Name:         m.Name,
			Description:  mDesc,
			InputSchema:  jsonBytesToMap(m.InputSchema),
			OutputSchema: jsonBytesToMap(m.OutputSchema),
			Modes:        m.Modes,
			Tags:         m.Tags,
			Examples:     jsonBytesToSlice(m.Examples),
		}
	}

	return &DescribeOutput{
		Cap:         fmt.Sprintf("%s.%s", cap.App, cap.Name),
		App:         cap.App,
		Name:        cap.Name,
		Description: desc,
		Version:     semver.ToVersionString(targetVersion.Major, targetVersion.Minor, targetVersion.Patch, pre),
		Major:       targetVersion.Major,
		Status:      targetVersion.Status,
		Methods:     methodDescs,
		Tags:        cap.Tags,
		Changelog:   changelog,
	}, nil
}

func jsonBytesToSlice(data []byte) []interface{} {
	if data == nil {
		return []interface{}{}
	}
	var s []interface{}
	if err := json.Unmarshal(data, &s); err != nil {
		return []interface{}{}
	}
	return s
}
