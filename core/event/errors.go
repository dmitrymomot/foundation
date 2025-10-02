package event

import "errors"

var (
	// ErrNoHandlers is returned when an event has no registered handlers.
	ErrNoHandlers = errors.New("no handlers registered for event")

	// ErrProcessorAlreadyStarted is returned when attempting to start an already running processor.
	ErrProcessorAlreadyStarted = errors.New("processor already started")

	// ErrProcessorNotStarted is returned when attempting to stop a processor that hasn't been started.
	ErrProcessorNotStarted = errors.New("processor not started")

	// ErrEventSourceNil is returned when the event source is nil during processor startup.
	ErrEventSourceNil = errors.New("event source cannot be nil")

	// ErrHealthcheckFailed is returned when the processor health check fails.
	ErrHealthcheckFailed = errors.New("healthcheck failed")

	// ErrProcessorNotRunning is returned when the processor is not running during health checks.
	ErrProcessorNotRunning = errors.New("processor not running")

	// ErrChannelBusClosed is returned when attempting to publish to a closed channel bus.
	ErrChannelBusClosed = errors.New("channel bus is closed")
)
