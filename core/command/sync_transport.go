package command

import (
	"context"
	"fmt"
)

// syncTransport executes commands synchronously in the caller's goroutine.
// This is the simplest and most efficient transport with zero overhead.
// It implements both DispatcherTransport and ProcessorTransport interfaces.
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

// NewSyncTransport creates a new synchronous transport.
func NewSyncTransport() *syncTransport {
	return &syncTransport{}
}

// Dispatch executes the command immediately in the caller's goroutine.
// Returns ErrHandlerNotFound if no handler is registered for the command.
// Panics from handlers are caught and converted to errors.
//
// Implements DispatcherTransport interface.
func (t *syncTransport) Dispatch(ctx context.Context, cmdName string, payload any) error {
	if t.getHandler == nil {
		return fmt.Errorf("sync transport not initialized; call SetGetHandler() first")
	}

	handler, exists := t.getHandler(cmdName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, cmdName)
	}

	return safeHandle(handler, ctx, payload)
}

// Subscribe returns nil for sync transport, signaling immediate execution.
// This is the special case that tells Processor to not start workers.
//
// Implements ProcessorTransport interface.
func (t *syncTransport) Subscribe(ctx context.Context) (<-chan envelope, error) {
	// Return nil to signal sync transport (no channel, no workers)
	return nil, nil
}

// SetGetHandler initializes the sync transport with the handler lookup function.
// This is called by Processor during initialization.
func (t *syncTransport) SetGetHandler(getHandler func(string) (Handler, bool)) {
	t.getHandler = getHandler
}

// Close performs cleanup. For sync transport, this is a no-op.
//
// Implements ProcessorTransport interface.
func (t *syncTransport) Close() error {
	return nil
}
