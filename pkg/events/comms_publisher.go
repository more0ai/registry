package events

import (
	"context"
	"fmt"
	"log/slog"

	comms "github.com/nats-io/nats.go"

	"github.com/morezero/capabilities-registry/pkg/commsutil"
)

const commsPublisherLogPrefix = "events:comms_publisher"

// CommsPublisherOpts configures CommsPublisher. Nil or zero values use defaults.
type CommsPublisherOpts struct {
	// GlobalChangeSubject overrides the global change event subject (e.g. from REGISTRY_CHANGE_EVENT_SUBJECT).
	GlobalChangeSubject string
}

// CommsPublisher publishes registry change events to COMMS subjects.
type CommsPublisher struct {
	nc                  *comms.Conn
	globalChangeSubject string
}

// NewCommsPublisher creates a new CommsPublisher. Pass nil for opts to use defaults.
func NewCommsPublisher(nc *comms.Conn, opts *CommsPublisherOpts) *CommsPublisher {
	globalSubject := commsutil.SubjectChangeEvent
	if opts != nil && opts.GlobalChangeSubject != "" {
		globalSubject = opts.GlobalChangeSubject
	}
	return &CommsPublisher{nc: nc, globalChangeSubject: globalSubject}
}

// PublishChanged publishes a RegistryChangedEvent to both the granular
// and global change event subjects.
func (p *CommsPublisher) PublishChanged(_ context.Context, event *RegistryChangedEvent) error {
	data, err := commsutil.EncodePayload(event)
	if err != nil {
		return fmt.Errorf("%s - failed to encode event: %w", commsPublisherLogPrefix, err)
	}

	// Publish to granular subject
	granularSubject := commsutil.BuildChangeSubject(event.App, event.Capability)
	if err := p.nc.Publish(granularSubject, data); err != nil {
		slog.Error(fmt.Sprintf("%s - failed to publish to %s: %v", commsPublisherLogPrefix, granularSubject, err))
		return err
	}

	// Publish to global subject
	if err := p.nc.Publish(p.globalChangeSubject, data); err != nil {
		slog.Error(fmt.Sprintf("%s - failed to publish to %s: %v", commsPublisherLogPrefix, p.globalChangeSubject, err))
		return err
	}

	slog.Debug(fmt.Sprintf("%s - Published change event for %s.%s", commsPublisherLogPrefix, event.App, event.Capability))
	return nil
}
