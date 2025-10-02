package event_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChannelBus(t *testing.T) {
	t.Parallel()

	t.Run("creates bus with default buffer size", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		require.NotNil(t, bus)
		defer bus.Close()

		// Verify channel is functional
		events := bus.Events()
		require.NotNil(t, events)
	})

	t.Run("creates bus with custom buffer size", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(10))
		require.NotNil(t, bus)
		defer bus.Close()

		events := bus.Events()
		require.NotNil(t, events)
	})

	t.Run("creates bus with custom logger", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		bus := event.NewChannelBus(event.WithChannelLogger(logger))
		require.NotNil(t, bus)
		defer bus.Close()
	})

	t.Run("ignores zero or negative buffer size", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(0))
		require.NotNil(t, bus)
		defer bus.Close()

		// Should still be functional with default buffer
		ctx := context.Background()
		err := bus.Publish(ctx, []byte("test"))
		require.NoError(t, err)
	})

	t.Run("ignores nil logger", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithChannelLogger(nil))
		require.NotNil(t, bus)
		defer bus.Close()

		// Should still be functional
		ctx := context.Background()
		err := bus.Publish(ctx, []byte("test"))
		require.NoError(t, err)
	})
}

func TestChannelBus_Publish_BasicPublish(t *testing.T) {
	t.Parallel()

	t.Run("publishes single event successfully", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		ctx := context.Background()
		data := []byte("test event data")

		err := bus.Publish(ctx, data)
		require.NoError(t, err)

		// Consume the event
		events := bus.Events()
		select {
		case received := <-events:
			assert.Equal(t, data, received)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("publishes multiple events in order", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		ctx := context.Background()
		events := bus.Events()

		// Publish multiple events
		data := [][]byte{
			[]byte("event1"),
			[]byte("event2"),
			[]byte("event3"),
		}

		for _, d := range data {
			err := bus.Publish(ctx, d)
			require.NoError(t, err)
		}

		// Verify order
		for i, expected := range data {
			select {
			case received := <-events:
				assert.Equal(t, expected, received, "event %d mismatch", i)
			case <-time.After(time.Second):
				t.Fatalf("timeout waiting for event %d", i)
			}
		}
	})

	t.Run("publishes empty data", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		ctx := context.Background()
		err := bus.Publish(ctx, []byte{})
		require.NoError(t, err)

		events := bus.Events()
		select {
		case received := <-events:
			assert.Empty(t, received)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("publishes nil data", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		ctx := context.Background()
		err := bus.Publish(ctx, nil)
		require.NoError(t, err)

		events := bus.Events()
		select {
		case received := <-events:
			assert.Nil(t, received)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	})
}

func TestChannelBus_Publish_ConcurrentPublishers(t *testing.T) {
	t.Parallel()

	t.Run("handles concurrent publishers safely", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(1000))
		defer bus.Close()

		ctx := context.Background()
		numPublishers := 10
		eventsPerPublisher := 100

		var wg sync.WaitGroup
		wg.Add(numPublishers)

		// Launch concurrent publishers
		for i := 0; i < numPublishers; i++ {
			go func(publisherID int) {
				defer wg.Done()
				for j := 0; j < eventsPerPublisher; j++ {
					data := []byte{byte(publisherID), byte(j)}
					err := bus.Publish(ctx, data)
					assert.NoError(t, err)
				}
			}(i)
		}

		// Consume all events
		expectedTotal := numPublishers * eventsPerPublisher
		received := make([][]byte, 0, expectedTotal)

		done := make(chan struct{})
		go func() {
			events := bus.Events()
			for i := 0; i < expectedTotal; i++ {
				select {
				case data := <-events:
					received = append(received, data)
				case <-time.After(5 * time.Second):
					t.Error("timeout waiting for events")
					return
				}
			}
			close(done)
		}()

		wg.Wait()
		<-done

		assert.Len(t, received, expectedTotal)
	})

	t.Run("handles concurrent publish and consume", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(50))
		defer bus.Close()

		ctx := context.Background()
		numEvents := 500

		var publishWg, consumeWg sync.WaitGroup
		publishWg.Add(1)
		consumeWg.Add(1)

		// Publisher
		go func() {
			defer publishWg.Done()
			for i := 0; i < numEvents; i++ {
				data := []byte{byte(i)}
				err := bus.Publish(ctx, data)
				assert.NoError(t, err)
			}
		}()

		// Consumer
		received := 0
		go func() {
			defer consumeWg.Done()
			events := bus.Events()
			for i := 0; i < numEvents; i++ {
				select {
				case <-events:
					received++
				case <-time.After(5 * time.Second):
					return
				}
			}
		}()

		publishWg.Wait()
		consumeWg.Wait()

		assert.Equal(t, numEvents, received)
	})
}

