package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/morezero/capabilities-registry/pkg/db"
	"github.com/morezero/capabilities-registry/pkg/events"
	"github.com/morezero/capabilities-registry/pkg/semver"
)

const (
	upsertLogPrefix       = "registry:upsert"
	maxVersionComponent   = 9999
	maxUpsertMethods      = 200
	maxMethodSchemaBytes  = 256 * 1024 // 256KB per schema
	maxMethodExamplesBytes = 64 * 1024 // 64KB per method examples
	maxMetadataBytes      = 64 * 1024  // 64KB for version metadata
)

// validateUpsertInput checks app, name, version bounds, method count, method names, and payload sizes.
func validateUpsertInput(input *UpsertInput) *RegistryError {
	if !semver.ValidateAppName(input.App) {
		return &RegistryError{Code: "INVALID_ARGUMENT", Message: "app must be lowercase alphanumeric with hyphens only"}
	}
	if !semver.ValidateCapabilityName(input.Name) {
		return &RegistryError{Code: "INVALID_ARGUMENT", Message: "name must start with a letter and contain only letters, digits, dots, hyphens, underscores"}
	}
	v := &input.Version
	if v.Major < 0 || v.Major > maxVersionComponent || v.Minor < 0 || v.Minor > maxVersionComponent || v.Patch < 0 || v.Patch > maxVersionComponent {
		return &RegistryError{Code: "INVALID_ARGUMENT", Message: "version major, minor, patch must be 0-9999"}
	}
	if len(input.Methods) == 0 {
		return &RegistryError{Code: "INVALID_ARGUMENT", Message: "at least one method is required"}
	}
	if len(input.Methods) > maxUpsertMethods {
		return &RegistryError{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("methods count exceeds maximum %d", maxUpsertMethods)}
	}
	if input.Version.Metadata != nil {
		b, _ := json.Marshal(input.Version.Metadata)
		if len(b) > maxMetadataBytes {
			return &RegistryError{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("version metadata exceeds %d bytes", maxMetadataBytes)}
		}
	}
	for _, m := range input.Methods {
		if !semver.ValidateCapabilityName(m.Name) {
			return &RegistryError{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("method name %q invalid: must start with a letter and contain only letters, digits, dots, hyphens, underscores", m.Name)}
		}
		if m.InputSchema != nil {
			b, _ := json.Marshal(m.InputSchema)
			if len(b) > maxMethodSchemaBytes {
				return &RegistryError{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("method %q inputSchema exceeds %d bytes", m.Name, maxMethodSchemaBytes)}
			}
		}
		if m.OutputSchema != nil {
			b, _ := json.Marshal(m.OutputSchema)
			if len(b) > maxMethodSchemaBytes {
				return &RegistryError{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("method %q outputSchema exceeds %d bytes", m.Name, maxMethodSchemaBytes)}
			}
		}
		if len(m.Examples) > 0 {
			b, _ := json.Marshal(m.Examples)
			if len(b) > maxMethodExamplesBytes {
				return &RegistryError{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("method %q examples exceed %d bytes", m.Name, maxMethodExamplesBytes)}
			}
		}
	}
	return nil
}

