package command

import "errors"

var (
	// ErrNoHandler is returned when a command has no registered handler.
	ErrNoHandler = errors.New("no handler registered for command")

	// ErrHandlerAlreadyRegistered is returned when attempting to register a duplicate handler for a command.
	ErrHandlerAlreadyRegistered = errors.New("handler already registered for command")

	// ErrDispatcherAlreadyStarted is returned when attempting to start an already running dispatcher.
	ErrDispatcherAlreadyStarted = errors.New("dispatcher already started")

	// ErrDispatcherNotStarted is returned when attempting to stop a dispatcher that hasn't been started.
	ErrDispatcherNotStarted = errors.New("dispatcher not started")

	// ErrCommandSourceNil is returned when the command source is nil during dispatcher startup.
	ErrCommandSourceNil = errors.New("command source cannot be nil")

	// ErrHealthcheckFailed is returned when the dispatcher health check fails.
	ErrHealthcheckFailed = errors.New("healthcheck failed")

	// ErrDispatcherNotRunning is returned when the dispatcher is not running during health checks.
	ErrDispatcherNotRunning = errors.New("dispatcher not running")

	// ErrChannelBusClosed is returned when attempting to publish to a closed channel bus.
	ErrChannelBusClosed = errors.New("channel bus is closed")

	// ErrDispatcherStale is returned when the dispatcher has not processed commands recently.
	ErrDispatcherStale = errors.New("dispatcher is stale - no recent activity")

	// ErrDispatcherStuck is returned when the dispatcher has too many active commands.
	ErrDispatcherStuck = errors.New("dispatcher may be stuck - too many active commands")
)
