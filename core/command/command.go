package command

import (
	"time"

	"github.com/google/uuid"
)

// Command represents a domain command with metadata and payload.
type Command struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Payload   any       `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// NewCommand creates a new Command with auto-generated ID and timestamp.
// The command name is automatically derived from the payload type using reflection.
//
// Example:
//
//	type CreateUser struct {
//	    UserID string
//	    Email  string
//	}
//
//	command := command.NewCommand(CreateUser{UserID: "123", Email: "user@example.com"})
//	// command.Name will be "CreateUser"
//	// command.ID will be a UUID
//	// command.CreatedAt will be time.Now()
func NewCommand(payload any) Command {
	return Command{
		ID:        uuid.New().String(),
		Name:      getCommandName(payload),
		Payload:   payload,
		CreatedAt: time.Now(),
	}
}
