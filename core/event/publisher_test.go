package event_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublisher_PublishVariousTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		payload       any
		expectedName  string
		validateEvent func(t *testing.T, evt event.Event)
	}{
		{
			name:         "struct payload",
			payload:      UserCreated{UserID: "123", Email: "test@example.com"},
			expectedName: "UserCreated",
			validateEvent: func(t *testing.T, evt event.Event) {
				assert.Equal(t, "UserCreated", evt.Name)

				// Verify payload can be extracted
				payloadBytes, err := json.Marshal(evt.Payload)
				require.NoError(t, err)

				var user UserCreated
				err = json.Unmarshal(payloadBytes, &user)
				require.NoError(t, err)
				assert.Equal(t, "123", user.UserID)
				assert.Equal(t, "test@example.com", user.Email)
			},
		},
		{
			name:         "pointer to struct",
			payload:      &OrderPlaced{OrderID: "ord-456", UserID: "u-123", Amount: 99.99, Timestamp: time.Now()},
			expectedName: "OrderPlaced",
			validateEvent: func(t *testing.T, evt event.Event) {
				assert.Equal(t, "OrderPlaced", evt.Name)

				payloadBytes, err := json.Marshal(evt.Payload)
				require.NoError(t, err)

				var order OrderPlaced
				err = json.Unmarshal(payloadBytes, &order)
				require.NoError(t, err)
				assert.Equal(t, "ord-456", order.OrderID)
				assert.Equal(t, 99.99, order.Amount)
			},
		},
		{
			name: "nested struct",
			payload: NestedPayload{ID: "nested-1", Data: struct {
				Name  string
				Items []string
			}{Name: "test", Items: []string{"a", "b"}}},
			expectedName: "NestedPayload",
			validateEvent: func(t *testing.T, evt event.Event) {
				assert.Equal(t, "NestedPayload", evt.Name)

				payloadBytes, err := json.Marshal(evt.Payload)
				require.NoError(t, err)

				var nested NestedPayload
				err = json.Unmarshal(payloadBytes, &nested)
				require.NoError(t, err)
				assert.Equal(t, "nested-1", nested.ID)
				assert.Equal(t, "test", nested.Data.Name)
				assert.Equal(t, []string{"a", "b"}, nested.Data.Items)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bus := event.NewChannelBus()
			defer bus.Close()

			publisher := event.NewPublisher(bus)

			ctx := context.Background()
			err := publisher.Publish(ctx, tt.payload)
			require.NoError(t, err)

			// Receive and unmarshal the event
			select {
			case data := <-bus.Events():
				var evt event.Event
				err = json.Unmarshal(data, &evt)
				require.NoError(t, err)

				// Validate basic event structure
				assert.NotEmpty(t, evt.ID)
				assert.Equal(t, tt.expectedName, evt.Name)
				assert.NotZero(t, evt.CreatedAt)
				assert.NotNil(t, evt.Payload)

				// Run custom validation
				tt.validateEvent(t, evt)

			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for event")
			}
		})
	}
}

func TestPublisher_PublishWithChannelBus(t *testing.T) {
	t.Parallel()

	t.Run("event contains all required fields", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		payload := UserCreated{UserID: "u-123", Email: "user@example.com"}
		ctx := context.Background()

		err := publisher.Publish(ctx, payload)
		require.NoError(t, err)

		// Receive the event
		select {
		case data := <-bus.Events():
			var evt event.Event
			err = json.Unmarshal(data, &evt)
			require.NoError(t, err)

			// Verify Event structure
			assert.NotEmpty(t, evt.ID, "event ID should not be empty")
			assert.Equal(t, "UserCreated", evt.Name, "event name should match payload type")
			assert.NotZero(t, evt.CreatedAt, "event CreatedAt should be set")
			assert.NotNil(t, evt.Payload, "event Payload should not be nil")

			// Verify the event was properly marshaled to JSON
			assert.True(t, json.Valid(data), "published data should be valid JSON")

		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("multiple events maintain order", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(10))
		defer bus.Close()

		publisher := event.NewPublisher(bus)
		ctx := context.Background()

		// Publish multiple events
		events := []UserCreated{
			{UserID: "1", Email: "user1@example.com"},
			{UserID: "2", Email: "user2@example.com"},
			{UserID: "3", Email: "user3@example.com"},
		}

		for _, e := range events {
			err := publisher.Publish(ctx, e)
			require.NoError(t, err)
		}

		// Receive events and verify order
		for i := 0; i < 3; i++ {
			select {
			case data := <-bus.Events():
				var evt event.Event
				err := json.Unmarshal(data, &evt)
				require.NoError(t, err)

				payloadBytes, err := json.Marshal(evt.Payload)
				require.NoError(t, err)

				var user UserCreated
				err = json.Unmarshal(payloadBytes, &user)
				require.NoError(t, err)

				expected := events[i]
				assert.Equal(t, expected.UserID, user.UserID)
				assert.Equal(t, expected.Email, user.Email)

			case <-time.After(1 * time.Second):
				t.Fatalf("timeout waiting for event %d", i)
			}
		}
	})
}