// Upsert creates or updates a capability with a version and methods.
func (r *Registry) Upsert(ctx context.Context, input *UpsertInput, userID string) (*UpsertOutput, error) {
	slog.Info(fmt.Sprintf("%s - app=%s name=%s version=%d.%d.%d",
		upsertLogPrefix, input.App, input.Name,
		input.Version.Major, input.Version.Minor, input.Version.Patch))

	if err := r.requireRepo(); err != nil {
		return nil, err
	}

	if err := validateUpsertInput(input); err != nil {
		return nil, err
	}

	existingCap, err := r.repo.GetCapability(ctx, input.App, input.Name)
	if err != nil {
		slog.Error(fmt.Sprintf("%s - GetCapability failed: %v", upsertLogPrefix, err))
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: "Failed to look up capability"}
	}
	var prerelease *string
	if input.Version.Prerelease != "" {
		prerelease = &input.Version.Prerelease
	}
	existingVersion := (*db.CapabilityVersion)(nil)
	if existingCap != nil {
		var verErr error
		existingVersion, verErr = r.repo.GetVersion(ctx, db.GetVersionParams{
			CapabilityID: existingCap.ID,
			Major:        input.Version.Major,
			Minor:        input.Version.Minor,
			Patch:        input.Version.Patch,
			Prerelease:   prerelease,
		})
		if verErr != nil {
			slog.Error(fmt.Sprintf("%s - GetVersion failed: %v", upsertLogPrefix, verErr))
		}
	}

	// Upsert capability
	var desc *string
	if input.Description != "" {
		desc = &input.Description
	}
	cap, err := r.repo.UpsertCapability(ctx, db.UpsertCapabilityParams{
		App:         input.App,
		Name:        input.Name,
		Description: desc,
		Tags:        input.Tags,
		UserID:      userID,
	})
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	// Upsert version
	var vDesc, vChangelog *string
	if input.Version.Description != "" {
		vDesc = &input.Version.Description
	}
	if input.Version.Changelog != "" {
		vChangelog = &input.Version.Changelog
	}

	version, err := r.repo.UpsertVersion(ctx, db.UpsertVersionParams{
		CapabilityID: cap.ID,
		Major:        input.Version.Major,
		Minor:        input.Version.Minor,
		Patch:        input.Version.Patch,
		Prerelease:   prerelease,
		Description:  vDesc,
		Changelog:    vChangelog,
		Metadata:     input.Version.Metadata,
		UserID:       userID,
	})
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	// Delete existing methods and insert new ones
	if err := r.repo.DeleteMethods(ctx, version.ID); err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	for _, method := range input.Methods {
		var mDesc *string
		if method.Description != "" {
			mDesc = &method.Description
		}
		_, err := r.repo.UpsertMethod(ctx, db.UpsertMethodParams{
			VersionID:    version.ID,
			Name:         method.Name,
			Description:  mDesc,
			InputSchema:  method.InputSchema,
			OutputSchema: method.OutputSchema,
			Modes:        method.Modes,
			Tags:         method.Tags,
			Policies:     method.Policies,
			Examples:     method.Examples,
			UserID:       userID,
		})
		if err != nil {
			return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
		}
	}

	// Set as default if requested
	if input.SetAsDefault {
		env := input.Env
		if env == "" {
			env = r.config.DefaultEnv
		}
		_, err := r.repo.SetDefault(ctx, db.SetDefaultParams{
			CapabilityID: cap.ID,
			Major:        input.Version.Major,
			Env:          env,
			UserID:       userID,
		})
		if err != nil {
			return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
		}
	}

	// Increment revision and publish event
	revision, revErr := r.repo.IncrementRevision(ctx, cap.ID)
	if revErr != nil {
		slog.Error(fmt.Sprintf("%s - IncrementRevision failed: %v", upsertLogPrefix, revErr))
		revision = cap.Revision
	}
	if err := r.publisher.PublishChanged(ctx, &events.RegistryChangedEvent{
		App:            input.App,
		Capability:     input.Name,
		ChangedFields:  []string{"version", "methods"},
		AffectedMajors: []int{input.Version.Major},
		Revision:       revision,
		Etag:           fmt.Sprintf("%s-%d", cap.ID, revision),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		slog.Error(fmt.Sprintf("%s - PublishChanged failed: %v", upsertLogPrefix, err))
	}

	pre := ""
	if prerelease != nil {
		pre = *prerelease
	}
	subject := r.buildSubject(input.App, input.Name, input.Version.Major)

	action := "created"
	if existingCap != nil || existingVersion != nil {
		action = "updated"
	}

	return &UpsertOutput{
		Action:       action,
		CapabilityID: cap.ID,
		VersionID:    version.ID,
		Cap:          fmt.Sprintf("%s.%s", input.App, input.Name),
		Version:      semver.ToVersionString(input.Version.Major, input.Version.Minor, input.Version.Patch, pre),
		Subject:      subject,
	}, nil
}
