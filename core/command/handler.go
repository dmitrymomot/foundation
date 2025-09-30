package command

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
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
	name    string
	cmdType reflect.Type
	fn      func(context.Context, T) error
}

var (
	// Global type registry for command deserialization
	typeRegistry   = make(map[string]reflect.Type)
	typeRegistryMu sync.RWMutex
)

// NewHandlerFunc creates a new type-safe command handler.
// The command name is automatically derived from the type T.
// The type T is registered globally for deserialization.
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

	// Register the type for deserialization
	registerType(name, cmdType)

	return &HandlerFunc[T]{
		name:    name,
		cmdType: cmdType,
		fn:      fn,
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

// getCommandName derives the command name from a reflect.Type.
// For structs, it returns the struct name.
// For pointers to structs, it returns the struct name.
func getCommandName(t reflect.Type) string {
	// Dereference pointers
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// For named types, use the name
	if t.Name() != "" {
		return t.Name()
	}

	// Fallback to string representation
	return t.String()
}

// registerType registers a command type in the global registry.
// This enables deserialization from JSON when only the command name is known.
func registerType(name string, t reflect.Type) {
	typeRegistryMu.Lock()
	defer typeRegistryMu.Unlock()
	typeRegistry[name] = t
}

// UnmarshalCommand deserializes a command from JSON using the registered type.
// Returns an error if the command name is not registered.
//
// This is primarily used by async transports that serialize commands.
func UnmarshalCommand(name string, data []byte) (any, error) {
	typeRegistryMu.RLock()
	cmdType, exists := typeRegistry[name]
	typeRegistryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("command type not registered: %s", name)
	}

	// Create a new instance of the type
	ptr := reflect.New(cmdType)
	instance := ptr.Interface()

	// Unmarshal into the instance
	if err := json.Unmarshal(data, instance); err != nil {
		return nil, fmt.Errorf("failed to unmarshal command %s: %w", name, err)
	}

	// Return the value (not pointer)
	return reflect.ValueOf(instance).Elem().Interface(), nil
}

// GetCommandName returns the command name for a given command instance.
// This is useful for logging and debugging.
func GetCommandName(cmd any) string {
	return getCommandName(reflect.TypeOf(cmd))
}
