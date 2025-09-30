package command

import (
	"context"
	"fmt"
)

// syncTransport executes commands synchronously in the caller's goroutine.
// This is the simplest and most efficient transport with zero overhead.
//
// Characteristics:
// - Direct function call (no goroutines, no channels)
// - Synchronous error handling
// - Runs in caller's context
// - No lifecycle management needed
//
// Use cases:
// - HTTP request-response handlers
// - Database transactions
// - Testing (deterministic execution)
// - Simple applications
type syncTransport struct {
	getHandler func(string) (Handler, bool)
}

// newSyncTransport creates a new synchronous transport.
// The getHandler function is used to look up handlers by command name.
func newSyncTransport(getHandler func(string) (Handler, bool)) Transport {
	return &syncTransport{
		getHandler: getHandler,
	}
}

// Dispatch executes the command immediately in the caller's goroutine.
// Returns ErrHandlerNotFound if no handler is registered for the command.
// Panics from handlers are caught and converted to errors.
func (t *syncTransport) Dispatch(ctx context.Context, cmdName string, payload any) error {
	handler, exists := t.getHandler(cmdName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, cmdName)
	}

	return safeHandle(handler, ctx, payload)
}
