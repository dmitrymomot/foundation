package event

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
)

// eventBus represents a message bus that can publish events.
type eventBus interface {
	Publish(ctx context.Context, data []byte) error
}

// Publisher publishes events to an event bus.
type Publisher struct {
	bus    eventBus
	logger *slog.Logger
}

// PublisherOption configures a Publisher.
type PublisherOption func(*Publisher)

// WithPublisherLogger configures structured logging for publisher operations.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithPublisherLogger(logger *slog.Logger) PublisherOption {
	return func(p *Publisher) {
		if logger != nil {
			p.logger = logger
		}
	}
}

// NewPublisher creates a new event publisher with the given event bus.
//
// Example:
//
//	publisher := event.NewPublisher(bus)
//	err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "user@example.com"})
func NewPublisher(bus eventBus, opts ...PublisherOption) *Publisher {
	p := &Publisher{
		bus:    bus,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Publish creates an Event from the payload and publishes it to the event bus.
// The event name is automatically derived from the payload type.
// The Event is marshaled to JSON before publishing.
func (p *Publisher) Publish(ctx context.Context, payload any) error {
	event := NewEvent(payload)

	data, err := json.Marshal(event)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to marshal event",
			slog.String("event_id", event.ID),
			slog.String("event_name", event.Name),
			slog.String("error", err.Error()))
		return err
	}

	if err := p.bus.Publish(ctx, data); err != nil {
		p.logger.ErrorContext(ctx, "failed to publish event",
			slog.String("event_id", event.ID),
			slog.String("event_name", event.Name),
			slog.String("error", err.Error()))
		return err
	}

	p.logger.DebugContext(ctx, "event published",
		slog.String("event_id", event.ID),
		slog.String("event_name", event.Name))

	return nil
}
