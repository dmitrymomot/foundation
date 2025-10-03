package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
)

func TestContextHelpers(t *testing.T) {
	t.Parallel()

	t.Run("CommandID", func(t *testing.T) {
		t.Parallel()

		t.Run("stores and retrieves command ID", func(t *testing.T) {
			t.Parallel()

			ctx := command.WithCommandID(context.Background(), "test-id-123")
			retrieved := command.CommandID(ctx)
			assert.Equal(t, "test-id-123", retrieved)
		})

		t.Run("returns empty string when not set", func(t *testing.T) {
			t.Parallel()

			retrieved := command.CommandID(context.Background())
			assert.Equal(t, "", retrieved)
		})

		t.Run("overwrites previous value", func(t *testing.T) {
			t.Parallel()

			ctx := command.WithCommandID(context.Background(), "first")
			ctx = command.WithCommandID(ctx, "second")
			retrieved := command.CommandID(ctx)
			assert.Equal(t, "second", retrieved)
		})
	})

	t.Run("CommandName", func(t *testing.T) {
		t.Parallel()

		t.Run("stores and retrieves command name", func(t *testing.T) {
			t.Parallel()

			ctx := command.WithCommandName(context.Background(), "CreateUser")
			retrieved := command.CommandName(ctx)
			assert.Equal(t, "CreateUser", retrieved)
		})

		t.Run("returns empty string when not set", func(t *testing.T) {
			t.Parallel()

			retrieved := command.CommandName(context.Background())
			assert.Equal(t, "", retrieved)
		})
	})

	t.Run("CommandTime", func(t *testing.T) {
		t.Parallel()

		t.Run("stores and retrieves command time", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			ctx := command.WithCommandTime(context.Background(), now)
			retrieved := command.CommandTime(ctx)
			assert.Equal(t, now, retrieved)
		})

		t.Run("returns zero time when not set", func(t *testing.T) {
			t.Parallel()

			retrieved := command.CommandTime(context.Background())
			assert.True(t, retrieved.IsZero())
		})
	})

	t.Run("StartProcessingTime", func(t *testing.T) {
		t.Parallel()

		t.Run("stores and retrieves processing start time", func(t *testing.T) {
			t.Parallel()

			start := time.Now()
			ctx := command.WithStartProcessingTime(context.Background(), start)
			retrieved := command.StartProcessingTime(ctx)
			assert.Equal(t, start, retrieved)
		})

		t.Run("returns zero time when not set", func(t *testing.T) {
			t.Parallel()

			retrieved := command.StartProcessingTime(context.Background())
			assert.True(t, retrieved.IsZero())
		})
	})

	t.Run("WithCommandMeta", func(t *testing.T) {
		t.Parallel()

		t.Run("attaches all command metadata", func(t *testing.T) {
			t.Parallel()

			createdAt := time.Now()
			cmd := command.Command{
				ID:        "cmd-789",
				Name:      "UpdateUser",
				Payload:   nil,
				CreatedAt: createdAt,
			}

			ctx := command.WithCommandMeta(context.Background(), cmd)

			assert.Equal(t, "cmd-789", command.CommandID(ctx))
			assert.Equal(t, "UpdateUser", command.CommandName(ctx))
			assert.Equal(t, createdAt, command.CommandTime(ctx))
		})
	})

	t.Run("context chaining", func(t *testing.T) {
		t.Parallel()

		t.Run("preserves all values when chaining", func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctx = command.WithCommandID(ctx, "id-1")
			ctx = command.WithCommandName(ctx, "CommandName")
			ctx = command.WithCommandTime(ctx, time.Now())
			ctx = command.WithStartProcessingTime(ctx, time.Now())

			assert.NotEmpty(t, command.CommandID(ctx))
			assert.NotEmpty(t, command.CommandName(ctx))
			assert.False(t, command.CommandTime(ctx).IsZero())
			assert.False(t, command.StartProcessingTime(ctx).IsZero())
		})
	})

	t.Run("context isolation", func(t *testing.T) {
		t.Parallel()

		t.Run("values do not leak between contexts", func(t *testing.T) {
			t.Parallel()

			ctx1 := command.WithCommandID(context.Background(), "context-1")
			ctx2 := command.WithCommandID(context.Background(), "context-2")

			assert.Equal(t, "context-1", command.CommandID(ctx1))
			assert.Equal(t, "context-2", command.CommandID(ctx2))
		})
	})

	t.Run("use case: latency calculation", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Now().Add(-100 * time.Millisecond)
		startedAt := time.Now()

		ctx := context.Background()
		ctx = command.WithCommandTime(ctx, createdAt)
		ctx = command.WithStartProcessingTime(ctx, startedAt)

		queueLatency := command.StartProcessingTime(ctx).Sub(command.CommandTime(ctx))
		assert.GreaterOrEqual(t, queueLatency, 100*time.Millisecond)
	})
}
