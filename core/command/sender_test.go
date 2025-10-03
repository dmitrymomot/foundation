package command_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCommandBus struct {
	mock.Mock
}

func (m *MockCommandBus) Publish(ctx context.Context, data []byte) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

type SendTestCommand struct {
	ID   string
	Name string
}

func TestNewSender(t *testing.T) {
	t.Parallel()

	t.Run("creates sender with command bus", func(t *testing.T) {
		t.Parallel()

		bus := &MockCommandBus{}
		sender := command.NewSender(bus)

		assert.NotNil(t, sender)
	})
}

func TestSenderSend(t *testing.T) {
	t.Parallel()

	t.Run("sends command successfully", func(t *testing.T) {
		t.Parallel()

		mockBus := &MockCommandBus{}
		mockBus.On("Publish", mock.Anything, mock.Anything).Return(nil)

		sender := command.NewSender(mockBus)
		payload := SendTestCommand{ID: "123", Name: "test"}

		err := sender.Send(context.Background(), payload)
		require.NoError(t, err)

		mockBus.AssertExpectations(t)
		mockBus.AssertNumberOfCalls(t, "Publish", 1)
	})

	t.Run("creates command with correct metadata", func(t *testing.T) {
		t.Parallel()

		var capturedData []byte
		mockBus := &MockCommandBus{}
		mockBus.On("Publish", mock.Anything, mock.MatchedBy(func(data []byte) bool {
			capturedData = data
			return true
		})).Return(nil)

		sender := command.NewSender(mockBus)
		payload := SendTestCommand{ID: "456", Name: "test-command"}

		err := sender.Send(context.Background(), payload)
		require.NoError(t, err)

		var cmd command.Command
		err = json.Unmarshal(capturedData, &cmd)
		require.NoError(t, err)

		assert.NotEmpty(t, cmd.ID)
		assert.Equal(t, "SendTestCommand", cmd.Name)
		assert.NotZero(t, cmd.CreatedAt)
	})

	t.Run("propagates publish errors", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("publish failed")
		mockBus := &MockCommandBus{}
		mockBus.On("Publish", mock.Anything, mock.Anything).Return(expectedErr)

		sender := command.NewSender(mockBus)
		err := sender.Send(context.Background(), SendTestCommand{})

		assert.ErrorIs(t, err, expectedErr)
		mockBus.AssertExpectations(t)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mockBus := &MockCommandBus{}
		mockBus.On("Publish", ctx, mock.Anything).Return(context.Canceled)

		sender := command.NewSender(mockBus)
		err := sender.Send(ctx, SendTestCommand{})

		assert.ErrorIs(t, err, context.Canceled)
		mockBus.AssertExpectations(t)
	})

	t.Run("is safe for concurrent use", func(t *testing.T) {
		t.Parallel()

		mockBus := &MockCommandBus{}
		mockBus.On("Publish", mock.Anything, mock.Anything).Return(nil)

		sender := command.NewSender(mockBus)

		const concurrency = 50
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				defer wg.Done()
				payload := SendTestCommand{ID: string(rune(id))}
				err := sender.Send(context.Background(), payload)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
		mockBus.AssertNumberOfCalls(t, "Publish", concurrency)
	})
}

func TestSenderWithRealBus(t *testing.T) {
	t.Parallel()

	t.Run("integrates with channel bus", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		sender := command.NewSender(bus)
		payload := SendTestCommand{ID: "integration-test", Name: "test"}

		err := sender.Send(context.Background(), payload)
		require.NoError(t, err)

		// Verify command was published to bus
		data := <-bus.Commands()
		var cmd command.Command
		err = json.Unmarshal(data, &cmd)
		require.NoError(t, err)

		assert.Equal(t, "SendTestCommand", cmd.Name)
		// Payload comes back as map[string]any from JSON unmarshaling
		payloadMap, ok := cmd.Payload.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "integration-test", payloadMap["ID"])
	})
}
