package command

import (
	"context"
	"log/slog"
)

// Dispatcher dispatches commands to handlers via transport.
// It is a stateless client with no lifecycle management.
// For processing commands with handlers and lifecycle, use Processor.
//
// Example:
//
//	dispatcher := command.NewDispatcher(transport)
//	err := dispatcher.Dispatch(ctx, CreateUser{Email: "user@example.com"})
type Dispatcher struct {
	transport DispatcherTransport
	logger    *slog.Logger
}

// DispatcherOption configures a Dispatcher.
type DispatcherOption func(*Dispatcher)

// NewDispatcher creates a new command dispatcher with the given transport.
//
// Example:
//
//	transport := command.NewChannelTransport(100)
//	dispatcher := command.NewDispatcher(transport, command.WithDispatcherLogger(logger))
func NewDispatcher(transport DispatcherTransport, opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		transport: transport,
		logger:    slog.Default(),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Dispatch sends a command for execution via the configured transport.
// The command is routed to its registered handler by the processor.
//
// For sync transport: blocks until handler completes, returns handler error.
// For async transports: returns immediately, returns dispatch error only.
//
// Example:
//
//	err := dispatcher.Dispatch(ctx, CreateUser{
//	    Email: "user@example.com",
//	    Name:  "John Doe",
//	})
func (d *Dispatcher) Dispatch(ctx context.Context, cmd any) error {
	cmdName := getCommandNameFromInstance(cmd)
	return d.transport.Dispatch(ctx, cmdName, cmd)
}

// WithDispatcherLogger sets the logger for the dispatcher.
// If not set, slog.Default() is used.
//
// Example:
//
//	dispatcher := command.NewDispatcher(transport, command.WithDispatcherLogger(logger))
func WithDispatcherLogger(logger *slog.Logger) DispatcherOption {
	return func(d *Dispatcher) {
		d.logger = logger
	}
}
