package command

import (
	"context"
	"time"
)

// Decorator wraps a command handler function to add cross-cutting functionality.
// It follows the same pattern as HTTP middleware, allowing decorators to be
// composed and applied in order.
//
// Example:
//
//	func LoggingDecorator[T any](fn HandlerFunc[T]) HandlerFunc[T] {
//	    return func(ctx context.Context, payload T) error {
//	        log.Info("processing command", "type", reflect.TypeOf(payload).Name())
//	        err := fn(ctx, payload)
//	        if err != nil {
//	            log.Error("command processing failed", "error", err)
//	        }
//	        return err
//	    }
//	}
type Decorator[T any] func(HandlerFunc[T]) HandlerFunc[T]

// ApplyDecorators applies a series of decorators to a handler function.
// Decorators are applied in the order they are defined: the first decorator
// in the list becomes the outermost wrapper (executes first).
//
// Example:
//
//	handler := command.ApplyDecorators(
//	    myHandler,
//	    LoggingDecorator[MyCommand],
//	    MetricsDecorator[MyCommand],
//	    RetryDecorator[MyCommand],
//	)
//
// Execution order: Logging -> Metrics -> Retry -> myHandler
func ApplyDecorators[T any](fn HandlerFunc[T], decorators ...Decorator[T]) HandlerFunc[T] {
	// Reverse iteration ensures first decorator becomes outermost wrapper
	for i := range len(decorators) {
		fn = decorators[len(decorators)-1-i](fn)
	}
	return fn
}

// WithTimeout creates a decorator that enforces a timeout for handler execution.
// The handler must respect context cancellation for the timeout to work.
// This decorator can be used to override the dispatcher-level default timeout.
//
// Example:
//
//	handler := command.NewHandlerFunc(
//	    command.ApplyDecorators(
//	        mySlowHandler,
//	        command.WithTimeout[MyCommand](5*time.Minute),
//	    ),
//	)
func WithTimeout[T any](timeout time.Duration) Decorator[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, payload T) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return next(ctx, payload)
		}
	}
}
