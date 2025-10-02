package event

import (
	"context"
	"log/slog"
	"time"
)

// ProcessorOption configures a Processor.
type ProcessorOption func(*Processor)

// WithHandler registers one or more handlers with the processor.
// Multiple handlers can be registered for the same event type.
//
// Example:
//
//	processor := event.NewProcessor(
//	    event.WithHandler(handler1),
//	    event.WithHandler(handler2, handler3),
//	)
func WithHandler(handlers ...Handler) ProcessorOption {
	return func(p *Processor) {
		for _, h := range handlers {
			eventName := h.EventName()
			p.handlers[eventName] = append(p.handlers[eventName], h)
		}
	}
}

// WithEventSource sets the event source for the processor to pull events from.
//
// Example:
//
//	processor := event.NewProcessor(
//	    event.WithEventSource(bus),
//	    event.WithHandler(handler1),
//	)
func WithEventSource(source eventSource) ProcessorOption {
	return func(p *Processor) {
		if source != nil {
			p.eventBus = source
		}
	}
}

// WithShutdownTimeout configures maximum wait time for active handlers during shutdown.
// Processor will wait this long for running handlers to complete before forcing shutdown.
func WithShutdownTimeout(d time.Duration) ProcessorOption {
	return func(p *Processor) {
		if d > 0 {
			p.shutdownTimeout = d
		}
	}
}

// WithMaxConcurrentHandlers limits the number of handlers that can execute concurrently.
// This prevents unbounded goroutine spawning under high load. Set to 0 (default) for unlimited.
// The limit applies across all event types - total concurrent handlers cannot exceed this value.
//
// Example:
//
//	processor := event.NewProcessor(
//	    event.WithEventSource(bus),
//	    event.WithHandler(handler1, handler2),
//	    event.WithMaxConcurrentHandlers(100), // Max 100 concurrent handlers
//	)
func WithMaxConcurrentHandlers(max int) ProcessorOption {
	return func(p *Processor) {
		if max >= 0 {
			p.maxConcurrentHandlers = max
		}
	}
}

// WithStaleThreshold configures the duration after which a processor with no activity
// is considered stale in health checks. Default is 5 minutes.
func WithStaleThreshold(d time.Duration) ProcessorOption {
	return func(p *Processor) {
		if d > 0 {
			p.staleThreshold = d
		}
	}
}

// WithStuckThreshold configures the number of active events that triggers a stuck
// processor warning in health checks. Default is 1000.
func WithStuckThreshold(threshold int32) ProcessorOption {
	return func(p *Processor) {
		if threshold > 0 {
			p.stuckThreshold = threshold
		}
	}
}

// WithProcessorLogger configures structured logging for processor operations.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithProcessorLogger(logger *slog.Logger) ProcessorOption {
	return func(p *Processor) {
		if logger != nil {
			p.logger = logger
		}
	}
}

// WithFallbackHandler sets a fallback handler for events with no registered handlers.
// The fallback handler receives the full Event with all metadata (ID, Name, Payload, CreatedAt).
// Useful for logging unhandled events, metrics, or forwarding to a dead letter queue.
//
// Example:
//
//	processor := event.NewProcessor(
//	    event.WithEventSource(bus),
//	    event.WithFallbackHandler(func(ctx context.Context, evt Event) error {
//	        log.Warn("unhandled event", "id", evt.ID, "name", evt.Name)
//	        return nil
//	    }),
//	)
func WithFallbackHandler(fn func(context.Context, Event) error) ProcessorOption {
	return func(p *Processor) {
		if fn != nil {
			p.fallbackHandler = fn
		}
	}
}
