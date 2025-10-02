package event

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Processor handles events with lifecycle management.
// Inspired by Watermill's Router - it's the active component that manages workers.
// It registers handlers, applies middleware, and manages worker lifecycle.
//
// Example:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithWorkers(5),
//	    event.WithMiddleware(event.LoggingMiddleware(logger)),
//	    event.WithErrorHandler(errorHandler),
//	)
//	processor.Register(event.NewHandlerFunc(invalidateCacheHandler))
//	err := processor.Start(ctx)
type Processor struct {
	handlers     map[string][]Handler // event name -> handlers (one-to-many)
	middleware   []Middleware
	transport    ProcessorTransport
	errorHandler func(context.Context, string, error)
	logger       *slog.Logger
	workers      int // Number of workers for async transports
	strictMode   bool
	running      atomic.Bool // Prevents handler registration after Start() starts
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

// NewProcessor creates a new event processor with the given transport and options.
//
// Example:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithWorkers(5),
//	    event.WithLogger(logger),
//	    event.WithMiddleware(event.LoggingMiddleware(logger)),
//	)
func NewProcessor(transport ProcessorTransport, opts ...ProcessorOption) *Processor {
	p := &Processor{
		handlers:        make(map[string][]Handler),
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
		st.SetGetHandlers(p.getHandlers)
	}

	return p
}

// Register registers a handler for an event type.
// Unlike commands, multiple handlers can be registered for the same event (fan-out).
// IMPORTANT: All handlers must be registered BEFORE calling Start().
// Attempting to register handlers after Start() starts will panic.
// Middleware is applied during registration for better performance.
//
// Example:
//
//	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//	    return cache.Invalidate(ctx, evt.UserID)
//	})
//	processor.Register(handler)
func (p *Processor) Register(handler Handler) {
	if p.running.Load() {
		panic("event: cannot register handlers after processor has started")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Apply middleware during registration (not per-call)
	if len(p.middleware) > 0 {
		handler = chainMiddleware(handler, p.middleware)
	}

	eventName := handler.Name()
	p.handlers[eventName] = append(p.handlers[eventName], handler)
}

// Start starts the processor and blocks until the context is cancelled or Stop is called.
// For async transports, this starts workers that consume from the transport's channel.
// For sync transports, this just blocks (events execute immediately on dispatch).
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

	// Mark processor as running to prevent new handler registrations
	p.running.Store(true)
	defer p.running.Store(false)

	// Get event channel from transport (Watermill Subscribe pattern)
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
	p.logger.InfoContext(p.ctx, "processor shutting down, draining events")
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
		p.logger.Warn("processor shutdown timeout exceeded - some events may be abandoned",
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

// worker processes events from the channel until it's closed.
// This is the Processor's internal worker goroutine.
func (p *Processor) worker(ctx context.Context, ch <-chan envelope, wg *sync.WaitGroup) {
	defer wg.Done()

	for env := range ch {
		p.handleEvent(env)
	}
}

// handleEvent processes a single event from the channel.
// Executes all registered handlers for the event in FIFO order.
// Panics are caught and converted to errors by safeHandle.
func (p *Processor) handleEvent(env envelope) {
	// Track that we received this event (before zero-handler check)
	p.received.Add(1)

	handlers := p.getHandlers(env.Name)

	// Check for zero handlers
	if len(handlers) == 0 {
		if p.strictMode {
			p.logger.Error("no handlers registered for event (strict mode)",
				slog.String("event", env.Name))
			p.failed.Add(1)
			if p.errorHandler != nil {
				p.errorHandler(env.Context, env.Name, fmt.Errorf("%w: %s", ErrNoHandlers, env.Name))
			}
		} else {
			p.logger.Warn("no handlers registered for event",
				slog.String("event", env.Name))
		}
		return
	}

	// Execute all handlers (fan-out pattern)
	var hadError bool
	for _, handler := range handlers {
		if err := safeHandle(handler, env.Context, env.Payload); err != nil {
			hadError = true
			p.logger.Error("event handler failed",
				slog.String("event", env.Name),
				slog.String("handler", handler.Name()),
				slog.Any("error", err))

			if p.errorHandler != nil {
				p.errorHandler(env.Context, env.Name, fmt.Errorf("handler %s failed: %w", handler.Name(), err))
			}
		}
	}

	if hadError {
		p.failed.Add(1)
	} else {
		p.processed.Add(1)
	}
}

// Publish sends an event for execution via the transport.
// This is a convenience method for sync transports where the processor
// can publish directly. For async transports, use a separate Publisher.
//
// Example (sync transport):
//
//	err := processor.Publish(ctx, UserCreated{
//	    Email: "user@example.com",
//	    Name:  "John Doe",
//	})
func (p *Processor) Publish(ctx context.Context, event any) error {
	// Check if transport supports direct dispatch (sync transport)
	pt, ok := p.transport.(PublisherTransport)
	if !ok {
		return errors.New("processor transport does not support direct publish; use a Publisher instead")
	}

	eventName := getEventNameFromInstance(event)
	// Note: Stats are tracked by the transport's handler execution, not here
	return pt.Dispatch(ctx, eventName, event)
}

// Stats returns current processing statistics.
// Only meaningful for async transports where stats are tracked.
type Stats struct {
	Received  uint64 // Events received
	Processed uint64 // Events successfully processed
	Failed    uint64 // Events that failed
}

// Stats returns the current processing statistics.
func (p *Processor) Stats() Stats {
	return Stats{
		Received:  p.received.Load(),
		Processed: p.processed.Load(),
		Failed:    p.failed.Load(),
	}
}

// getHandlers retrieves all handlers for an event name.
// This is used by transports to look up and execute handlers.
// Returns handlers in FIFO registration order with middleware already applied.
// Middleware is applied during registration, not during lookup, for better performance.
func (p *Processor) getHandlers(eventName string) []Handler {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.handlers[eventName]
}

// WithWorkers sets the number of worker goroutines for async transports.
// Default is 1. More workers increase concurrency.
// This option is ignored for sync transports.
//
// Example:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithWorkers(5),
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
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
//	        logger.Error("event handler failed", "event", evtName, "error", err)
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
//	processor := event.NewProcessor(transport, event.WithLogger(logger))
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
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithShutdownTimeout(60 * time.Second),
//	)
func WithShutdownTimeout(timeout time.Duration) ProcessorOption {
	return func(p *Processor) {
		if timeout > 0 {
			p.shutdownTimeout = timeout
		}
	}
}

// WithStrictHandlers enables strict mode where zero handlers for an event is an error.
// By default, zero handlers is just a warning.
//
// Example:
//
//	processor := event.NewProcessor(transport, event.WithStrictHandlers(true))
func WithStrictHandlers(strict bool) ProcessorOption {
	return func(p *Processor) {
		p.strictMode = strict
	}
}

// WithMiddleware sets middleware for the processor.
// Middleware is applied to all handlers in the order provided.
// Middleware must be configured at construction time and cannot be changed later.
//
// Example:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithMiddleware(
//	        event.LoggingMiddleware(logger),
//	        metricsMiddleware,
//	    ),
//	)
func WithMiddleware(middleware ...Middleware) ProcessorOption {
	return func(p *Processor) {
		p.middleware = middleware
	}
}
