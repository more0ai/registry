package registry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/events"
	"github.com/morezero/capabilities-registry/pkg/semver"
)

const deprecateLogPrefix = "registry:deprecate"

// Deprecate marks versions of a capability as deprecated.
func (r *Registry) Deprecate(ctx context.Context, input *DeprecateInput, userID string) (*DeprecateOutput, error) {
	slog.Info(fmt.Sprintf("%s - cap=%s", deprecateLogPrefix, input.Cap))

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

	var affectedVersions []string
	affectedMajorsMap := make(map[int]bool)

	for _, v := range versions {
		pre := ""
		if v.Prerelease != nil {
			pre = *v.Prerelease
		}
		vStr := semver.ToVersionString(v.Major, v.Minor, v.Patch, pre)

		shouldUpdate := false
		if input.Major != nil && v.Major == *input.Major {
			shouldUpdate = true
		} else if input.Version != "" && vStr == input.Version {
			shouldUpdate = true
		} else if input.Major == nil && input.Version == "" {
			shouldUpdate = true
		}

		if shouldUpdate {
			reason := input.Reason
			_, err := r.repo.UpdateVersionStatus(ctx, db.UpdateVersionStatusParams{
				VersionID: v.ID,
				Status:    "deprecated",
				Reason:    &reason,
				UserID:    userID,
			})
			if err != nil {
				return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("failed to deprecate version %s: %v", vStr, err)}
			}
			affectedVersions = append(affectedVersions, vStr)
			affectedMajorsMap[v.Major] = true
		}
	}

	affectedMajors := make([]int, 0, len(affectedMajorsMap))
	for m := range affectedMajorsMap {
		affectedMajors = append(affectedMajors, m)
	}

	// Publish event
	revision, _ := r.repo.IncrementRevision(ctx, cap.ID)
	_ = r.publisher.PublishChanged(ctx, &events.RegistryChangedEvent{
		App:            parsed.App,
		Capability:     parsed.Name,
		ChangedFields:  []string{"status"},
		AffectedMajors: affectedMajors,
		Revision:       revision,
		Etag:           fmt.Sprintf("%s-%d", cap.ID, revision),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})

	return &DeprecateOutput{
		Success:          true,
		AffectedVersions: affectedVersions,
	}, nil
}

// Disable marks versions of a capability as disabled.
func (r *Registry) Disable(ctx context.Context, input *DisableInput, userID string) (*DisableOutput, error) {
	slog.Info(fmt.Sprintf("%s - cap=%s", deprecateLogPrefix, input.Cap))

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

	var affectedVersions []string
	affectedMajorsMap := make(map[int]bool)

	for _, v := range versions {
		pre := ""
		if v.Prerelease != nil {
			pre = *v.Prerelease
		}
		vStr := semver.ToVersionString(v.Major, v.Minor, v.Patch, pre)

		shouldUpdate := false
		if input.Major != nil && v.Major == *input.Major {
			shouldUpdate = true
		} else if input.Version != "" && vStr == input.Version {
			shouldUpdate = true
		} else if input.Major == nil && input.Version == "" {
			shouldUpdate = true
		}

		if shouldUpdate {
			reason := input.Reason
			_, err := r.repo.UpdateVersionStatus(ctx, db.UpdateVersionStatusParams{
				VersionID: v.ID,
				Status:    "disabled",
				Reason:    &reason,
				UserID:    userID,
			})
			if err != nil {
				return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("failed to disable version %s: %v", vStr, err)}
			}
			affectedVersions = append(affectedVersions, vStr)
			affectedMajorsMap[v.Major] = true
		}
	}

	affectedMajors := make([]int, 0, len(affectedMajorsMap))
	for m := range affectedMajorsMap {
		affectedMajors = append(affectedMajors, m)
	}

	// Publish event
	revision, _ := r.repo.IncrementRevision(ctx, cap.ID)
	_ = r.publisher.PublishChanged(ctx, &events.RegistryChangedEvent{
		App:            parsed.App,
		Capability:     parsed.Name,
		ChangedFields:  []string{"status"},
		AffectedMajors: affectedMajors,
		Revision:       revision,
		Etag:           fmt.Sprintf("%s-%d", cap.ID, revision),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})

	return &DisableOutput{
		Success:          true,
		AffectedVersions: affectedVersions,
	}, nil
}
