package command

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// channelTransport executes commands asynchronously using buffered channels.
// Commands are dispatched to a channel and processed by worker goroutines.
// Lifecycle is managed via the context passed to WithChannelTransport.
//
// Characteristics:
// - Non-blocking dispatch
// - Buffered channel (configurable size)
// - Local execution (same process)
// - Context-based lifecycle management
// - Error handling via callback
//
// Use cases:
// - Fire-and-forget operations
// - Decoupling (don't block HTTP response)
// - Local background tasks
// - Non-critical async work
type channelTransport struct {
	ch           chan envelope
	getHandler   func(string) (Handler, bool)
	errorHandler func(context.Context, string, error)
	logger       *slog.Logger
	workers      int
	ctx          context.Context
	wg           sync.WaitGroup
}

// ChannelOption configures the channel transport.
type ChannelOption func(*channelTransport)

// WithWorkers sets the number of worker goroutines.
// Default is 1. More workers increase concurrency.
func WithWorkers(n int) ChannelOption {
	return func(t *channelTransport) {
		if n > 0 {
			t.workers = n
		}
	}
}

// newChannelTransport creates a new channel-based async transport.
// Workers are started immediately and begin processing commands.
// When ctx is cancelled, workers drain the channel and exit gracefully.
func newChannelTransport(
	ctx context.Context,
	bufferSize int,
	getHandler func(string) (Handler, bool),
	errorHandler func(context.Context, string, error),
	logger *slog.Logger,
	opts ...ChannelOption,
) Transport {
	if bufferSize < 1 {
		panic("bufferSize must be at least 1")
	}

	t := &channelTransport{
		ch:           make(chan envelope, bufferSize),
		getHandler:   getHandler,
		errorHandler: errorHandler,
		logger:       logger,
		workers:      1, // default
		ctx:          ctx,
	}

	// Apply options
	for _, opt := range opts {
		opt(t)
	}

	// Start workers
	for i := 0; i < t.workers; i++ {
		t.wg.Add(1)
		go t.worker()
	}

	// Monitor context and close channel when cancelled
	go func() {
		<-ctx.Done()
		close(t.ch)
		t.logger.Info("channel transport shutting down, draining commands")
		t.wg.Wait()
		t.logger.Info("channel transport shutdown complete")
	}()

	return t
}

// Dispatch sends a command to the channel for async execution.
// Validates handler exists before enqueuing (fail fast).
// Returns ErrBufferFull if the channel buffer is full.
// Returns ErrHandlerNotFound if no handler is registered.
// The dispatch context is preserved and passed to the handler.
func (t *channelTransport) Dispatch(ctx context.Context, cmdName string, payload any) error {
	// Validate handler exists (fail fast)
	if _, exists := t.getHandler(cmdName); !exists {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, cmdName)
	}

	env := envelope{
		Context: ctx,
		Name:    cmdName,
		Payload: payload,
	}

	// Non-blocking send with timeout from context
	select {
	case t.ch <- env:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrBufferFull
	}
}

// worker processes commands from the channel until it's closed.
func (t *channelTransport) worker() {
	defer t.wg.Done()

	for env := range t.ch {
		t.handleCommand(env)
	}
}

// handleCommand processes a single command.
// Panics are caught and converted to errors by safeHandle.
func (t *channelTransport) handleCommand(env envelope) {
	// Get handler (with middleware applied)
	handler, exists := t.getHandler(env.Name)
	if !exists {
		// This shouldn't happen since we validate in Dispatch,
		// but handle it gracefully anyway
		err := fmt.Errorf("%w: %s", ErrHandlerNotFound, env.Name)
		t.logger.Error("handler not found",
			slog.String("command", env.Name))

		if t.errorHandler != nil {
			t.errorHandler(env.Context, env.Name, err)
		}
		return
	}

	// Execute handler with dispatch context
	// safeHandle catches panics and converts them to errors
	if err := safeHandle(handler, env.Context, env.Payload); err != nil {
		if t.errorHandler != nil {
			t.errorHandler(env.Context, env.Name, err)
		}
	}
}
