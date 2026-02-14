package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/morezero/capabilities-registry/pkg/semver"
)

const listMajorsLogPrefix = "registry:listMajors"

// ListMajors returns all major versions for a capability.
func (r *Registry) ListMajors(ctx context.Context, input *ListMajorsInput) (*ListMajorsOutput, error) {
	slog.Info(fmt.Sprintf("%s - cap=%s", listMajorsLogPrefix, input.Cap))

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

	defaultEntry, _ := r.repo.GetDefault(ctx, cap.ID, r.config.DefaultEnv)

	// Group by major
	type majorGroup struct {
		versions []semver.VersionRecord
	}
	majorMap := make(map[int]*majorGroup)

	for _, v := range versions {
		if !input.IncludeInactive && v.Status == "disabled" {
			continue
		}
		pre := ""
		if v.Prerelease != nil {
			pre = *v.Prerelease
		}
		rec := semver.VersionRecord{
			ID:            v.ID,
			Major:         v.Major,
			Minor:         v.Minor,
			Patch:         v.Patch,
			Prerelease:    pre,
			Status:        v.Status,
			VersionString: semver.ToVersionString(v.Major, v.Minor, v.Patch, pre),
		}
		if g, ok := majorMap[v.Major]; ok {
			g.versions = append(g.versions, rec)
		} else {
			majorMap[v.Major] = &majorGroup{versions: []semver.VersionRecord{rec}}
		}
	}

	majors := make([]MajorInfo, 0, len(majorMap))
	for major, group := range majorMap {
		// Sort versions descending within major
		sort.Slice(group.versions, func(i, j int) bool {
			a, b := group.versions[i], group.versions[j]
			if a.Minor != b.Minor {
				return a.Minor > b.Minor
			}
			return a.Patch > b.Patch
		})

		latest := group.versions[0]
		isDefault := false
		if defaultEntry != nil {
			isDefault = defaultEntry.DefaultMajor == major
		}

		majors = append(majors, MajorInfo{
			Major:         major,
			LatestVersion: latest.VersionString,
			Status:        latest.Status,
			VersionCount:  len(group.versions),
			IsDefault:     isDefault,
		})
	}

	// Sort majors descending
	sort.Slice(majors, func(i, j int) bool {
		return majors[i].Major > majors[j].Major
	})

	return &ListMajorsOutput{Majors: majors}, nil
}
