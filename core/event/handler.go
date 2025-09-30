package event

import (
	"context"
	"fmt"
	"reflect"
)

// Handler processes events.
// Implementations are registered with a Processor to handle specific event types.
type Handler interface {
	// Name returns the event name this handler processes.
	Name() string

	// Handle executes the handler with the given event payload.
	Handle(ctx context.Context, payload any) error
}

// HandlerFunc is a generic, type-safe event handler implementation.
// It uses reflection to automatically derive the event name from the type parameter.
//
// Example:
//
//	type UserCreated struct {
//	    UserID string
//	    Email  string
//	}
//
//	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//	    return cache.Invalidate(ctx, evt.UserID)
//	})
//	// handler.Name() returns "UserCreated"
type HandlerFunc[T any] struct {
	name string
	fn   func(context.Context, T) error
}

// NewHandlerFunc creates a new type-safe handler from a function.
// The event name is automatically derived from the type parameter using reflection.
//
// Example:
//
//	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//	    return processEvent(ctx, evt)
//	})
func NewHandlerFunc[T any](fn func(context.Context, T) error) Handler {
	var zero T
	name := getEventName(zero)

	return &HandlerFunc[T]{
		name: name,
		fn:   fn,
	}
}

// Name returns the event name this handler processes.
func (h *HandlerFunc[T]) Name() string {
	return h.name
}

// Handle executes the handler function with type conversion.
func (h *HandlerFunc[T]) Handle(ctx context.Context, payload any) error {
	typed, ok := payload.(T)
	if !ok {
		return fmt.Errorf("invalid payload type: expected %s, got %T", h.name, payload)
	}

	return h.fn(ctx, typed)
}

// getEventName extracts the event name from a value using reflection.
// For struct types, it returns the struct name (e.g., "UserCreated").
// For pointer types, it returns the pointed-to type name.
func getEventName(v any) string {
	t := reflect.TypeOf(v)

	// Dereference pointer if needed
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Return type name
	return t.Name()
}

// getEventNameFromInstance extracts event name from an event instance.
func getEventNameFromInstance(event any) string {
	return getEventName(event)
}
