package event_test

import (
	"testing"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
)

func TestNewEvent(t *testing.T) {
	t.Parallel()

	t.Run("creates event with struct payload", func(t *testing.T) {
		t.Parallel()

		type TestEvent struct {
			Data string
		}

		evt := event.NewEvent(TestEvent{Data: "test"})

		assert.NotEmpty(t, evt.ID, "event ID should be generated")
		assert.Equal(t, "TestEvent", evt.Name, "event name should match type")
		assert.False(t, evt.CreatedAt.IsZero(), "created_at should be set")
		assert.Equal(t, TestEvent{Data: "test"}, evt.Payload)
	})

	t.Run("panics with nil payload", func(t *testing.T) {
		t.Parallel()

		// NewEvent currently panics with nil payload
		// This test documents the current behavior
		assert.Panics(t, func() {
			_ = event.NewEvent(nil)
		}, "NewEvent should panic with nil payload (current behavior)")
	})

	t.Run("creates event with map payload", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{"key": "value"}
		evt := event.NewEvent(payload)

		assert.NotEmpty(t, evt.ID)
		assert.False(t, evt.CreatedAt.IsZero())
		assert.Equal(t, payload, evt.Payload)
	})

	t.Run("creates event with slice payload", func(t *testing.T) {
		t.Parallel()

		payload := []string{"item1", "item2"}
		evt := event.NewEvent(payload)

		assert.NotEmpty(t, evt.ID)
		assert.False(t, evt.CreatedAt.IsZero())
		assert.Equal(t, payload, evt.Payload)
	})

	t.Run("creates event with primitive payload", func(t *testing.T) {
		t.Parallel()

		evt := event.NewEvent("string payload")

		assert.NotEmpty(t, evt.ID)
		assert.False(t, evt.CreatedAt.IsZero())
		assert.Equal(t, "string payload", evt.Payload)
	})

	t.Run("generates unique IDs for multiple events", func(t *testing.T) {
		t.Parallel()

		evt1 := event.NewEvent("payload1")
		evt2 := event.NewEvent("payload2")

		assert.NotEqual(t, evt1.ID, evt2.ID, "event IDs should be unique")
	})

	t.Run("creates event with pointer payload", func(t *testing.T) {
		t.Parallel()

		type TestEvent struct {
			Value int
		}

		payload := &TestEvent{Value: 42}
		evt := event.NewEvent(payload)

		assert.NotEmpty(t, evt.ID)
		assert.Equal(t, "TestEvent", evt.Name)
		assert.Equal(t, payload, evt.Payload)
	})
}