func TestPublisher_WithPublisherLogger(t *testing.T) {
	t.Parallel()

	t.Run("uses custom logger", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus, event.WithPublisherLogger(logger))

		ctx := context.Background()
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		// Consume the event
		<-bus.Events()

		// Verify logger was used (should contain debug message)
		logOutput := buf.String()
		assert.Contains(t, logOutput, "event published")
		assert.Contains(t, logOutput, "event_id")
		assert.Contains(t, logOutput, "event_name")
	})

	t.Run("uses discard logger by default", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		ctx := context.Background()
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		// Just verify it works without logging errors
		select {
		case <-bus.Events():
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("nil logger option is ignored", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		// Should not panic with nil logger
		publisher := event.NewPublisher(bus, event.WithPublisherLogger(nil))

		ctx := context.Background()
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		select {
		case <-bus.Events():
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("logs marshal errors", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus, event.WithPublisherLogger(logger))

		ctx := context.Background()
		err := publisher.Publish(ctx, UnmarshalableType{Channel: make(chan int)})
		require.Error(t, err)

		// Verify error was logged
		logOutput := buf.String()
		assert.Contains(t, logOutput, "failed to marshal event")
		assert.Contains(t, logOutput, "error")
	})

	t.Run("logs publish errors", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

		bus := event.NewChannelBus()
		bus.Close() // Close bus to trigger publish error

		publisher := event.NewPublisher(bus, event.WithPublisherLogger(logger))

		ctx := context.Background()
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.Error(t, err)

		// Verify error was logged
		logOutput := buf.String()
		assert.Contains(t, logOutput, "failed to publish event")
		assert.Contains(t, logOutput, "error")
	})
}

func TestPublisher_MarshalErrors(t *testing.T) {
	t.Parallel()

	t.Run("returns error for unmarshalable type", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		payload := UnmarshalableType{Channel: make(chan int)}
		ctx := context.Background()

		err := publisher.Publish(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json")

		// Verify no event was published
		select {
		case <-bus.Events():
			t.Fatal("no event should have been published")
		case <-time.After(100 * time.Millisecond):
			// Expected - no event published
		}
	})

	t.Run("returns error for function type", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		// Functions cannot be marshaled
		payload := struct {
			Fn func()
		}{
			Fn: func() {},
		}

		ctx := context.Background()
		err := publisher.Publish(ctx, payload)
		require.Error(t, err)
	})
}

func TestPublisher_BusErrors(t *testing.T) {
	t.Parallel()

	t.Run("returns error when bus is closed", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		bus.Close()

		publisher := event.NewPublisher(bus)

		ctx := context.Background()
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.Error(t, err)
		assert.Equal(t, event.ErrChannelBusClosed, err)
	})

	t.Run("returns error when publishing to closed bus multiple times", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		bus.Close()

		publisher := event.NewPublisher(bus)
		ctx := context.Background()

		// Multiple publish attempts should all fail
		for i := 0; i < 3; i++ {
			err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
			require.Error(t, err)
			assert.Equal(t, event.ErrChannelBusClosed, err)
		}
	})
}

func TestPublisher_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("passes context to bus", func(t *testing.T) {
		t.Parallel()

		// The publisher passes the context to the bus.Publish method
		// Context cancellation behavior is tested in the bus tests
		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Publish with valid context should succeed
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		// Verify event was published
		select {
		case data := <-bus.Events():
			var evt event.Event
			err = json.Unmarshal(data, &evt)
			require.NoError(t, err)
			assert.Equal(t, "UserCreated", evt.Name)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("succeeds with timeout context", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		select {
		case <-bus.Events():
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})

	t.Run("context values are passed to logger", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus, event.WithPublisherLogger(logger))

		// Create context with value
		type contextKey string
		ctx := context.WithValue(context.Background(), contextKey("request_id"), "test-123")

		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		// Consume event
		<-bus.Events()

		// Logger should have been called with context
		logOutput := buf.String()
		assert.Contains(t, logOutput, "event published")
	})
}

