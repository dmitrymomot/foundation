package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful processor shutdown.
	DefaultShutdownTimeout = 30 * time.Second

	// DefaultStaleThreshold is the default time after which a processor is considered stale.
	DefaultStaleThreshold = 5 * time.Minute

	// DefaultStuckThreshold is the default number of active events that indicates a stuck processor.
	DefaultStuckThreshold = 1000
)

// Processor manages event handlers and coordinates event processing.
type Processor struct {
	handlers        map[string][]Handler
	eventBus        eventSource
	fallbackHandler func(context.Context, Event) error
	mu              sync.RWMutex

	shutdownTimeout       time.Duration
	maxConcurrentHandlers int
	handlerSemaphore      chan struct{}
	staleThreshold        time.Duration
	stuckThreshold        int32
	logger                *slog.Logger

	running    atomic.Bool
	cancelFunc atomic.Pointer[context.CancelFunc]
	done       chan struct{}
	wg         sync.WaitGroup

	eventsProcessed atomic.Int64
	eventsFailed    atomic.Int64
	activeEvents    atomic.Int32
	lastActivityAt  atomic.Int64
}

type eventSource interface {
	Events() <-chan []byte
}

// ProcessorStats provides observability metrics for monitoring and debugging.
type ProcessorStats struct {
	EventsProcessed int64
	EventsFailed    int64
	ActiveEvents    int32
	IsRunning       bool
	LastActivityAt  time.Time
}

// NewProcessor creates a new event processor with the given options.
//
// Example:
//
//	processor := event.NewProcessor(
//	    event.WithEventSource(bus),
//	    event.WithHandler(handler1, handler2),
//	)
func NewProcessor(opts ...ProcessorOption) *Processor {
	p := &Processor{
		handlers:              make(map[string][]Handler),
		shutdownTimeout:       DefaultShutdownTimeout,
		maxConcurrentHandlers: 0, // 0 means unlimited
		staleThreshold:        DefaultStaleThreshold,
		stuckThreshold:        DefaultStuckThreshold,
		logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(p)
	}

	// Initialize semaphore if max concurrent handlers is set
	if p.maxConcurrentHandlers > 0 {
		p.handlerSemaphore = make(chan struct{}, p.maxConcurrentHandlers)
	}

	return p
}

// Start begins processing events from the event source. This is a blocking operation
// that runs until the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (p *Processor) Start(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		return ErrProcessorAlreadyStarted
	}
	defer p.running.Store(false)

	p.mu.RLock()
	hasHandlers := len(p.handlers) > 0 || p.fallbackHandler != nil
	p.mu.RUnlock()

	if p.eventBus == nil {
		return ErrEventSourceNil
	}

	if !hasHandlers {
		return ErrNoHandlers
	}

	procCtx, cancel := context.WithCancel(ctx)
	p.cancelFunc.Store(&cancel)
	p.done = make(chan struct{})

	defer close(p.done)

	p.logger.InfoContext(procCtx, "event processor started",
		slog.Int("handler_count", len(p.handlers)))

	events := p.eventBus.Events()

	for {
		select {
		case <-procCtx.Done():
			p.logger.Info("event processor stopping")
			return procCtx.Err()
		case data, ok := <-events:
			if !ok {
				p.logger.Info("event source closed")
				return nil
			}

			var event Event
			if err := json.Unmarshal(data, &event); err != nil {
				p.logger.ErrorContext(procCtx, "failed to unmarshal event",
					slog.String("error", err.Error()))
				continue
			}

			if err := p.processHandlers(procCtx, event); err != nil {
				if !errors.Is(err, ErrNoHandlers) {
					p.logger.ErrorContext(procCtx, "failed to process event",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.String("error", err.Error()))
				}
			}
		}
	}
}

