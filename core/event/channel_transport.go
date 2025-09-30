package event

import (
	"context"
	"sync/atomic"
)

// channelTransport executes events asynchronously using buffered channels.
// This is a passive wire - it just provides a channel for the Processor to consume.
// The Processor manages workers that read from this channel.
// It implements both PublisherTransport and ProcessorTransport interfaces.
//
// Characteristics:
// - Non-blocking dispatch
// - Buffered channel (configurable size)
// - Passive wire (no worker management)
// - Local execution (same process)
//
// Use cases:
// - Fire-and-forget operations
// - Decoupling (don't block HTTP response)
// - Local background tasks
// - Async notifications
type channelTransport struct {
	ch     chan envelope
	closed atomic.Bool
}

// NewChannelTransport creates a new channel-based async transport.
// The Processor manages workers that consume from this channel.
//
// Example:
//
//	transport := event.NewChannelTransport(100)
//	processor := event.NewProcessor(transport, event.WithWorkers(5))
func NewChannelTransport(bufferSize int) *channelTransport {
	if bufferSize < 1 {
		panic("event: bufferSize must be at least 1")
	}

	return &channelTransport{
		ch: make(chan envelope, bufferSize),
	}
}

// Dispatch sends an event to the channel for async execution.
// Returns ErrBufferFull immediately if the channel buffer is full (non-blocking).
// The dispatch context is preserved and passed to handlers when they execute.
//
// Implements PublisherTransport interface.
func (t *channelTransport) Dispatch(ctx context.Context, eventName string, payload any) error {
	env := envelope{
		Context: ctx,
		Name:    eventName,
		Payload: payload,
	}

	// Non-blocking send - return immediately if buffer is full
	select {
	case t.ch <- env:
		return nil
	default:
		return ErrBufferFull
	}
}

// Subscribe returns the event channel for the Processor to consume.
// The Processor will manage workers that read from this channel.
//
// Implements ProcessorTransport interface.
func (t *channelTransport) Subscribe(ctx context.Context) (<-chan envelope, error) {
	return t.ch, nil
}

// Close closes the event channel, signaling workers to stop.
// Workers managed by the Processor will drain remaining events before exiting.
// This method is idempotent - multiple calls are safe.
//
// Implements ProcessorTransport interface.
func (t *channelTransport) Close() error {
	if t.closed.CompareAndSwap(false, true) {
		close(t.ch)
	}
	return nil
}
