package events

import "context"

// EventPublisher is the interface for publishing registry change events.
type EventPublisher interface {
	PublishChanged(ctx context.Context, event *RegistryChangedEvent) error
}

// NoOpPublisher is an EventPublisher that does nothing (for in-process usage without events).
type NoOpPublisher struct{}

// PublishChanged is a no-op.
func (p *NoOpPublisher) PublishChanged(_ context.Context, _ *RegistryChangedEvent) error {
	return nil
}

// CallbackPublisher is an EventPublisher that calls a callback function (for testing).
type CallbackPublisher struct {
	callback func(ctx context.Context, event *RegistryChangedEvent) error
}

// NewCallbackPublisher creates a new CallbackPublisher.
func NewCallbackPublisher(cb func(ctx context.Context, event *RegistryChangedEvent) error) *CallbackPublisher {
	return &CallbackPublisher{callback: cb}
}

// PublishChanged calls the callback.
func (p *CallbackPublisher) PublishChanged(ctx context.Context, event *RegistryChangedEvent) error {
	return p.callback(ctx, event)
}
