package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Processor handles commands with lifecycle management.
// Inspired by Watermill's Router - it's the active component that manages workers.
// It registers handlers, applies middleware, and manages worker lifecycle.
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithWorkers(5),
//	    command.WithMiddleware(command.LoggingMiddleware(logger)),
//	    command.WithErrorHandler(errorHandler),
//	)
//	processor.Register(command.NewHandlerFunc(createUserHandler))
//	err := processor.Run(ctx)
type Processor struct {
	handlers     map[string]Handler
	middleware   []Middleware
	transport    ProcessorTransport
	errorHandler func(context.Context, string, error)
	logger       *slog.Logger
	workers      int // Number of workers for async transports
	mu           sync.RWMutex

	// Stats tracking
	received  atomic.Uint64
	processed atomic.Uint64
	failed    atomic.Uint64
}

// ProcessorOption configures a Processor.
type ProcessorOption func(*Processor)

// NewProcessor creates a new command processor with the given transport and options.
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithWorkers(5),
//	    command.WithLogger(logger),
//	    command.WithMiddleware(command.LoggingMiddleware(logger)),
//	)
func NewProcessor(transport ProcessorTransport, opts ...ProcessorOption) *Processor {
	p := &Processor{
		handlers:   make(map[string]Handler),
		middleware: []Middleware{},
		transport:  transport,
		logger:     slog.Default(),
		workers:    1, // Default to 1 worker
	}

	for _, opt := range opts {
		opt(p)
	}

	// Initialize sync transport immediately (avoid race conditions)
	if st, ok := p.transport.(*syncTransport); ok {
		st.SetGetHandler(p.getHandler)
	}

	return p
}

// Register registers a handler for a command type.
// Panics if a handler is already registered for the command.
//
// Example:
//
//	handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
//	    return db.Insert(ctx, cmd.Email, cmd.Name)
//	})
//	processor.Register(handler)
func (p *Processor) Register(handler Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmdName := handler.Name()
	if _, exists := p.handlers[cmdName]; exists {
		panic(fmt.Sprintf("command: %s: %s", ErrDuplicateHandler, cmdName))
	}

	p.handlers[cmdName] = handler
}

// Run starts the processor and blocks until the context is cancelled.
// For async transports, this starts workers that consume from the transport's channel.
// For sync transports, this just blocks (commands execute immediately on dispatch).
// This method should be called in a goroutine, typically via errgroup.
//
// Example:
//
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(func() error {
//	    return processor.Run(ctx)
//	})
func (p *Processor) Run(ctx context.Context) error {
	// Get command channel from transport (Watermill Subscribe pattern)
	ch, err := p.transport.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to transport: %w", err)
	}

	// Handle sync transport special case (ch == nil)
	if ch == nil {
		p.logger.InfoContext(ctx, "processor started (sync mode)")
		// Sync transport - just block until context cancelled
		<-ctx.Done()
		p.logger.InfoContext(ctx, "processor shutdown complete")
		return nil
	}

	// Async transport - start workers (Watermill Router pattern)
	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go p.worker(ctx, ch, &wg)
	}

	p.logger.InfoContext(ctx, "processor started",
		slog.Int("workers", p.workers))

	// Block until context is cancelled
	<-ctx.Done()

	// Close transport (closes channel)
	p.logger.InfoContext(ctx, "processor shutting down, draining commands")
	if err := p.transport.Close(); err != nil {
		p.logger.ErrorContext(ctx, "error closing transport", slog.Any("error", err))
	}

	// Wait for all workers to finish processing
	wg.Wait()
	p.logger.InfoContext(ctx, "processor shutdown complete")

	return nil
}

// worker processes commands from the channel until it's closed.
// This is the Processor's internal worker goroutine.
func (p *Processor) worker(ctx context.Context, ch <-chan envelope, wg *sync.WaitGroup) {
	defer wg.Done()

	for env := range ch {
		p.handleCommand(env)
	}
}

