package event

import "errors"

var (
	// ErrBufferFull is returned when the channel buffer is full.
	ErrBufferFull = errors.New("event buffer is full")

	// ErrNoHandlers is returned in strict mode when no handlers are registered for an event.
	ErrNoHandlers = errors.New("no handlers registered for event")
)
