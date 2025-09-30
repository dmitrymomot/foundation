package event

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// syncTransport executes events synchronously in the caller's goroutine.
// This is the simplest and most efficient transport with zero overhead.
// It implements both PublisherTransport and ProcessorTransport interfaces.
//
// Characteristics:
// - Direct function call (no goroutines, no channels)
// - Synchronous error handling (errors.Join for all handlers)
// - Runs in caller's context
// - No worker management needed
//
// Use cases:
// - Testing (deterministic execution)
// - Simple applications
// - Transaction boundaries
// - HTTP request handlers
type syncTransport struct {
	getHandlers func(string) []Handler
	once        sync.Once
}

// NewSyncTransport creates a new synchronous transport.
func NewSyncTransport() *syncTransport {
	return &syncTransport{}
}

// Dispatch executes all handlers for the event immediately in the caller's goroutine.
// Returns aggregated errors from all handlers via errors.Join().
// Panics from handlers are caught and converted to errors.
//
// Implements PublisherTransport interface.
func (t *syncTransport) Dispatch(ctx context.Context, eventName string, payload any) error {
	if t.getHandlers == nil {
		return fmt.Errorf("sync transport not initialized; call SetGetHandlers() first")
	}

	// Check if context is already cancelled before starting
	if err := ctx.Err(); err != nil {
		return err
	}

	handlers := t.getHandlers(eventName)

	// Collect all handler errors
	var errs []error
	for _, handler := range handlers {
		if err := safeHandle(handler, ctx, payload); err != nil {
			errs = append(errs, fmt.Errorf("handler %s failed: %w", handler.Name(), err))
		}
	}

	// Return aggregated errors
	return errors.Join(errs...)
}

// Subscribe returns nil for sync transport, signaling immediate execution.
// This is the special case that tells Processor to not start workers.
//
// Implements ProcessorTransport interface.
func (t *syncTransport) Subscribe(ctx context.Context) (<-chan envelope, error) {
	// Return nil to signal sync transport (no channel, no workers)
	return nil, nil
}

// SetGetHandlers initializes the sync transport with the handler lookup function.
// This is called by Processor during initialization.
// Uses sync.Once to ensure thread-safe initialization.
func (t *syncTransport) SetGetHandlers(getHandlers func(string) []Handler) {
	t.once.Do(func() {
		t.getHandlers = getHandlers
	})
}

// Close performs cleanup. For sync transport, this is a no-op.
//
// Implements ProcessorTransport interface.
func (t *syncTransport) Close() error {
	return nil
}