// Stop gracefully shuts down the processor with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (p *Processor) Stop() error {
	if !p.running.Load() {
		return ErrProcessorNotStarted
	}

	if cancel := p.cancelFunc.Load(); cancel != nil {
		(*cancel)()
	}

	p.logger.Info("event processor stopping, waiting for active handlers to complete",
		slog.Duration("timeout", p.shutdownTimeout))

	<-p.done

	ctx, ctxCancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
	defer ctxCancel()

	waitDone := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		p.logger.Info("event processor stopped cleanly")
		return nil
	case <-ctx.Done():
		p.logger.Warn("event processor shutdown timeout exceeded - some handlers may be abandoned",
			slog.Duration("timeout", p.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", p.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the processor, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (p *Processor) Run(ctx context.Context) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- p.Start(ctx)
		}()

		select {
		case <-ctx.Done():
			// Context cancelled - perform graceful shutdown
			if stopErr := p.Stop(); stopErr != nil {
				p.logger.Error("graceful shutdown failed", slog.String("error", stopErr.Error()))
			}
			<-errCh // Wait for Start() to exit
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

// acquireSemaphore blocks until a handler slot is available (if limiting is enabled)
func (p *Processor) acquireSemaphore(ctx context.Context) bool {
	if p.handlerSemaphore == nil {
		return true // No limiting enabled
	}
	select {
	case p.handlerSemaphore <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// releaseSemaphore releases a handler slot (if limiting is enabled)
func (p *Processor) releaseSemaphore() {
	if p.handlerSemaphore != nil {
		<-p.handlerSemaphore
	}
}

func (p *Processor) processHandlers(ctx context.Context, event Event) error {
	p.mu.RLock()
	handlers, exists := p.handlers[event.Name]
	fallback := p.fallbackHandler
	p.mu.RUnlock()

	if !exists || len(handlers) == 0 {
		if fallback != nil {
			p.wg.Add(1)
			p.activeEvents.Add(1)

			go func() {
				defer p.wg.Done()
				defer p.activeEvents.Add(-1)

				handlerCtx := WithStartProcessingTime(WithEventMeta(ctx, event), time.Now())

				// Acquire semaphore slot if limiting is enabled
				if !p.acquireSemaphore(handlerCtx) {
					return // Context cancelled while waiting
				}
				defer p.releaseSemaphore()

				// Check if processor is shutting down before starting work
				select {
				case <-handlerCtx.Done():
					return
				default:
				}

				defer func() {
					if r := recover(); r != nil {
						p.eventsFailed.Add(1)
						p.logger.ErrorContext(handlerCtx, "fallback handler panicked",
							slog.String("event_id", event.ID),
							slog.String("event_name", event.Name),
							slog.Any("panic", r))
					}
				}()

				start := time.Now()

				if err := fallback(handlerCtx, event); err != nil {
					p.eventsFailed.Add(1)
					p.logger.ErrorContext(handlerCtx, "fallback handler failed",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.Duration("duration", time.Since(start)),
						slog.String("error", err.Error()))
				} else {
					p.eventsProcessed.Add(1)
					p.logger.DebugContext(handlerCtx, "fallback handler completed",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.Duration("duration", time.Since(start)))
				}

				p.lastActivityAt.Store(time.Now().UnixNano())
			}()

			return nil
		}
		return ErrNoHandlers
	}

	for _, h := range handlers {
		p.wg.Add(1)
		p.activeEvents.Add(1)

		go func(handler Handler) {
			defer p.wg.Done()
			defer p.activeEvents.Add(-1)

			handlerCtx := WithStartProcessingTime(WithEventMeta(ctx, event), time.Now())

			// Acquire semaphore slot if limiting is enabled
			if !p.acquireSemaphore(handlerCtx) {
				return // Context cancelled while waiting
			}
			defer p.releaseSemaphore()

			// Check if processor is shutting down before starting work
			select {
			case <-handlerCtx.Done():
				return
			default:
			}

			defer func() {
				if r := recover(); r != nil {
					p.eventsFailed.Add(1)
					p.logger.ErrorContext(handlerCtx, "event handler panicked",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.String("handler", handler.EventName()),
						slog.Any("panic", r))
				}
			}()

			start := time.Now()

			if err := handler.Handle(handlerCtx, event.Payload); err != nil {
				p.eventsFailed.Add(1)
				p.logger.ErrorContext(handlerCtx, "event handler failed",
					slog.String("event_id", event.ID),
					slog.String("event_name", event.Name),
					slog.String("handler", handler.EventName()),
					slog.Duration("duration", time.Since(start)),
					slog.String("error", err.Error()))
			} else {
				p.eventsProcessed.Add(1)
				p.logger.DebugContext(handlerCtx, "event handler completed",
					slog.String("event_id", event.ID),
					slog.String("event_name", event.Name),
					slog.String("handler", handler.EventName()),
					slog.Duration("duration", time.Since(start)))
			}

			p.lastActivityAt.Store(time.Now().UnixNano())
		}(h)
	}

	return nil
}

// Stats returns current processor statistics for observability and monitoring.
func (p *Processor) Stats() ProcessorStats {
	lastActivity := p.lastActivityAt.Load()
	var lastActivityTime time.Time
	if lastActivity > 0 {
		lastActivityTime = time.Unix(0, lastActivity)
	}

	return ProcessorStats{
		EventsProcessed: p.eventsProcessed.Load(),
		EventsFailed:    p.eventsFailed.Load(),
		ActiveEvents:    p.activeEvents.Load(),
		IsRunning:       p.running.Load(),
		LastActivityAt:  lastActivityTime,
	}
}

// Healthcheck validates that the processor is operational.
// Returns nil if healthy, or an error describing the health issue.
// Checks for:
// - Processor running status
// - Stale processor (no recent activity)
// - Stuck processor (too many active events)
func (p *Processor) Healthcheck(ctx context.Context) error {
	stats := p.Stats()

	if !stats.IsRunning {
		return errors.Join(ErrHealthcheckFailed, ErrProcessorNotRunning)
	}

	var healthErrors []error

	// Check if processor is stale (no recent activity)
	if !stats.LastActivityAt.IsZero() {
		timeSinceActivity := time.Since(stats.LastActivityAt)
		if timeSinceActivity > p.staleThreshold {
			healthErrors = append(healthErrors, fmt.Errorf("%w: last activity %s ago (threshold: %s)",
				ErrProcessorStale, timeSinceActivity.Round(time.Second), p.staleThreshold))
		}
	}

	// Check if processor is stuck (too many active events)
	if stats.ActiveEvents > p.stuckThreshold {
		healthErrors = append(healthErrors, fmt.Errorf("%w: %d active events (threshold: %d)",
			ErrProcessorStuck, stats.ActiveEvents, p.stuckThreshold))
	}

	if len(healthErrors) > 0 {
		return errors.Join(append([]error{ErrHealthcheckFailed}, healthErrors...)...)
	}

	return nil
}
