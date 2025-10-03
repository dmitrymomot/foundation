package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// DispatcherOption configures a Dispatcher.
type DispatcherOption func(*Dispatcher)

// WithHandler registers one or more handlers with the dispatcher.
// Only one handler can be registered per command type.
// If a handler is already registered for a command type, this function will panic.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithHandler(handler1),
//	    command.WithHandler(handler2),
//	)
func WithHandler(handlers ...Handler) DispatcherOption {
	return func(d *Dispatcher) {
		for _, h := range handlers {
			commandName := h.CommandName()

			if _, exists := d.handlers[commandName]; exists {
				panic(fmt.Sprintf("handler already registered for command: %s", commandName))
			}

			d.handlers[commandName] = h
		}
	}
}

// WithCommandSource sets the command source for the dispatcher to pull commands from.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithCommandSource(bus),
//	    command.WithHandler(handler1),
//	)
func WithCommandSource(source commandSource) DispatcherOption {
	return func(d *Dispatcher) {
		if source != nil {
			d.commandBus = source
		}
	}
}

// WithShutdownTimeout configures maximum wait time for active handlers during shutdown.
// Dispatcher will wait this long for running handlers to complete before forcing shutdown.
func WithShutdownTimeout(d time.Duration) DispatcherOption {
	return func(disp *Dispatcher) {
		if d > 0 {
			disp.shutdownTimeout = d
		}
	}
}

// WithMaxConcurrentHandlers limits the number of handlers that can execute concurrently.
// This prevents unbounded goroutine spawning under high load. Set to 0 (default) for unlimited.
// The limit applies across all command types - total concurrent handlers cannot exceed this value.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithCommandSource(bus),
//	    command.WithHandler(handler1),
//	    command.WithMaxConcurrentHandlers(100), // Max 100 concurrent handlers
//	)
func WithMaxConcurrentHandlers(max int) DispatcherOption {
	return func(d *Dispatcher) {
		if max >= 0 {
			d.maxConcurrentHandlers = max
		}
	}
}

// WithStaleThreshold configures the duration after which a dispatcher with no activity
// is considered stale in health checks. Default is 5 minutes.
func WithStaleThreshold(d time.Duration) DispatcherOption {
	return func(disp *Dispatcher) {
		if d > 0 {
			disp.staleThreshold = d
		}
	}
}

// WithStuckThreshold configures the number of active commands that triggers a stuck
// dispatcher warning in health checks. Default is 1000.
func WithStuckThreshold(threshold int32) DispatcherOption {
	return func(d *Dispatcher) {
		if threshold > 0 {
			d.stuckThreshold = threshold
		}
	}
}

// WithDispatcherLogger configures structured logging for dispatcher operations.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithDispatcherLogger(logger *slog.Logger) DispatcherOption {
	return func(d *Dispatcher) {
		if logger != nil {
			d.logger = logger
		}
	}
}

// WithFallbackHandler sets a fallback handler for commands with no registered handler.
// The fallback handler receives the full Command with all metadata (ID, Name, Payload, CreatedAt).
// Useful for logging unhandled commands, metrics, or forwarding to a dead letter queue.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithCommandSource(bus),
//	    command.WithFallbackHandler(func(ctx context.Context, cmd Command) error {
//	        log.Warn("unhandled command", "id", cmd.ID, "name", cmd.Name)
//	        return nil
//	    }),
//	)
func WithFallbackHandler(fn func(context.Context, Command) error) DispatcherOption {
	return func(d *Dispatcher) {
		if fn != nil {
			d.fallbackHandler = fn
		}
	}
}
