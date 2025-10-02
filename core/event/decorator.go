package event

// Decorator wraps an event handler function to add cross-cutting functionality.
// It follows the same pattern as HTTP middleware, allowing decorators to be
// composed and applied in order.
//
// Example:
//
//	func LoggingDecorator[T any](fn HandlerFunc[T]) HandlerFunc[T] {
//	    return func(ctx context.Context, payload T) error {
//	        log.Info("processing event", "type", reflect.TypeOf(payload).Name())
//	        err := fn(ctx, payload)
//	        if err != nil {
//	            log.Error("event processing failed", "error", err)
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
//	handler := event.ApplyDecorators(
//	    myHandler,
//	    LoggingDecorator[MyEvent],
//	    MetricsDecorator[MyEvent],
//	    RetryDecorator[MyEvent],
//	)
//
// Execution order: Logging -> Metrics -> Retry -> myHandler
func ApplyDecorators[T any](fn HandlerFunc[T], decorators ...Decorator[T]) HandlerFunc[T] {
	// Apply decorators from last to first to achieve proper nesting order
	// This ensures the first decorator in the slice becomes the outermost wrapper
	for i := len(decorators) - 1; i >= 0; i-- {
		fn = decorators[i](fn)
	}
	return fn
}
