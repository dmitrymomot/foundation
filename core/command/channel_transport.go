package command

import (
	"context"
)

// channelTransport executes commands asynchronously using buffered channels.
// This is a passive wire - it just provides a channel for the Processor to consume.
// The Processor manages workers that read from this channel.
// It implements both DispatcherTransport and ProcessorTransport interfaces.
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
// - Non-critical async work
type channelTransport struct {
	ch chan envelope
}

// NewChannelTransport creates a new channel-based async transport.
// The Processor manages workers that consume from this channel.
//
// Example:
//
//	transport := command.NewChannelTransport(100)
//	processor := command.NewProcessor(transport, command.WithWorkers(5))
func NewChannelTransport(bufferSize int) *channelTransport {
	if bufferSize < 1 {
		panic("command: bufferSize must be at least 1")
	}

	return &channelTransport{
		ch: make(chan envelope, bufferSize),
	}
}

// Dispatch sends a command to the channel for async execution.
// Returns ErrBufferFull if the channel buffer is full.
// The dispatch context is preserved and passed to the handler.
//
// Implements DispatcherTransport interface.
func (t *channelTransport) Dispatch(ctx context.Context, cmdName string, payload any) error {
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

// Subscribe returns the command channel for the Processor to consume.
// The Processor will manage workers that read from this channel.
//
// Implements ProcessorTransport interface.
func (t *channelTransport) Subscribe(ctx context.Context) (<-chan envelope, error) {
	return t.ch, nil
}

// Close closes the command channel, signaling workers to stop.
// Workers managed by the Processor will drain remaining commands before exiting.
//
// Implements ProcessorTransport interface.
func (t *channelTransport) Close() error {
	close(t.ch)
	return nil
}
