package command_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type CreateUser struct {
	UserID string
	Email  string
}

type UpdateUser struct {
	UserID string
	Name   string
}

func TestNewHandlerFunc(t *testing.T) {
	t.Parallel()

	t.Run("creates handler with auto-derived command name", func(t *testing.T) {
		t.Parallel()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		assert.Equal(t, "CreateUser", handler.CommandName())
	})

	t.Run("executes handler with correct payload", func(t *testing.T) {
		t.Parallel()

		var capturedCmd CreateUser
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			capturedCmd = cmd
			return nil
		})

		payload := CreateUser{UserID: "123", Email: "test@example.com"}
		err := handler.Handle(context.Background(), payload)

		require.NoError(t, err)
		assert.Equal(t, payload, capturedCmd)
	})

	t.Run("propagates handler errors", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("validation failed")
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return expectedErr
		})

		err := handler.Handle(context.Background(), CreateUser{})
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("handles JSON unmarshaling from map payload", func(t *testing.T) {
		t.Parallel()

		var capturedCmd CreateUser
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			capturedCmd = cmd
			return nil
		})

		// Simulate payload coming from JSON deserialization
		mapPayload := map[string]any{
			"UserID": "456",
			"Email":  "user@example.com",
		}

		err := handler.Handle(context.Background(), mapPayload)
		require.NoError(t, err)
		assert.Equal(t, "456", capturedCmd.UserID)
		assert.Equal(t, "user@example.com", capturedCmd.Email)
	})

	t.Run("handles JSON unmarshaling from byte slice", func(t *testing.T) {
		t.Parallel()

		var capturedCmd CreateUser
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			capturedCmd = cmd
			return nil
		})

		payload := CreateUser{UserID: "789", Email: "bytes@example.com"}
		data, err := json.Marshal(payload)
		require.NoError(t, err)

		err = handler.Handle(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, payload, capturedCmd)
	})

	t.Run("returns error for invalid payload type", func(t *testing.T) {
		t.Parallel()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		err := handler.Handle(context.Background(), "invalid-payload")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command CreateUser")
		assert.Contains(t, err.Error(), "unsupported payload type")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			<-ctx.Done()
			return ctx.Err()
		})

		err := handler.Handle(ctx, CreateUser{})
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestNewHandler(t *testing.T) {
	t.Parallel()

	t.Run("creates handler with explicit command name", func(t *testing.T) {
		t.Parallel()

		handler := command.NewHandler("user.create", func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		assert.Equal(t, "user.create", handler.CommandName())
	})

	t.Run("allows namespaced command names", func(t *testing.T) {
		t.Parallel()

		handler1 := command.NewHandler("billing.create.user", func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		handler2 := command.NewHandler("auth.create.user", func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		assert.Equal(t, "billing.create.user", handler1.CommandName())
		assert.Equal(t, "auth.create.user", handler2.CommandName())
	})

	t.Run("executes with type assertion", func(t *testing.T) {
		t.Parallel()

		var capturedCmd CreateUser
		handler := command.NewHandler("user.create", func(ctx context.Context, cmd CreateUser) error {
			capturedCmd = cmd
			return nil
		})

		payload := CreateUser{UserID: "999", Email: "explicit@example.com"}
		err := handler.Handle(context.Background(), payload)

		require.NoError(t, err)
		assert.Equal(t, payload, capturedCmd)
	})
}

func TestHandlerConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("handler is safe for concurrent use", func(t *testing.T) {
		t.Parallel()

		var counter atomic.Int32
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			counter.Add(1)
			return nil
		})

		const concurrency = 100
		done := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				done <- handler.Handle(context.Background(), CreateUser{})
			}()
		}

		for i := 0; i < concurrency; i++ {
			err := <-done
			require.NoError(t, err)
		}

		assert.Equal(t, int32(concurrency), counter.Load())
	})
}
