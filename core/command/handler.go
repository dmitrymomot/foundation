package command

import (
	"context"
)

// HandlerFunc is a type-safe function signature for processing commands of type T.
type HandlerFunc[T any] func(context.Context, T) error

// Handler processes commands.
// Implementations are registered with a Dispatcher to handle specific command types.
type Handler interface {
	// CommandName returns the command name this handler processes.
	CommandName() string

	// Handle executes the handler with the given command payload.
	Handle(ctx context.Context, payload any) error
}

// NewHandler creates a new handler with a manually specified command name.
// Use this when you need explicit control over the command name.
//
// Example:
//
//	handler := command.NewHandler("user.create", func(ctx context.Context, payload any) error {
//	    cmd := payload.(CreateUser)
//	    return processCommand(ctx, cmd)
//	})
func NewHandler[T any](commandName string, fn HandlerFunc[T]) Handler {
	return &handlerFuncWrapper[T]{
		name: commandName,
		fn:   fn,
	}
}

// NewHandlerFunc creates a new type-safe handler from a function.
// The command name is automatically derived from the type parameter using reflection.
//
// Example:
//
//	handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
//	    return processCommand(ctx, cmd)
//	})
func NewHandlerFunc[T any](fn HandlerFunc[T]) Handler {
	var zero T
	name := getCommandName(zero)

	return &handlerFuncWrapper[T]{
		name: name,
		fn:   fn,
	}
}

type handlerFuncWrapper[T any] struct {
	name string
	fn   func(context.Context, T) error
}

func (h *handlerFuncWrapper[T]) CommandName() string {
	return h.name
}

func (h *handlerFuncWrapper[T]) Handle(ctx context.Context, payload any) error {
	typed, err := unmarshalPayload[T](payload)
	if err != nil {
		return err
	}
	return h.fn(ctx, typed)
}
