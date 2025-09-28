package queue

import (
	"context"
	"encoding/json"
)

type (
	// Handler defines the interface for task processors.
	// All task handlers must implement Name() to identify the task type
	// and Handle() to process the task payload.
	Handler interface {
		// Name returns the task type name used for handler registration and routing.
		Name() string
		// Handle processes the task with the given payload.
		// The payload is provided as raw JSON and must be unmarshaled by the handler.
		Handle(ctx context.Context, payload json.RawMessage) error
	}

	// TaskHandlerFunc is a type-safe handler function for one-time tasks.
	// The generic type T represents the expected payload structure.
	TaskHandlerFunc[T any] func(ctx context.Context, payload T) error

	// PeriodicTaskHandlerFunc is a handler function for periodic tasks.
	// Periodic tasks have no payload and are triggered by the scheduler.
	PeriodicTaskHandlerFunc func(ctx context.Context) error
)

// NewTaskHandler creates a type-safe handler for one-time tasks.
// The handler function receives a strongly-typed payload and the task name
// is automatically derived from the payload type (e.g., "EmailPayload").
func NewTaskHandler[T any](handler TaskHandlerFunc[T]) Handler {
	var payload T
	return &oneTimeTaskHandler[T]{
		name:    qualifiedStructName(payload),
		handler: handler,
	}
}

// NewPeriodicTaskHandler creates a handler for periodic tasks.
// The name parameter specifies the task name used for scheduling.
// Periodic tasks have no payload and are triggered by the scheduler.
func NewPeriodicTaskHandler(name string, handler PeriodicTaskHandlerFunc) Handler {
	return &periodicTaskHandler{
		name:    name,
		handler: handler,
	}
}

type oneTimeTaskHandler[T any] struct {
	name    string
	handler TaskHandlerFunc[T]
}

func (h *oneTimeTaskHandler[T]) Name() string {
	return h.name
}

func (h *oneTimeTaskHandler[T]) Handle(ctx context.Context, payload json.RawMessage) error {
	var t T
	if err := json.Unmarshal(payload, &t); err != nil {
		return err
	}
	return h.handler(ctx, t)
}

type periodicTaskHandler struct {
	name    string
	handler PeriodicTaskHandlerFunc
}

func (h *periodicTaskHandler) Name() string {
	return h.name
}

func (h *periodicTaskHandler) Handle(ctx context.Context, _ json.RawMessage) error {
	return h.handler(ctx)
}
