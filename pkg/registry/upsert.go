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

const upsertLogPrefix = "registry:upsert"

// Upsert creates or updates a capability with a version and methods.
func (r *Registry) Upsert(ctx context.Context, input *UpsertInput, userID string) (*UpsertOutput, error) {
	slog.Info(fmt.Sprintf("%s - app=%s name=%s version=%d.%d.%d",
		upsertLogPrefix, input.App, input.Name,
		input.Version.Major, input.Version.Minor, input.Version.Patch))

	if err := r.requireRepo(); err != nil {
		return nil, err
	}

	existingCap, _ := r.repo.GetCapability(ctx, input.App, input.Name)
	var prerelease *string
	if input.Version.Prerelease != "" {
		prerelease = &input.Version.Prerelease
	}
	existingVersion := (*db.CapabilityVersion)(nil)
	if existingCap != nil {
		existingVersion, _ = r.repo.GetVersion(ctx, db.GetVersionParams{
			CapabilityID: existingCap.ID,
			Major:        input.Version.Major,
			Minor:        input.Version.Minor,
			Patch:        input.Version.Patch,
			Prerelease:   prerelease,
		})
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
	revision, _ := r.repo.IncrementRevision(ctx, cap.ID)
	_ = r.publisher.PublishChanged(ctx, &events.RegistryChangedEvent{
		App:            input.App,
		Capability:     input.Name,
		ChangedFields:  []string{"version", "methods"},
		AffectedMajors: []int{input.Version.Major},
		Revision:       revision,
		Etag:           fmt.Sprintf("%s-%d", cap.ID, revision),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})

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
