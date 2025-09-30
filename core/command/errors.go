package command

import "errors"

var (
	// ErrHandlerNotFound is returned when no handler is registered for a command type.
	ErrHandlerNotFound = errors.New("handler not found for command")

	// ErrDuplicateHandler is returned when attempting to register a handler
	// for a command type that already has a handler registered.
	ErrDuplicateHandler = errors.New("handler already registered for command")

	// ErrBufferFull is returned by channel transport when the command buffer is full
	// and cannot accept more commands without blocking.
	ErrBufferFull = errors.New("command buffer is full")
)