func TestPublisher_ConcurrentPublishing(t *testing.T) {
	t.Parallel()

	t.Run("multiple goroutines can publish concurrently", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(100))
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		const numGoroutines = 10
		const eventsPerGoroutine = 10
		const totalEvents = numGoroutines * eventsPerGoroutine

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Launch concurrent publishers
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				defer wg.Done()

				ctx := context.Background()
				for j := 0; j < eventsPerGoroutine; j++ {
					err := publisher.Publish(ctx, UserCreated{
						UserID: string(rune(workerID*100 + j)),
						Email:  "concurrent@example.com",
					})
					require.NoError(t, err)
				}
			}(i)
		}

		// Collect all events
		received := 0
		done := make(chan struct{})

		go func() {
			for range bus.Events() {
				received++
				if received == totalEvents {
					close(done)
					return
				}
			}
		}()

		// Wait for publishers to finish
		wg.Wait()

		// Wait for all events to be received
		select {
		case <-done:
			assert.Equal(t, totalEvents, received)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for events, received %d out of %d", received, totalEvents)
		}
	})

	t.Run("concurrent publishing with different payload types", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus(event.WithBufferSize(100))
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		var wg sync.WaitGroup
		wg.Add(3)

		// Publisher 1: UserCreated
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < 10; i++ {
				err := publisher.Publish(ctx, UserCreated{UserID: "user", Email: "test@example.com"})
				require.NoError(t, err)
			}
		}()

		// Publisher 2: OrderPlaced
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < 10; i++ {
				err := publisher.Publish(ctx, OrderPlaced{OrderID: "order", UserID: "user", Amount: 99.99})
				require.NoError(t, err)
			}
		}()

		// Publisher 3: NestedPayload
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < 10; i++ {
				err := publisher.Publish(ctx, NestedPayload{ID: "nested"})
				require.NoError(t, err)
			}
		}()

		// Collect events and count by type
		eventCounts := make(map[string]int)
		var mu sync.Mutex

		go func() {
			for data := range bus.Events() {
				var evt event.Event
				if err := json.Unmarshal(data, &evt); err == nil {
					mu.Lock()
					eventCounts[evt.Name]++
					mu.Unlock()
				}
			}
		}()

		wg.Wait()

		// Give time for events to be processed
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.Equal(t, 10, eventCounts["UserCreated"])
		assert.Equal(t, 10, eventCounts["OrderPlaced"])
		assert.Equal(t, 10, eventCounts["NestedPayload"])
	})

	t.Run("publisher is safe for concurrent use with logger", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		bus := event.NewChannelBus(event.WithBufferSize(50))
		defer bus.Close()

		publisher := event.NewPublisher(bus, event.WithPublisherLogger(logger))

		var wg sync.WaitGroup
		wg.Add(5)

		// Launch concurrent publishers with logging
		for i := 0; i < 5; i++ {
			go func() {
				defer wg.Done()
				ctx := context.Background()
				for j := 0; j < 10; j++ {
					err := publisher.Publish(ctx, UserCreated{UserID: "test", Email: "test@example.com"})
					require.NoError(t, err)
				}
			}()
		}

		// Consume events
		go func() {
			for range bus.Events() {
				// Just consume
			}
		}()

		wg.Wait()

		// Verify no race conditions in logging
		logOutput := buf.String()
		assert.NotEmpty(t, logOutput)
	})
}

func TestPublisher_NilLogger(t *testing.T) {
	t.Parallel()

	t.Run("handles discard logger", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
		publisher := event.NewPublisher(bus, event.WithPublisherLogger(discardLogger))

		ctx := context.Background()
		err := publisher.Publish(ctx, UserCreated{UserID: "123", Email: "test@example.com"})
		require.NoError(t, err)

		select {
		case data := <-bus.Events():
			var evt event.Event
			err = json.Unmarshal(data, &evt)
			require.NoError(t, err)
			assert.Equal(t, "UserCreated", evt.Name)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for event")
		}
	})
}
