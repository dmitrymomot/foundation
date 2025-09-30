package command

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// channelTransport executes commands asynchronously using buffered channels.
// Commands are dispatched to a channel and processed by worker goroutines.
//
// Characteristics:
// - Non-blocking dispatch
// - Buffered channel (configurable size)
// - Local execution (same process)
// - No persistence (commands lost on shutdown)
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
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	shutdownOnce sync.Once
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
func newChannelTransport(
	bufferSize int,
	getHandler func(string) (Handler, bool),
	errorHandler func(context.Context, string, error),
	logger *slog.Logger,
	opts ...ChannelOption,
) Transport {
	ctx, cancel := context.WithCancel(context.Background())

	t := &channelTransport{
		ch:           make(chan envelope, bufferSize),
		getHandler:   getHandler,
		errorHandler: errorHandler,
		logger:       logger,
		workers:      1, // default
		ctx:          ctx,
		cancel:       cancel,
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

	return t
}

// Dispatch sends a command to the channel for async execution.
// Validates handler exists before enqueuing (fail fast).
// Returns ErrBufferFull if the channel buffer is full.
// Returns ErrHandlerNotFound if no handler is registered.
func (t *channelTransport) Dispatch(ctx context.Context, cmdName string, payload any) error {
	// Validate handler exists (fail fast)
	if _, exists := t.getHandler(cmdName); !exists {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, cmdName)
	}

	// Serialize command
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal command %s: %w", cmdName, err)
	}

	env := envelope{
		Name:    cmdName,
		Payload: data,
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

// worker processes commands from the channel.
func (t *channelTransport) worker() {
	defer t.wg.Done()

	for {
		select {
		case <-t.ctx.Done():
			return
		case env, ok := <-t.ch:
			if !ok {
				// Channel closed
				return
			}
			t.handleCommand(env)
		}
	}
}

// handleCommand processes a single command with panic recovery.
func (t *channelTransport) handleCommand(env envelope) {
	defer func() {
		if r := recover(); r != nil {
			t.logger.Error("command handler panicked",
				slog.String("command", env.Name),
				slog.Any("panic", r))

			if t.errorHandler != nil {
				t.errorHandler(context.Background(), env.Name,
					fmt.Errorf("handler panicked: %v", r))
			}
		}
	}()

	// Deserialize command
	cmd, err := UnmarshalCommand(env.Name, env.Payload)
	if err != nil {
		t.logger.Error("failed to unmarshal command",
			slog.String("command", env.Name),
			slog.String("error", err.Error()))

		if t.errorHandler != nil {
			t.errorHandler(context.Background(), env.Name, err)
		}
		return
	}

	// Get handler (with middleware applied)
	handler, exists := t.getHandler(env.Name)
	if !exists {
		// This shouldn't happen since we validate in Dispatch,
		// but handle it gracefully anyway
		err := fmt.Errorf("%w: %s", ErrHandlerNotFound, env.Name)
		t.logger.Error("handler not found",
			slog.String("command", env.Name))

		if t.errorHandler != nil {
			t.errorHandler(context.Background(), env.Name, err)
		}
		return
	}

	// Execute handler with fresh context
	ctx := context.Background()
	if err := handler.Handle(ctx, cmd); err != nil {
		if t.errorHandler != nil {
			t.errorHandler(ctx, env.Name, err)
		}
	}
}

// Stop gracefully shuts down the transport.
// Closes the channel and waits for workers to finish processing.
// Blocks until all workers are done or timeout (30 seconds).
func (t *channelTransport) Stop() {
	t.shutdownOnce.Do(func() {
		// Signal workers to stop
		t.cancel()

		// Close channel to drain remaining commands
		close(t.ch)

		// Wait for workers with timeout
		done := make(chan struct{})
		go func() {
			t.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.logger.Info("channel transport stopped gracefully")
		case <-time.After(30 * time.Second):
			t.logger.Warn("channel transport shutdown timeout")
		}
	})
}
