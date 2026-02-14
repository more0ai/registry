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

const setDefaultLogPrefix = "registry:setDefaultMajor"

// SetDefaultMajor sets the default major version for a capability in an environment.
func (r *Registry) SetDefaultMajor(ctx context.Context, input *SetDefaultMajorInput, userID string) (*SetDefaultMajorOutput, error) {
	slog.Info(fmt.Sprintf("%s - cap=%s major=%d", setDefaultLogPrefix, input.Cap, input.Major))

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

	env := input.Env
	if env == "" {
		env = r.config.DefaultEnv
	}

	existingDefault, _ := r.repo.GetDefault(ctx, cap.ID, env)

	_, err = r.repo.SetDefault(ctx, db.SetDefaultParams{
		CapabilityID: cap.ID,
		Major:        input.Major,
		Env:          env,
		UserID:       userID,
	})
	if err != nil {
		return nil, &RegistryError{Code: "INTERNAL_ERROR", Message: err.Error()}
	}

	// Publish event
	revision, _ := r.repo.IncrementRevision(ctx, cap.ID)
	newMajor := input.Major
	_ = r.publisher.PublishChanged(ctx, &events.RegistryChangedEvent{
		App:             parsed.App,
		Capability:      parsed.Name,
		ChangedFields:   []string{"defaultMajor"},
		NewDefaultMajor: &newMajor,
		AffectedMajors:  []int{input.Major},
		Revision:        revision,
		Etag:            fmt.Sprintf("%s-%d", cap.ID, revision),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	})

	result := &SetDefaultMajorOutput{
		Success:  true,
		NewMajor: input.Major,
	}
	if existingDefault != nil {
		result.PreviousMajor = &existingDefault.DefaultMajor
	}

	return result, nil
}
