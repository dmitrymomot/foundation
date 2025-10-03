package command

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
)

// commandBus represents a message bus that can publish commands.
type commandBus interface {
	Publish(ctx context.Context, data []byte) error
}

// Sender publishes commands to a command bus.
type Sender struct {
	bus    commandBus
	logger *slog.Logger
}

// SenderOption configures a Sender.
type SenderOption func(*Sender)

// WithSenderLogger configures structured logging for sender operations.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithSenderLogger(logger *slog.Logger) SenderOption {
	return func(s *Sender) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// NewSender creates a new command sender with the given command bus.
//
// Example:
//
//	sender := command.NewSender(bus)
//	err := sender.Send(ctx, CreateUser{UserID: "123", Email: "user@example.com"})
func NewSender(bus commandBus, opts ...SenderOption) *Sender {
	s := &Sender{
		bus:    bus,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Send creates a Command from the payload and publishes it to the command bus.
// The command name is automatically derived from the payload type.
// The Command is marshaled to JSON before publishing.
func (s *Sender) Send(ctx context.Context, payload any) error {
	command := NewCommand(payload)

	data, err := json.Marshal(command)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to marshal command",
			slog.String("command_id", command.ID),
			slog.String("command_name", command.Name),
			slog.String("error", err.Error()))
		return err
	}

	if err := s.bus.Publish(ctx, data); err != nil {
		s.logger.ErrorContext(ctx, "failed to publish command",
			slog.String("command_id", command.ID),
			slog.String("command_name", command.Name),
			slog.String("error", err.Error()))
		return err
	}

	s.logger.DebugContext(ctx, "command published",
		slog.String("command_id", command.ID),
		slog.String("command_name", command.Name))

	return nil
}
