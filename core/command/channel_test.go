package command_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChannelBus(t *testing.T) {
	t.Parallel()

	t.Run("creates channel bus with default buffer size", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		assert.NotNil(t, bus)
		assert.NotNil(t, bus.Commands())
	})

	t.Run("creates channel bus with custom buffer size", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(50))
		defer bus.Close()

		assert.NotNil(t, bus)
	})

	t.Run("ignores zero or negative buffer size", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(0))
		defer bus.Close()

		// Should still work with default size
		assert.NotNil(t, bus)
	})
}

func TestChannelBusPublish(t *testing.T) {
	t.Parallel()

	t.Run("publishes command successfully", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		data := []byte(`{"test": "data"}`)
		err := bus.Publish(context.Background(), data)

		require.NoError(t, err)
	})

	t.Run("receives published command", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		data := []byte(`{"test": "data"}`)
		err := bus.Publish(context.Background(), data)
		require.NoError(t, err)

		received := <-bus.Commands()
		assert.Equal(t, data, received)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(1))
		defer bus.Close()

		// Fill the buffer first
		err := bus.Publish(context.Background(), []byte("fill"))
		require.NoError(t, err)

		// Now publish with cancelled context should fail
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = bus.Publish(ctx, []byte("test"))
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns error when bus is closed", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		err := bus.Close()
		require.NoError(t, err)

		err = bus.Publish(context.Background(), []byte("test"))
		assert.ErrorIs(t, err, command.ErrChannelBusClosed)
	})

	t.Run("handles concurrent publishers", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(100))
		defer bus.Close()

		const publishers = 10
		const messagesPerPublisher = 10
		var wg sync.WaitGroup
		wg.Add(publishers)

		for i := 0; i < publishers; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerPublisher; j++ {
					data := []byte("message")
					err := bus.Publish(context.Background(), data)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()

		// Drain the channel and count messages
		count := 0
		timeout := time.After(1 * time.Second)
	loop:
		for {
			select {
			case <-bus.Commands():
				count++
				if count == publishers*messagesPerPublisher {
					break loop
				}
			case <-timeout:
				break loop
			}
		}

		assert.Equal(t, publishers*messagesPerPublisher, count)
	})
}

func TestChannelBusCommands(t *testing.T) {
	t.Parallel()

	t.Run("returns readable channel", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		commands := bus.Commands()
		assert.NotNil(t, commands)
	})

	t.Run("channel closes when bus closes", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		commands := bus.Commands()

		err := bus.Close()
		require.NoError(t, err)

		// Reading from closed channel should return zero value and false
		_, ok := <-commands
		assert.False(t, ok)
	})

	t.Run("multiple readers can consume from channel", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(100))
		defer bus.Close()

		const numMessages = 10
		receivedChan := make(chan []byte, numMessages)

		// Start multiple readers (only one will receive each message)
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for data := range bus.Commands() {
					receivedChan <- data
				}
			}()
		}

		// Publish messages
		for i := 0; i < numMessages; i++ {
			err := bus.Publish(context.Background(), []byte("message"))
			require.NoError(t, err)
		}

		// Close bus to stop readers
		bus.Close()
		wg.Wait()
		close(receivedChan)

		// Count received messages
		count := 0
		for range receivedChan {
			count++
		}
		assert.Equal(t, numMessages, count)
	})
}

func TestChannelBusClose(t *testing.T) {
	t.Parallel()

	t.Run("closes successfully", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		err := bus.Close()
		require.NoError(t, err)
	})

	t.Run("returns error on double close", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		err := bus.Close()
		require.NoError(t, err)

		err = bus.Close()
		assert.ErrorIs(t, err, command.ErrChannelBusClosed)
	})

	t.Run("close is safe to call concurrently", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()

		var wg sync.WaitGroup
		errors := make(chan error, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errors <- bus.Close()
			}()
		}

		wg.Wait()
		close(errors)

		successCount := 0
		errorCount := 0
		for err := range errors {
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
		}

		assert.Equal(t, 1, successCount, "exactly one close should succeed")
		assert.Equal(t, 4, errorCount, "other closes should return error")
	})
}

func TestChannelBusBuffering(t *testing.T) {
	t.Parallel()

	t.Run("buffers commands up to buffer size", func(t *testing.T) {
		t.Parallel()

		const bufferSize = 5
		bus := command.NewChannelBus(command.WithBufferSize(bufferSize))
		defer bus.Close()

		// Publish up to buffer size without blocking
		for i := 0; i < bufferSize; i++ {
			err := bus.Publish(context.Background(), []byte("message"))
			require.NoError(t, err)
		}

		// All messages should be buffered
		for i := 0; i < bufferSize; i++ {
			select {
			case <-bus.Commands():
			case <-time.After(10 * time.Millisecond):
				t.Fatal("expected buffered message")
			}
		}
	})

	t.Run("blocks when buffer is full", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(1))
		defer bus.Close()

		// Fill the buffer
		err := bus.Publish(context.Background(), []byte("message1"))
		require.NoError(t, err)

		// Next publish should block
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err = bus.Publish(ctx, []byte("message2"))
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
