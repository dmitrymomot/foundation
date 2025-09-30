package event

import (
	"context"
	"log/slog"
	"time"
)

// Middleware wraps a Handler to add additional functionality.
// Multiple middleware can be composed using the chain function.
type Middleware func(Handler) Handler

// middlewareHandler wraps a Handler with additional functionality.
type middlewareHandler struct {
	name string
	next Handler
	fn   func(ctx context.Context, payload any) error
}

func (h *middlewareHandler) Name() string {
	return h.name
}

func (h *middlewareHandler) Handle(ctx context.Context, payload any) error {
	return h.fn(ctx, payload)
}

// chainMiddleware applies multiple middleware to a handler in order.
// Middleware are applied left-to-right (first middleware wraps innermost).
func chainMiddleware(handler Handler, middleware []Middleware) Handler {
	// Apply middleware in order
	for _, mw := range middleware {
		handler = mw(handler)
	}
	return handler
}

// LoggingMiddleware logs event handler execution with timing.
// Logs start, completion, and errors for all event handlers.
//
// Example:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithMiddleware(event.LoggingMiddleware(logger)),
//	)
func LoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return &middlewareHandler{
			name: next.Name(),
			next: next,
			fn: func(ctx context.Context, payload any) error {
				start := time.Now()
				logger.InfoContext(ctx, "event started",
					slog.String("event", next.Name()))

				err := next.Handle(ctx, payload)
				duration := time.Since(start)

				if err != nil {
					logger.ErrorContext(ctx, "event failed",
						slog.String("event", next.Name()),
						slog.Duration("duration", duration),
						slog.Any("error", err))
				} else {
					logger.InfoContext(ctx, "event completed",
						slog.String("event", next.Name()),
						slog.Duration("duration", duration))
				}

				return err
			},
		}
	}
}
