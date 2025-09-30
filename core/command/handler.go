package command

import (
	"context"
	"fmt"
	"reflect"
)

// Handler defines the interface for command handlers.
// Each handler processes a specific command type.
type Handler interface {
	// Name returns the unique command name this handler processes.
	Name() string

	// Handle executes the handler with the given command payload.
	// The payload must be of the type expected by this handler.
	Handle(ctx context.Context, payload any) error
}

// HandlerFunc is a generic function type that handles commands of type T.
// It automatically derives the command name from the type T and provides
// type-safe handling without manual type assertions.
type HandlerFunc[T any] struct {
	name string
	fn   func(context.Context, T) error
}

// NewHandlerFunc creates a new type-safe command handler.
// The command name is automatically derived from the type T.
//
// Example:
//
//	type CreateUser struct {
//	    Email string
//	    Name  string
//	}
//
//	handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
//	    return db.Insert(ctx, cmd.Email, cmd.Name)
//	})
func NewHandlerFunc[T any](fn func(context.Context, T) error) Handler {
	var zero T
	cmdType := reflect.TypeOf(zero)

	// Get the command name from the type
	name := getCommandName(cmdType)

	return &HandlerFunc[T]{
		name: name,
		fn:   fn,
	}
}

// Name returns the command name this handler processes.
func (h *HandlerFunc[T]) Name() string {
	return h.name
}

// Handle executes the handler with the given payload.
// The payload must be of type T or Handle will panic.
func (h *HandlerFunc[T]) Handle(ctx context.Context, payload any) error {
	cmd, ok := payload.(T)
	if !ok {
		return fmt.Errorf("invalid payload type: expected %s, got %T", h.name, payload)
	}
	return h.fn(ctx, cmd)
}
