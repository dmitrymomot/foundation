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

// Processor manages event handlers and coordinates event processing.
type Processor struct {
	handlers        map[string][]Handler
	eventBus        eventSource
	fallbackHandler fallbackHandlerFunc
	mu              sync.RWMutex

	shutdownTimeout time.Duration
	logger          *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

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
		handlers:        make(map[string][]Handler),
		shutdownTimeout: 30 * time.Second,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Start begins processing events from the event source. This is a blocking operation
// that runs until the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (p *Processor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return ErrProcessorAlreadyStarted
	}

	if p.eventBus == nil {
		p.mu.Unlock()
		return ErrEventSourceNil
	}

	if len(p.handlers) == 0 && p.fallbackHandler == nil {
		p.mu.Unlock()
		return ErrNoHandlers
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	ctx = p.ctx
	p.mu.Unlock()

	p.logger.InfoContext(ctx, "event processor started",
		slog.Int("handler_count", len(p.handlers)))

	events := p.eventBus.Events()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("event processor stopping")
			return ctx.Err()
		case data, ok := <-events:
			if !ok {
				p.logger.Info("event source closed")
				return nil
			}

			var event Event
			if err := json.Unmarshal(data, &event); err != nil {
				p.logger.ErrorContext(ctx, "failed to unmarshal event",
					slog.String("error", err.Error()))
				continue
			}

			if err := p.processHandlers(event); err != nil {
				if !errors.Is(err, ErrNoHandlers) {
					p.logger.ErrorContext(ctx, "failed to process event",
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
	p.mu.Lock()
	if p.cancel == nil {
		p.mu.Unlock()
		return ErrProcessorNotStarted
	}

	cancel := p.cancel
	p.cancel = nil
	p.mu.Unlock()

	cancel()

	p.logger.Info("event processor stopping, waiting for active handlers to complete",
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

func (p *Processor) processHandlers(event Event) error {
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

				ctx := WithStartProcessingTime(WithEventMeta(p.ctx, event), time.Now())

				defer func() {
					if r := recover(); r != nil {
						p.eventsFailed.Add(1)
						p.logger.ErrorContext(ctx, "fallback handler panicked",
							slog.String("event_id", event.ID),
							slog.String("event_name", event.Name),
							slog.Any("panic", r))
					}
				}()

				start := time.Now()

				if err := fallback(ctx, event); err != nil {
					p.eventsFailed.Add(1)
					p.logger.ErrorContext(ctx, "fallback handler failed",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.Duration("duration", time.Since(start)),
						slog.String("error", err.Error()))
				} else {
					p.eventsProcessed.Add(1)
					p.logger.DebugContext(ctx, "fallback handler completed",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.Duration("duration", time.Since(start)))
				}

				p.lastActivityAt.Store(time.Now().Unix())
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

			ctx := WithStartProcessingTime(WithEventMeta(p.ctx, event), time.Now())

			defer func() {
				if r := recover(); r != nil {
					p.eventsFailed.Add(1)
					p.logger.ErrorContext(ctx, "event handler panicked",
						slog.String("event_id", event.ID),
						slog.String("event_name", event.Name),
						slog.String("handler", handler.EventName()),
						slog.Any("panic", r))
				}
			}()

			start := time.Now()

			if err := handler.Handle(ctx, event.Payload); err != nil {
				p.eventsFailed.Add(1)
				p.logger.ErrorContext(ctx, "event handler failed",
					slog.String("event_id", event.ID),
					slog.String("event_name", event.Name),
					slog.String("handler", handler.EventName()),
					slog.Duration("duration", time.Since(start)),
					slog.String("error", err.Error()))
			} else {
				p.eventsProcessed.Add(1)
				p.logger.DebugContext(ctx, "event handler completed",
					slog.String("event_id", event.ID),
					slog.String("event_name", event.Name),
					slog.String("handler", handler.EventName()),
					slog.Duration("duration", time.Since(start)))
			}

			p.lastActivityAt.Store(time.Now().Unix())
		}(h)
	}

	return nil
}

// Stats returns current processor statistics for observability and monitoring.
func (p *Processor) Stats() ProcessorStats {
	p.mu.RLock()
	isRunning := p.cancel != nil
	p.mu.RUnlock()

	lastActivity := p.lastActivityAt.Load()
	var lastActivityTime time.Time
	if lastActivity > 0 {
		lastActivityTime = time.Unix(lastActivity, 0)
	}

	return ProcessorStats{
		EventsProcessed: p.eventsProcessed.Load(),
		EventsFailed:    p.eventsFailed.Load(),
		ActiveEvents:    p.activeEvents.Load(),
		IsRunning:       isRunning,
		LastActivityAt:  lastActivityTime,
	}
}

// Healthcheck validates that the processor is operational.
// Returns nil if healthy, or an error describing the health issue.
func (p *Processor) Healthcheck(ctx context.Context) error {
	stats := p.Stats()

	if !stats.IsRunning {
		return errors.Join(ErrHealthcheckFailed, ErrProcessorNotRunning)
	}

	return nil
}