func TestChannelBus_Publish_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("returns error when context is cancelled and channel would block", func(t *testing.T) {
		t.Parallel()

		// Use buffer size of 1 so we can easily fill it
		bus := event.NewChannelBus(event.WithBufferSize(1))
		defer bus.Close()

		// Fill the buffer
		bgCtx := context.Background()
		err := bus.Publish(bgCtx, []byte("fill"))
		require.NoError(t, err)

		// Cancel context before attempting to publish (which will block)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Should return context.Canceled since buffer is full
		err = bus.Publish(ctx, []byte("test"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns error when context times out", func(t *testing.T) {
		t.Parallel()

		// Use buffer size of 1 and fill it to force blocking
		bus := event.NewChannelBus(event.WithBufferSize(1))
		defer bus.Close()

		// Fill the buffer
		bgCtx := context.Background()
		err := bus.Publish(bgCtx, []byte("fill"))
		require.NoError(t, err)

		// Now publish with a timeout that will expire while blocked
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err = bus.Publish(ctx, []byte("test"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("cancels blocked publish", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(1))
		defer bus.Close()

		// Fill the buffer
		ctx := context.Background()
		err := bus.Publish(ctx, []byte("fill"))
		require.NoError(t, err)

		// Try to publish with cancellable context
		cancelCtx, cancel := context.WithCancel(context.Background())

		// Cancel after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		err = bus.Publish(cancelCtx, []byte("blocked"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestChannelBus_Close(t *testing.T) {
	t.Parallel()

	t.Run("closes bus successfully", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()

		err := bus.Close()
		assert.NoError(t, err)
	})

	t.Run("closes events channel", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		events := bus.Events()

		err := bus.Close()
		require.NoError(t, err)

		// Channel should be closed
		_, ok := <-events
		assert.False(t, ok, "channel should be closed")
	})

	t.Run("returns error on double close", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()

		err := bus.Close()
		require.NoError(t, err)

		err = bus.Close()
		assert.Error(t, err)
		assert.ErrorIs(t, err, event.ErrChannelBusClosed)
	})

	t.Run("publish after close returns error", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()

		err := bus.Close()
		require.NoError(t, err)

		ctx := context.Background()
		err = bus.Publish(ctx, []byte("test"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, event.ErrChannelBusClosed)
	})

	t.Run("concurrent close is safe", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()

		var wg sync.WaitGroup
		numClosers := 10
		wg.Add(numClosers)

		// Only one should succeed
		errors := make([]error, numClosers)
		for i := 0; i < numClosers; i++ {
			go func(idx int) {
				defer wg.Done()
				errors[idx] = bus.Close()
			}(i)
		}

		wg.Wait()

		// Count successes
		successes := 0
		failures := 0
		for _, err := range errors {
			if err == nil {
				successes++
			} else {
				failures++
				assert.ErrorIs(t, err, event.ErrChannelBusClosed)
			}
		}

		assert.Equal(t, 1, successes, "exactly one close should succeed")
		assert.Equal(t, numClosers-1, failures, "remaining closes should fail")
	})
}

func TestChannelBus_BlockingBehavior(t *testing.T) {
	t.Parallel()

	t.Run("publisher blocks when buffer is full", func(t *testing.T) {
		t.Parallel()

		bufferSize := 5
		bus := event.NewChannelBus(event.WithBufferSize(bufferSize))
		defer bus.Close()

		ctx := context.Background()

		// Fill the buffer completely
		for i := 0; i < bufferSize; i++ {
			err := bus.Publish(ctx, []byte{byte(i)})
			require.NoError(t, err)
		}

		// Next publish should block
		published := make(chan struct{})
		go func() {
			err := bus.Publish(ctx, []byte("blocked"))
			assert.NoError(t, err)
			close(published)
		}()

		// Verify publish is blocked
		select {
		case <-published:
			t.Fatal("publish should be blocked")
		case <-time.After(100 * time.Millisecond):
			// Expected - publish is blocked
		}

		// Consume one event to unblock
		events := bus.Events()
		<-events

		// Now publish should complete
		select {
		case <-published:
			// Expected
		case <-time.After(time.Second):
			t.Fatal("publish should have completed after consuming event")
		}
	})

	t.Run("multiple publishers unblock as consumer reads", func(t *testing.T) {
		t.Parallel()

		bufferSize := 2
		bus := event.NewChannelBus(event.WithBufferSize(bufferSize))
		defer bus.Close()

		ctx := context.Background()

		// Fill buffer
		for i := 0; i < bufferSize; i++ {
			err := bus.Publish(ctx, []byte{byte(i)})
			require.NoError(t, err)
		}

		// Start multiple blocked publishers
		numBlockedPublishers := 5
		publishComplete := make(chan int, numBlockedPublishers)

		for i := 0; i < numBlockedPublishers; i++ {
			go func(id int) {
				err := bus.Publish(ctx, []byte{byte(id + 100)})
				assert.NoError(t, err)
				publishComplete <- id
			}(i)
		}

		// Verify all are blocked
		time.Sleep(100 * time.Millisecond)
		select {
		case <-publishComplete:
			t.Fatal("publishers should be blocked")
		default:
			// Expected
		}

		// Consume events to unblock publishers
		events := bus.Events()
		for i := 0; i < bufferSize+numBlockedPublishers; i++ {
			select {
			case <-events:
				// Event consumed
			case <-time.After(time.Second):
				t.Fatal("timeout consuming events")
			}
		}

		// All publishers should complete
		for i := 0; i < numBlockedPublishers; i++ {
			select {
			case <-publishComplete:
				// Expected
			case <-time.After(time.Second):
				t.Fatalf("publisher %d did not complete", i)
			}
		}
	})
}

func TestChannelBus_Events(t *testing.T) {
	t.Parallel()

	t.Run("returns read-only channel", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		events := bus.Events()
		require.NotNil(t, events)

		// Verify it's usable as receive-only channel
		ctx := context.Background()
		err := bus.Publish(ctx, []byte("test"))
		require.NoError(t, err)

		select {
		case data := <-events:
			assert.Equal(t, []byte("test"), data)
		case <-time.After(time.Second):
			t.Fatal("timeout receiving event")
		}
	})

	t.Run("multiple calls return same channel", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		events1 := bus.Events()
		events2 := bus.Events()

		// Should be the same channel
		assert.Equal(t, events1, events2)
	})

	t.Run("multiple consumers receive same events", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		events := bus.Events()

		// Start two consumers - only one will receive each event
		// (channels don't broadcast, they distribute)
		ctx := context.Background()
		numEvents := 10

		received1 := make([][]byte, 0)
		received2 := make([][]byte, 0)
		var mu sync.Mutex

		var wg sync.WaitGroup
		wg.Add(2)

		// Consumer 1
		go func() {
			defer wg.Done()
			for i := 0; i < numEvents/2; i++ {
				select {
				case data := <-events:
					mu.Lock()
					received1 = append(received1, data)
					mu.Unlock()
				case <-time.After(2 * time.Second):
					return
				}
			}
		}()

		// Consumer 2
		go func() {
			defer wg.Done()
			for i := 0; i < numEvents/2; i++ {
				select {
				case data := <-events:
					mu.Lock()
					received2 = append(received2, data)
					mu.Unlock()
				case <-time.After(2 * time.Second):
					return
				}
			}
		}()

		// Publish events
		for i := 0; i < numEvents; i++ {
			err := bus.Publish(ctx, []byte{byte(i)})
			require.NoError(t, err)
		}

		wg.Wait()

		// Both consumers should have received events
		// Total should equal numEvents
		mu.Lock()
		total := len(received1) + len(received2)
		mu.Unlock()

		assert.Equal(t, numEvents, total)
	})
}

