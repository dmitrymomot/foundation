package command

import "reflect"

// envelope is an internal type used by async transports to pass commands
// through channels with their metadata.
type envelope struct {
	Name    string // Command name for handler lookup
	Payload any    // Command data
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

// getCommandNameFromInstance returns the command name for a given command instance.
func getCommandNameFromInstance(cmd any) string {
	return getCommandName(reflect.TypeOf(cmd))
}

// chainMiddleware applies multiple middleware in order.
// The first middleware in the slice is the outermost (executed first).
func chainMiddleware(handler Handler, middleware []Middleware) Handler {
	// Apply middleware in reverse order so the first middleware
	// in the slice becomes the outermost wrapper
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}
