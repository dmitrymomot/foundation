package event

import (
	"context"

	"github.com/dmitrymomot/foundation/pkg/async"
)

// Processor holds registered event handlers grouped by event name.
type Processor struct {
	handlers map[string][]Handler
}

// ProcessorOption configures a Processor.
type ProcessorOption func(*Processor)

// NewProcessor creates a new event processor with the given options.
//
// Example:
//
//	processor := event.NewProcessor(
//	    event.WithHandler(handler1, handler2),
//	)
func NewProcessor(opts ...ProcessorOption) *Processor {
	p := &Processor{
		handlers: make(map[string][]Handler),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// WithHandler registers one or more handlers with the processor.
// Multiple handlers can be registered for the same event type.
// Handlers are automatically wrapped with all registered middlewares.
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

// processHandlers processes all registered handlers for the given event name and payload.
// It executes all handlers concurrently using the async package and waits for all to complete.
// Returns an error if any handler fails.
func (p *Processor) processHandlers(ctx context.Context, eventName string, payload any) error {
	handlers, exists := p.handlers[eventName]
	if !exists || len(handlers) == 0 {
		// No handlers registered for this event
		return nil
	}

	// Create futures for each handler
	futures := make([]*async.ExecFuture, 0, len(handlers))
	for _, h := range handlers {
		futures = append(futures, async.Exec(ctx, payload, h.Handle))
	}

	// Wait for all handlers to complete
	return async.ExecAll(futures...)
}
