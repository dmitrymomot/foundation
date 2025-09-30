package event

import (
	"context"
	"log/slog"
)

// Publisher publishes events to handlers via transport.
// It is a stateless client with no lifecycle management.
// For processing events with handlers and lifecycle, use Processor.
//
// Example:
//
//	publisher := event.NewPublisher(transport)
//	err := publisher.Publish(ctx, UserCreated{Email: "user@example.com"})
type Publisher struct {
	transport PublisherTransport
	logger    *slog.Logger
}

// PublisherOption configures a Publisher.
type PublisherOption func(*Publisher)

// NewPublisher creates a new event publisher with the given transport.
//
// Example:
//
//	transport := event.NewChannelTransport(100)
//	publisher := event.NewPublisher(transport, event.WithPublisherLogger(logger))
func NewPublisher(transport PublisherTransport, opts ...PublisherOption) *Publisher {
	p := &Publisher{
		transport: transport,
		logger:    slog.Default(),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Publish sends an event for execution via the configured transport.
// The event is routed to all registered handlers by the processor.
//
// For sync transport: blocks until all handlers complete, returns aggregated errors.
// For async transports: returns immediately, returns dispatch error only.
//
// Example:
//
//	err := publisher.Publish(ctx, UserCreated{
//	    Email: "user@example.com",
//	    Name:  "John Doe",
//	})
func (p *Publisher) Publish(ctx context.Context, event any) error {
	eventName := getEventNameFromInstance(event)
	return p.transport.Dispatch(ctx, eventName, event)
}

// WithPublisherLogger sets the logger for the publisher.
// If not set, slog.Default() is used.
//
// Example:
//
//	publisher := event.NewPublisher(transport, event.WithPublisherLogger(logger))
func WithPublisherLogger(logger *slog.Logger) PublisherOption {
	return func(p *Publisher) {
		p.logger = logger
	}
}
