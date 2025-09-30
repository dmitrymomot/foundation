package command

import "context"

// Transport defines how commands are dispatched and executed.
// Different transport implementations provide different execution strategies:
// - Sync: Direct synchronous execution
// - Channel: Asynchronous execution via buffered channels
type Transport interface {
	// Dispatch sends a command for execution.
	// The cmdName identifies the handler, payload contains the command data.
	// Returns an error if dispatch fails (e.g., buffer full, handler not found).
	Dispatch(ctx context.Context, cmdName string, payload any) error
}

// envelope is an internal type used by async transports to pass commands
// through channels with their metadata.
type envelope struct {
	Name    string // Command name for handler lookup
	Payload []byte // JSON-serialized command data
}
