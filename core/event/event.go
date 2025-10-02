package event

import (
	"time"

	"github.com/google/uuid"
)

// Event represents a domain event with metadata and payload.
type Event struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Payload   any       `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// NewEvent creates a new Event with auto-generated ID and timestamp.
// The event name is automatically derived from the payload type using reflection.
//
// Example:
//
//	type UserCreated struct {
//	    UserID string
//	    Email  string
//	}
//
//	event := event.NewEvent(UserCreated{UserID: "123", Email: "user@example.com"})
//	// event.Name will be "UserCreated"
//	// event.ID will be a UUID
//	// event.CreatedAt will be time.Now()
func NewEvent(payload any) Event {
	return Event{
		ID:        uuid.New().String(),
		Name:      getEventName(payload),
		Payload:   payload,
		CreatedAt: time.Now(),
	}
}
