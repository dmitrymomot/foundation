package event

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// HandlerFunc is a type-safe function signature for processing events of type T.
type HandlerFunc[T any] func(context.Context, T) error

// Handler processes events.
// Implementations are registered with a Processor to handle specific event types.
type Handler interface {
	// EventName returns the event name this handler processes.
	EventName() string

	// Handle executes the handler with the given event payload.
	Handle(ctx context.Context, payload any) error
}

// NewHandler creates a new handler with a manually specified event name.
// Use this when you need explicit control over the event name.
//
// Example:
//
//	handler := event.NewHandler("user.created", func(ctx context.Context, payload any) error {
//	    evt := payload.(UserCreated)
//	    return processEvent(ctx, evt)
//	})
func NewHandler[T any](eventName string, fn HandlerFunc[T]) Handler {
	return &handlerFuncWrapper[T]{
		name: eventName,
		fn:   fn,
	}
}

// NewHandlerFunc creates a new type-safe handler from a function.
// The event name is automatically derived from the type parameter using reflection.
//
// Example:
//
//	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//	    return processEvent(ctx, evt)
//	})
func NewHandlerFunc[T any](fn HandlerFunc[T]) Handler {
	var zero T
	name := getEventName(zero)

	return &handlerFuncWrapper[T]{
		name: name,
		fn:   fn,
	}
}

// handlerFuncWrapper is a generic, type-safe event handler implementation.
type handlerFuncWrapper[T any] struct {
	name string
	fn   func(context.Context, T) error
}

// EventName returns the event name this handler processes.
func (h *handlerFuncWrapper[T]) EventName() string {
	return h.name
}

// Handle executes the handler function with type-safe payload conversion.
// Returns an error if the payload cannot be converted to type T.
func (h *handlerFuncWrapper[T]) Handle(ctx context.Context, payload any) error {
	typed, err := unmarshalPayload[T](payload)
	if err != nil {
		return err
	}
	return h.fn(ctx, typed)
}

// getEventName extracts the event name from a value using reflection.
// For struct types, it returns the struct name (e.g., "UserCreated").
// For pointer types, it returns the pointed-to type name.
func getEventName(v any) string {
	t := reflect.TypeOf(v)

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}

// unmarshalPayload attempts to convert payload to type T.
// Handles both pre-typed payloads and []byte that needs unmarshaling.
func unmarshalPayload[T any](payload any) (T, error) {
	var zero T

	// Already the correct type
	if v, ok := payload.(T); ok {
		return v, nil
	}

	// Unmarshal from bytes
	if data, ok := payload.([]byte); ok {
		var evt T
		if err := json.Unmarshal(data, &evt); err != nil {
			return zero, fmt.Errorf("failed to unmarshal event: %w", err)
		}
		return evt, nil
	}

	return zero, fmt.Errorf("unexpected payload type: %T", payload)
}
