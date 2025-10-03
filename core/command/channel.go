package command

import (
	"context"
	"io"
	"log/slog"
	"sync"
)

const (
	// DefaultChannelBufferSize is the default buffer size for the in-memory channel bus.
	DefaultChannelBufferSize = 100
)

// ChannelBus implements both commandBus and commandSource interfaces using Go channels.
// It provides a simple in-memory command bus suitable for single-instance monolithic applications.
//
// ChannelBus is thread-safe and can handle concurrent publishers.
// It uses a buffered channel to prevent blocking publishers when the dispatcher is slow.
//
// Example:
//
//	bus := command.NewChannelBus(
//	    command.WithBufferSize(100),
//	    command.WithChannelLogger(logger),
//	)
//	defer bus.Close()
//
//	sender := command.NewSender(bus)
//	dispatcher := command.NewDispatcher(
//	    command.WithCommandSource(bus),
//	    command.WithHandler(handler),
//	)
type ChannelBus struct {
	ch     chan []byte
	logger *slog.Logger
	mu     sync.RWMutex
	closed bool
}

// ChannelBusOption configures a ChannelBus.
type ChannelBusOption func(*ChannelBus)

// WithBufferSize sets the buffer size for the command channel.
// Default is 100. A larger buffer allows more commands to be queued
// before publishers block.
func WithBufferSize(size int) ChannelBusOption {
	return func(b *ChannelBus) {
		if size > 0 {
			b.ch = make(chan []byte, size)
		}
	}
}

// WithChannelLogger configures structured logging for the channel bus.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithChannelLogger(logger *slog.Logger) ChannelBusOption {
	return func(b *ChannelBus) {
		if logger != nil {
			b.logger = logger
		}
	}
}

// NewChannelBus creates a new in-memory channel-based command bus.
//
// Example:
//
//	bus := command.NewChannelBus(
//	    command.WithBufferSize(100),
//	    command.WithChannelLogger(logger),
//	)
//	defer bus.Close()
func NewChannelBus(opts ...ChannelBusOption) *ChannelBus {
	b := &ChannelBus{
		ch:     make(chan []byte, DefaultChannelBufferSize),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		closed: false,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Publish implements the commandBus interface.
// It sends the command data to the channel. If the channel is closed,
// it returns an error.
func (b *ChannelBus) Publish(ctx context.Context, data []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return ErrChannelBusClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.ch <- data:
		b.logger.DebugContext(ctx, "command published to channel",
			slog.Int("data_size", len(data)))
		return nil
	}
}

// Commands implements the commandSource interface.
// It returns a read-only channel that emits command data.
func (b *ChannelBus) Commands() <-chan []byte {
	return b.ch
}

// Close gracefully shuts down the channel bus by closing the underlying channel.
// After Close is called, Publish will return an error.
// This should be called to signal the dispatcher that no more commands will be published.
func (b *ChannelBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrChannelBusClosed
	}

	b.closed = true
	close(b.ch)
	b.logger.Info("channel bus closed")
	return nil
}
