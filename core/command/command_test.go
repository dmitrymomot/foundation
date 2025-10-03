package command_test

import (
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCommand struct {
	UserID string
	Email  string
}

type AnotherTestCommand struct {
	OrderID string
}

func TestNewCommand(t *testing.T) {
	t.Parallel()

	t.Run("creates command with auto-generated ID", func(t *testing.T) {
		t.Parallel()

		payload := TestCommand{UserID: "123", Email: "test@example.com"}
		cmd := command.NewCommand(payload)

		require.NotEmpty(t, cmd.ID)
		assert.Equal(t, "TestCommand", cmd.Name)
		assert.Equal(t, payload, cmd.Payload)
		assert.WithinDuration(t, time.Now(), cmd.CreatedAt, time.Second)
	})

	t.Run("creates command with different payload type", func(t *testing.T) {
		t.Parallel()

		payload := AnotherTestCommand{OrderID: "order-456"}
		cmd := command.NewCommand(payload)

		require.NotEmpty(t, cmd.ID)
		assert.Equal(t, "AnotherTestCommand", cmd.Name)
		assert.Equal(t, payload, cmd.Payload)
	})

	t.Run("generates unique IDs for multiple commands", func(t *testing.T) {
		t.Parallel()

		cmd1 := command.NewCommand(TestCommand{})
		cmd2 := command.NewCommand(TestCommand{})
		cmd3 := command.NewCommand(TestCommand{})

		require.NotEqual(t, cmd1.ID, cmd2.ID)
		require.NotEqual(t, cmd2.ID, cmd3.ID)
		require.NotEqual(t, cmd1.ID, cmd3.ID)
	})

	t.Run("derives command name from pointer type", func(t *testing.T) {
		t.Parallel()

		payload := &TestCommand{UserID: "123"}
		cmd := command.NewCommand(payload)

		assert.Equal(t, "TestCommand", cmd.Name)
	})

	t.Run("handles nested pointer types", func(t *testing.T) {
		t.Parallel()

		payload := &TestCommand{UserID: "123"}
		payloadPtr := &payload
		cmd := command.NewCommand(payloadPtr)

		assert.Equal(t, "TestCommand", cmd.Name)
	})
}
