package command

import (
	"context"
	"log/slog"
	"time"
)

// Middleware wraps a Handler to add cross-cutting functionality.
// Middleware can be used for logging, metrics, tracing, validation, etc.
type Middleware func(next Handler) Handler

// middlewareHandler wraps a Handler with middleware functionality.
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

// LoggingMiddleware returns a middleware that logs command execution.
// It logs the command name, execution duration, and any errors.
//
// Example:
//
//	dispatcher := command.NewDispatcher()
//	dispatcher.Use(command.LoggingMiddleware(logger))
func LoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return &middlewareHandler{
			name: next.Name(),
			next: next,
			fn: func(ctx context.Context, payload any) error {
				start := time.Now()
				cmdName := next.Name()

				logger.InfoContext(ctx, "command started",
					slog.String("command", cmdName))

				err := next.Handle(ctx, payload)
				duration := time.Since(start)

				if err != nil {
					logger.ErrorContext(ctx, "command failed",
						slog.String("command", cmdName),
						slog.Duration("duration", duration),
						slog.String("error", err.Error()))
					return err
				}

				logger.InfoContext(ctx, "command completed",
					slog.String("command", cmdName),
					slog.Duration("duration", duration))

				return nil
			},
		}
	}
}

// chainMiddleware applies multiple middleware in order.
// The first middleware in the slice is the outermost (executed first).
func chainMiddleware(handler Handler, middleware []Middleware) Handler {
	// Apply middleware in reverse order so the first middleware
	// in the slice becomes the outermost wrapper
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}
