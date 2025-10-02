package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
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
//	err := processor.Start(ctx)
type Processor struct {
	handlers     map[string]Handler
	middleware   []Middleware
	transport    ProcessorTransport
	errorHandler func(context.Context, string, error)
	logger       *slog.Logger
	workers      int // Number of workers for async transports
	mu           sync.RWMutex

	// Lifecycle management
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	shutdownTimeout time.Duration

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
		handlers:        make(map[string]Handler),
		middleware:      []Middleware{},
		transport:       transport,
		logger:          slog.Default(),
		workers:         1,                // Default to 1 worker
		shutdownTimeout: 30 * time.Second, // Default shutdown timeout
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

// Start starts the processor and blocks until the context is cancelled or Stop is called.
// For async transports, this starts workers that consume from the transport's channel.
// For sync transports, this just blocks (commands execute immediately on dispatch).
// Returns ErrProcessorAlreadyStarted if the processor is already running.
// Use Stop() for graceful shutdown.
//
// Example:
//
//	if err := processor.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
func (p *Processor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return ErrProcessorAlreadyStarted
	}

	// Create internal context for lifecycle management
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	// Get command channel from transport (Watermill Subscribe pattern)
	ch, err := p.transport.Subscribe(p.ctx)
	if err != nil {
		p.mu.Lock()
		p.cancel = nil
		p.mu.Unlock()
		return fmt.Errorf("failed to subscribe to transport: %w", err)
	}

	// Handle sync transport special case (ch == nil)
	if ch == nil {
		p.logger.InfoContext(p.ctx, "processor started (sync mode)")
		// Sync transport - just block until context cancelled
		<-p.ctx.Done()
		p.logger.InfoContext(p.ctx, "processor shutdown complete")

		p.mu.Lock()
		p.cancel = nil
		p.mu.Unlock()
		return p.ctx.Err()
	}

	// Async transport - start workers (Watermill Router pattern)
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(p.ctx, ch, &p.wg)
	}

	p.logger.InfoContext(p.ctx, "processor started",
		slog.Int("workers", p.workers))

	// Block until context is cancelled
	<-p.ctx.Done()

	// Close transport (closes channel)
	p.logger.InfoContext(p.ctx, "processor shutting down, draining commands")
	if err := p.transport.Close(); err != nil {
		p.logger.ErrorContext(p.ctx, "error closing transport", slog.Any("error", err))
	}

	// Wait for all workers to finish processing
	p.wg.Wait()
	p.logger.InfoContext(p.ctx, "processor shutdown complete")

	p.mu.Lock()
	p.cancel = nil
	p.mu.Unlock()

	return p.ctx.Err()
}

// Stop gracefully shuts down the processor with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (p *Processor) Stop() error {
	p.mu.Lock()
	if p.cancel == nil {
		p.mu.Unlock()
		return ErrProcessorNotStarted
	}

	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()

	cancel()

	p.logger.Info("processor stopping, waiting for active workers to complete",
		slog.Duration("timeout", p.shutdownTimeout))

	ctx, ctxCancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
	defer ctxCancel()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("processor stopped cleanly")
		return nil
	case <-ctx.Done():
		p.logger.Warn("processor shutdown timeout exceeded - some commands may be abandoned",
			slog.Duration("timeout", p.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", p.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the processor, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
//
// Example:
//
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(processor.Run(ctx))
func (p *Processor) Run(ctx context.Context) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- p.Start(ctx)
		}()

		select {
		case <-ctx.Done():
			// Context cancelled - perform graceful shutdown
			_ = p.Stop() // Ignore stop error in normal shutdown
			<-errCh      // Wait for Start() to exit
			return nil
		case err := <-errCh:
			// Start() returned - check if it's a normal shutdown
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
	}
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

// WithShutdownTimeout sets the graceful shutdown timeout for the processor.
// Default is 30 seconds. If workers don't finish within this timeout,
// Stop() returns an error but the processor still stops.
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithShutdownTimeout(60 * time.Second),
//	)
func WithShutdownTimeout(timeout time.Duration) ProcessorOption {
	return func(p *Processor) {
		if timeout > 0 {
			p.shutdownTimeout = timeout
		}
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
