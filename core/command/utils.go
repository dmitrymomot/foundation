package command

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// envelope is an internal type used by async transports to pass commands
// through channels with their metadata.
type envelope struct {
	Context context.Context // Original dispatch context
	Name    string          // Command name for handler lookup
	Payload any             // Command data
}

// commandNameCache caches reflection results for command name lookups.
// Key is reflect.Type, value is the command name string.
var commandNameCache sync.Map

// getCommandName derives the command name from a reflect.Type.
// For structs, it returns the struct name.
// For pointers to structs, it returns the struct name.
// Results are cached to avoid repeated reflection overhead.
func getCommandName(t reflect.Type) string {
	if name, ok := commandNameCache.Load(t); ok {
		return name.(string)
	}

	original := t
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	var name string
	if t.Name() != "" {
		name = t.Name()
	} else {
		name = t.String()
	}

	commandNameCache.Store(original, name)
	return name
}

// getCommandNameFromInstance returns the command name for a given command instance.
func getCommandNameFromInstance(cmd any) string {
	return getCommandName(reflect.TypeOf(cmd))
}

// chainMiddleware applies multiple middleware in order.
// The first middleware in the slice is the outermost (executed first).
func chainMiddleware(handler Handler, middleware []Middleware) Handler {
	// Reverse order required: wrapping innermost first makes it execute last
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

// safeHandle executes a handler with panic recovery.
// If the handler panics, the panic is caught and converted to an error.
// This provides a single point of panic recovery for all transports.
func safeHandle(handler Handler, ctx context.Context, payload any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler %s panicked: %v", handler.Name(), r)
		}
	}()
	return handler.Handle(ctx, payload)
}
