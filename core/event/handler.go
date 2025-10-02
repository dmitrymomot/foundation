package event

import (
	"context"
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

type handlerFuncWrapper[T any] struct {
	name string
	fn   func(context.Context, T) error
}

func (h *handlerFuncWrapper[T]) EventName() string {
	return h.name
}

func (h *handlerFuncWrapper[T]) Handle(ctx context.Context, payload any) error {
	typed, err := unmarshalPayload[T](payload)
	if err != nil {
		return err
	}
	return h.fn(ctx, typed)
}
