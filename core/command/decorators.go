package command

import (
	"context"
	"fmt"
	"time"
)

// decoratorHandler wraps a Handler with additional functionality.
type decoratorHandler struct {
	name string
	next Handler
	fn   func(ctx context.Context, payload any) error
}

func (h *decoratorHandler) Name() string {
	return h.name
}

func (h *decoratorHandler) Handle(ctx context.Context, payload any) error {
	return h.fn(ctx, payload)
}

// WithRetry wraps a handler to retry on errors up to maxRetries times.
// Returns the last error if all retries fail.
//
// Example:
//
//	handler := command.WithRetry(
//	    command.NewHandlerFunc(createUserHandler),
//	    3, // max retries
//	)
//	dispatcher.Register(handler)
func WithRetry(handler Handler, maxRetries int) Handler {
	return &decoratorHandler{
		name: handler.Name(),
		next: handler,
		fn: func(ctx context.Context, payload any) error {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					if ctx.Err() != nil {
						return ctx.Err()
					}
				}

				err := handler.Handle(ctx, payload)
				if err == nil {
					return nil
				}

				lastErr = err
			}

			return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
		},
	}
}

// WithBackoff wraps a handler with exponential backoff retry logic.
// Waits between retries with exponentially increasing delays.
//
// Parameters:
// - handler: The handler to wrap
// - maxRetries: Maximum number of retry attempts
// - initialDelay: Starting delay duration
// - maxDelay: Maximum delay duration (caps exponential growth)
//
// Example:
//
//	handler := command.WithBackoff(
//	    command.NewHandlerFunc(sendEmailHandler),
//	    5,                    // max retries
//	    100*time.Millisecond, // initial delay
//	    10*time.Second,       // max delay
//	)
//	dispatcher.Register(handler)
func WithBackoff(handler Handler, maxRetries int, initialDelay, maxDelay time.Duration) Handler {
	return &decoratorHandler{
		name: handler.Name(),
		next: handler,
		fn: func(ctx context.Context, payload any) error {
			var lastErr error
			delay := initialDelay

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
					}

					// Cap exponential growth to prevent unbounded waits
					delay *= 2
					if delay > maxDelay {
						delay = maxDelay
					}
				}

				err := handler.Handle(ctx, payload)
				if err == nil {
					return nil
				}

				lastErr = err
			}

			return fmt.Errorf("failed after %d retries with backoff: %w", maxRetries, lastErr)
		},
	}
}

// WithTimeout wraps a handler to enforce a maximum execution time.
// Cancels the handler's context if it exceeds the timeout.
//
// Example:
//
//	handler := command.WithTimeout(
//	    command.NewHandlerFunc(processImageHandler),
//	    30*time.Second,
//	)
//	dispatcher.Register(handler)
func WithTimeout(handler Handler, timeout time.Duration) Handler {
	return &decoratorHandler{
		name: handler.Name(),
		next: handler,
		fn: func(ctx context.Context, payload any) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- handler.Handle(ctx, payload)
			}()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				return fmt.Errorf("handler timeout after %s: %w", timeout, ctx.Err())
			}
		},
	}
}
