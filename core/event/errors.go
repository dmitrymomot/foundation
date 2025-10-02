package event

import "errors"

var (
	ErrNoHandlers = errors.New("no handlers registered for event")
)