func TestChannelBus_Integration(t *testing.T) {
	t.Parallel()

	t.Run("realistic publish-consume workflow", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		bus := event.NewChannelBus(
			event.WithBufferSize(50),
			event.WithChannelLogger(logger),
		)
		defer bus.Close()

		ctx := context.Background()
		numEvents := 100
		processed := make([]int, 0, numEvents)
		var mu sync.Mutex

		// Start consumer
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			events := bus.Events()
			for data := range events {
				mu.Lock()
				processed = append(processed, int(data[0]))
				mu.Unlock()
			}
		}()

		// Publish events
		for i := 0; i < numEvents; i++ {
			err := bus.Publish(ctx, []byte{byte(i)})
			require.NoError(t, err)
		}

		// Close and wait for consumer
		time.Sleep(100 * time.Millisecond) // Allow processing
		err := bus.Close()
		require.NoError(t, err)

		wg.Wait()

		mu.Lock()
		assert.Len(t, processed, numEvents)
		mu.Unlock()
	})

	t.Run("graceful shutdown with pending events", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(100))
		defer bus.Close()

		ctx := context.Background()
		numEvents := 50

		// Publish events
		for i := 0; i < numEvents; i++ {
			err := bus.Publish(ctx, []byte{byte(i)})
			require.NoError(t, err)
		}

		// Close bus
		err := bus.Close()
		require.NoError(t, err)

		// Consumer can still drain the channel
		events := bus.Events()
		drained := 0
		for range events {
			drained++
		}

		assert.Equal(t, numEvents, drained, "all pending events should be drainable")
	})
}
