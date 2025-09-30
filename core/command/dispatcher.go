package command

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Dispatcher is the central component that routes commands to their handlers.
// It supports multiple transport strategies (sync, async) and middleware.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithSyncTransport(),
//	    command.WithLogger(logger),
//	    command.WithMiddleware(command.LoggingMiddleware(logger)),
//	)
//	dispatcher.Register(command.NewHandlerFunc(createUserHandler))
//	dispatcher.Dispatch(ctx, CreateUser{Email: "user@example.com"})
type Dispatcher struct {
	handlers     map[string]Handler
	middleware   []Middleware
	transport    Transport
	errorHandler func(context.Context, string, error)
	logger       *slog.Logger
	mu           sync.RWMutex
}

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// NewDispatcher creates a new command dispatcher with the given options.
// If no transport is specified, WithSyncTransport() is used by default.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithSyncTransport(),
//	    command.WithLogger(logger),
//	)
func NewDispatcher(opts ...Option) *Dispatcher {
	d := &Dispatcher{
		handlers:   make(map[string]Handler),
		middleware: []Middleware{},
		logger:     slog.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(d)
	}

	// Default to sync transport if none specified
	if d.transport == nil {
		d.transport = newSyncTransport(d.getHandler)
	}

	return d
}

// Register registers a handler for a command type.
// Panics if a handler is already registered for the command.
//
// Example:
//
//	handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
//	    return db.Insert(ctx, cmd.Email, cmd.Name)
//	})
//	dispatcher.Register(handler)
func (d *Dispatcher) Register(handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cmdName := handler.Name()
	if _, exists := d.handlers[cmdName]; exists {
		panic(fmt.Sprintf("%s: %s", ErrDuplicateHandler, cmdName))
	}

	d.handlers[cmdName] = handler
}

// Dispatch sends a command for execution via the configured transport.
// The command is routed to its registered handler.
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

// Stop gracefully shuts down the dispatcher.
// For channel transport, this closes the channel and waits for workers to finish.
// For sync transport, this is a no-op.
//
// Example:
//
//	dispatcher := command.NewDispatcher(command.WithChannelTransport(100))
//	defer dispatcher.Stop()
func (d *Dispatcher) Stop() {
	// Check if transport supports graceful shutdown
	type stopper interface {
		Stop()
	}

	if s, ok := d.transport.(stopper); ok {
		s.Stop()
	}
}

// getHandler retrieves a handler by command name with middleware applied.
// This is used by transports to look up and execute handlers.
func (d *Dispatcher) getHandler(cmdName string) (Handler, bool) {
	d.mu.RLock()
	handler, exists := d.handlers[cmdName]
	middleware := d.middleware
	d.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Apply middleware to handler
	if len(middleware) > 0 {
		handler = chainMiddleware(handler, middleware)
	}

	return handler, true
}

// WithSyncTransport configures the dispatcher to use synchronous execution.
// Commands are executed immediately in the caller's goroutine.
//
// This is the default transport if none is specified.
//
// Example:
//
//	dispatcher := command.NewDispatcher(command.WithSyncTransport())
func WithSyncTransport() Option {
	return func(d *Dispatcher) {
		d.transport = newSyncTransport(d.getHandler)
	}
}

// WithChannelTransport configures the dispatcher to use channel-based async execution.
// Commands are dispatched to a buffered channel and processed by worker goroutines.
//
// Parameters:
// - bufferSize: Size of the command buffer (blocks when full)
// - opts: Optional configuration (worker count, etc.)
//
// Important: Call dispatcher.Stop() for graceful shutdown.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(100, command.WithWorkers(5)),
//	    command.WithErrorHandler(errorHandler),
//	)
//	defer dispatcher.Stop()
func WithChannelTransport(bufferSize int, opts ...ChannelOption) Option {
	return func(d *Dispatcher) {
		d.transport = newChannelTransport(
			bufferSize,
			d.getHandler,
			d.errorHandler,
			d.logger,
			opts...,
		)
	}
}

// WithErrorHandler sets a callback for handling errors from async transports.
// This is only used by async transports (channel) where errors cannot be
// returned to the caller.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(100),
//	    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	        logger.Error("command failed", "command", cmdName, "error", err)
//	    }),
//	)
func WithErrorHandler(handler func(context.Context, string, error)) Option {
	return func(d *Dispatcher) {
		d.errorHandler = handler
	}
}

// WithLogger sets the logger for the dispatcher.
// If not set, slog.Default() is used.
//
// Example:
//
//	dispatcher := command.NewDispatcher(command.WithLogger(logger))
func WithLogger(logger *slog.Logger) Option {
	return func(d *Dispatcher) {
		d.logger = logger
	}
}

// WithMiddleware sets middleware for the dispatcher.
// Middleware is applied to all handlers in the order provided.
// Middleware must be configured at construction time and cannot be changed later.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithMiddleware(
//	        command.LoggingMiddleware(logger),
//	        metricsMiddleware,
//	    ),
//	)
func WithMiddleware(middleware ...Middleware) Option {
	return func(d *Dispatcher) {
		d.middleware = middleware
	}
}
