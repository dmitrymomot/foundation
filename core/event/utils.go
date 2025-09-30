package event

import (
	"context"
	"fmt"
	"runtime/debug"
)

// envelope wraps an event with its execution context.
// Used internally by transports to pass events between components.
type envelope struct {
	Context context.Context
	Name    string
	Payload any
}

// safeHandle executes a handler with panic recovery.
// If the handler panics, the panic is caught and converted to an error.
// This prevents a panicking handler from crashing the entire process.
func safeHandle(handler Handler, ctx context.Context, payload any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Capture stack trace
			stack := debug.Stack()
			err = fmt.Errorf("handler panicked: %v\nstack trace:\n%s", r, stack)
		}
	}()

	return handler.Handle(ctx, payload)
}