// handleCommand processes a single command from the channel.
// Panics are caught and converted to errors by safeHandle.
func (p *Processor) handleCommand(env envelope) {
	handler, exists := p.getHandler(env.Name)
	if !exists {
		err := fmt.Errorf("%w: %s", ErrHandlerNotFound, env.Name)
		p.logger.Error("handler not found",
			slog.String("command", env.Name))

		p.failed.Add(1)

		if p.errorHandler != nil {
			p.errorHandler(env.Context, env.Name, err)
		}
		return
	}

	p.received.Add(1)

	if err := safeHandle(handler, env.Context, env.Payload); err != nil {
		p.failed.Add(1)
		if p.errorHandler != nil {
			p.errorHandler(env.Context, env.Name, err)
		}
	} else {
		p.processed.Add(1)
	}
}

// Dispatch sends a command for execution via the transport.
// This is a convenience method for sync transports where the processor
// can dispatch directly. For async transports, use a separate Dispatcher.
//
// Example (sync transport):
//
//	err := processor.Dispatch(ctx, CreateUser{
//	    Email: "user@example.com",
//	    Name:  "John Doe",
//	})
func (p *Processor) Dispatch(ctx context.Context, cmd any) error {
	// Check if transport supports direct dispatch (sync transport)
	dt, ok := p.transport.(DispatcherTransport)
	if !ok {
		return errors.New("processor transport does not support direct dispatch; use a Dispatcher instead")
	}

	cmdName := getCommandNameFromInstance(cmd)
	p.received.Add(1)

	err := dt.Dispatch(ctx, cmdName, cmd)
	if err != nil {
		p.failed.Add(1)
	} else {
		p.processed.Add(1)
	}

	return err
}

// Stats returns current processing statistics.
// Only meaningful for async transports where stats are tracked.
type Stats struct {
	Received  uint64 // Commands received
	Processed uint64 // Commands successfully processed
	Failed    uint64 // Commands that failed
}

// Stats returns the current processing statistics.
func (p *Processor) Stats() Stats {
	return Stats{
		Received:  p.received.Load(),
		Processed: p.processed.Load(),
		Failed:    p.failed.Load(),
	}
}

// getHandler retrieves a handler by command name with middleware applied.
// This is used by transports to look up and execute handlers.
func (p *Processor) getHandler(cmdName string) (Handler, bool) {
	p.mu.RLock()
	handler, exists := p.handlers[cmdName]
	middleware := p.middleware
	p.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if len(middleware) > 0 {
		handler = chainMiddleware(handler, middleware)
	}

	return handler, true
}

// WithWorkers sets the number of worker goroutines for async transports.
// Default is 1. More workers increase concurrency.
// This option is ignored for sync transports.
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithWorkers(5),
//	)
func WithWorkers(n int) ProcessorOption {
	return func(p *Processor) {
		if n > 0 {
			p.workers = n
		}
	}
}

// WithErrorHandler sets a callback for handling errors from async transports.
// This is only used by async transports where errors cannot be
// returned to the caller.
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	        logger.Error("command failed", "command", cmdName, "error", err)
//	    }),
//	)
func WithErrorHandler(handler func(context.Context, string, error)) ProcessorOption {
	return func(p *Processor) {
		p.errorHandler = handler
	}
}

// WithLogger sets the logger for the processor.
// If not set, slog.Default() is used.
//
// Example:
//
//	processor := command.NewProcessor(transport, command.WithLogger(logger))
func WithLogger(logger *slog.Logger) ProcessorOption {
	return func(p *Processor) {
		p.logger = logger
	}
}

// WithMiddleware sets middleware for the processor.
// Middleware is applied to all handlers in the order provided.
// Middleware must be configured at construction time and cannot be changed later.
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithMiddleware(
//	        command.LoggingMiddleware(logger),
//	        metricsMiddleware,
//	    ),
//	)
func WithMiddleware(middleware ...Middleware) ProcessorOption {
	return func(p *Processor) {
		p.middleware = middleware
	}
}
