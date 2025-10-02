package event

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

// ChannelBus implements both eventBus and eventSource interfaces using Go channels.
// It provides a simple in-memory event bus suitable for single-instance monolithic applications.
//
// ChannelBus is thread-safe and can handle concurrent publishers.
// It uses a buffered channel to prevent blocking publishers when the processor is slow.
//
// Example:
//
//	bus := event.NewChannelBus(
//	    event.WithBufferSize(100),
//	    event.WithChannelLogger(logger),
//	)
//	defer bus.Close()
//
//	publisher := event.NewPublisher(bus)
//	processor := event.NewProcessor(
//	    event.WithEventSource(bus),
//	    event.WithHandler(handler),
//	)
type ChannelBus struct {
	ch     chan []byte
	logger *slog.Logger
	mu     sync.RWMutex
	closed bool
}

// ChannelBusOption configures a ChannelBus.
type ChannelBusOption func(*ChannelBus)

// WithBufferSize sets the buffer size for the event channel.
// Default is 100. A larger buffer allows more events to be queued
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

// NewChannelBus creates a new in-memory channel-based event bus.
//
// Example:
//
//	bus := event.NewChannelBus(
//	    event.WithBufferSize(100),
//	    event.WithChannelLogger(logger),
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

// Publish implements the eventBus interface.
// It sends the event data to the channel. If the channel is closed,
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
		b.logger.DebugContext(ctx, "event published to channel",
			slog.Int("data_size", len(data)))
		return nil
	}
}

// Events implements the eventSource interface.
// It returns a read-only channel that emits event data.
func (b *ChannelBus) Events() <-chan []byte {
	return b.ch
}

// Close gracefully shuts down the channel bus by closing the underlying channel.
// After Close is called, Publish will return an error.
// This should be called to signal the processor that no more events will be published.
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
