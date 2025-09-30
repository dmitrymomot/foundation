package event

import (
	"context"
	"fmt"
	"time"
)

// Decorator wraps a Handler to add additional functionality.
// Multiple decorators can be composed using the Decorate helper.
type Decorator func(Handler) Handler

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
//	handler := event.WithRetry(
//	    event.NewHandlerFunc(notifyWebhookHandler),
//	    3, // max retries
//	)
//	processor.Register(handler)
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
//	handler := event.WithBackoff(
//	    event.NewHandlerFunc(notifyWebhookHandler),
//	    5,                    // max retries
//	    100*time.Millisecond, // initial delay
//	    10*time.Second,       // max delay
//	)
//	processor.Register(handler)
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
//	handler := event.WithTimeout(
//	    event.NewHandlerFunc(processImageHandler),
//	    30*time.Second,
//	)
//	processor.Register(handler)
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

// Retry returns a Decorator that wraps a handler with retry logic.
// This is a factory function for use with the Decorate helper.
//
// Example:
//
//	handler := event.Decorate(
//	    event.NewHandlerFunc(notifyWebhookHandler),
//	    event.Retry(3),
//	)
func Retry(maxRetries int) Decorator {
	return func(h Handler) Handler {
		return WithRetry(h, maxRetries)
	}
}

// Backoff returns a Decorator that wraps a handler with exponential backoff retry logic.
// This is a factory function for use with the Decorate helper.
//
// Example:
//
//	handler := event.Decorate(
//	    event.NewHandlerFunc(notifyWebhookHandler),
//	    event.Backoff(5, 100*time.Millisecond, 10*time.Second),
//	)
func Backoff(maxRetries int, initialDelay, maxDelay time.Duration) Decorator {
	return func(h Handler) Handler {
		return WithBackoff(h, maxRetries, initialDelay, maxDelay)
	}
}

// Timeout returns a Decorator that wraps a handler with timeout logic.
// This is a factory function for use with the Decorate helper.
//
// Example:
//
//	handler := event.Decorate(
//	    event.NewHandlerFunc(processImageHandler),
//	    event.Timeout(30*time.Second),
//	)
func Timeout(timeout time.Duration) Decorator {
	return func(h Handler) Handler {
		return WithTimeout(h, timeout)
	}
}

// Decorate applies multiple decorators to a handler in sequence.
// Decorators are applied left-to-right (first decorator wraps innermost).
//
// Example:
//
//	handler := event.Decorate(
//	    event.NewHandlerFunc(notifyWebhookHandler),
//	    event.Retry(3),
//	    event.Backoff(5, 100*time.Millisecond, 10*time.Second),
//	    event.Timeout(30*time.Second),
//	)
//	processor.Register(handler)
func Decorate(handler Handler, decorators ...Decorator) Handler {
	for _, decorator := range decorators {
		handler = decorator(handler)
	}
	return handler
}
