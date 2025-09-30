package event

import "context"

// PublisherTransport defines how events are dispatched.
// Implementations send events to processors for execution.
type PublisherTransport interface {
	// Dispatch sends an event for processing.
	// Returns an error if dispatch fails (e.g., buffer full).
	Dispatch(ctx context.Context, eventName string, payload any) error
}

// ProcessorTransport defines how events are received and processed.
// Inspired by Watermill's Subscriber pattern - transport is a passive wire.
// Implementations provide a channel of events for the Processor to consume.
type ProcessorTransport interface {
	// Subscribe returns a channel of events to process.
	// Returns nil for sync transports (special case - immediate execution).
	// The Processor manages workers that consume from this channel.
	Subscribe(ctx context.Context) (<-chan envelope, error)

	// Close performs cleanup and releases resources.
	Close() error
}
