package registry

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/semver"
)

const (
	discoverLogPrefix   = "registry:discover"
	discoverDefaultLimit = 20
	discoverMaxLimit     = 500
)

// Discover lists capabilities matching filters.
func (r *Registry) Discover(ctx context.Context, input *DiscoverInput) (*DiscoverOutput, error) {
	slog.Info(fmt.Sprintf("%s - app=%s query=%s", discoverLogPrefix, input.App, input.Query))

	if err := r.requireRepo(); err != nil {
		return nil, err
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	limit := input.Limit
	if limit < 1 {
		limit = discoverDefaultLimit
	}
	if limit > discoverMaxLimit {
		limit = discoverMaxLimit
	}

	status := input.Status
	if status == "" {
		status = "Active"
	}

	caps, total, err := r.repo.ListCapabilities(ctx, db.ListCapabilitiesParams{
		App:    input.App,
		Tags:   input.Tags,
		Query:  input.Query,
		Status: status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	env := r.getEnv(input.Ctx)

	capIDs := make([]string, 0, len(caps))
	for _, c := range caps {
		capIDs = append(capIDs, c.ID)
	}

	versionsByCap, err := r.repo.GetVersionsByCapabilityIDs(ctx, capIDs)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}
	defaultsByCap, err := r.repo.GetDefaultsBatch(ctx, capIDs, env)
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	capabilities := make([]DiscoveredCapability, 0, len(caps))
	for _, cap := range caps {
		versions := versionsByCap[cap.ID]
		records := dbVersionsToRecords(versions)
		majors := semver.GetUniqueMajors(records)

		defaultEntry := defaultsByCap[cap.ID]
		defaultMajor := 1
		if defaultEntry != nil {
			defaultMajor = defaultEntry.DefaultMajor
		} else if len(majors) > 0 {
			defaultMajor = majors[0]
		}

		latestVersion := "0.0.0"
		if len(records) > 0 {
			latestVersion = records[0].VersionString
		}

		desc := ""
		if cap.Description != nil {
			desc = *cap.Description
		}

		capabilities = append(capabilities, DiscoveredCapability{
			Cap:           fmt.Sprintf("%s.%s", cap.App, cap.Name),
			App:           cap.App,
			Name:          cap.Name,
			Description:   desc,
			Tags:          cap.Tags,
			DefaultMajor:  defaultMajor,
			LatestVersion: latestVersion,
			Majors:        majors,
			Status:        cap.Status,
		})
	}

	return &DiscoverOutput{
		Capabilities: capabilities,
		Pagination: Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: int(math.Ceil(float64(total) / float64(limit))),
		},
	}, nil
}
