package command

import "context"

// DispatcherTransport defines how commands are dispatched from clients.
// Implementations send commands to be processed asynchronously or synchronously.
type DispatcherTransport interface {
	// Dispatch sends a command for execution.
	// The cmdName identifies the handler, payload contains the command data.
	// Returns an error if dispatch fails (e.g., buffer full, handler not found).
	Dispatch(ctx context.Context, cmdName string, payload any) error
}

// ProcessorTransport defines how commands are received and processed.
// Inspired by Watermill's Subscriber pattern - transport is a passive wire.
// Implementations provide a channel of commands for the Processor to consume.
type ProcessorTransport interface {
	// Subscribe returns a channel of commands to process.
	// Returns nil for sync transports (special case - immediate execution).
	// The Processor manages workers that consume from this channel.
	Subscribe(ctx context.Context) (<-chan envelope, error)

	// Close performs cleanup and releases resources.
	Close() error
}
