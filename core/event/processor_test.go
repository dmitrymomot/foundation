package event_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_StartStop_Lifecycle(t *testing.T) {
	t.Parallel()

	t.Run("start and stop cleanly", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		var handledCount atomic.Int32
		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			handledCount.Add(1)
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		ctx := context.Background()

		// Start processor in background
		errCh := make(chan error, 1)
		go func() {
			errCh <- processor.Start(ctx)
		}()

		// Give processor time to start
		time.Sleep(50 * time.Millisecond)

		// Verify processor is running
		stats := processor.Stats()
		assert.True(t, stats.IsRunning)

		// Publish event
		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		// Wait for event to be processed
		require.Eventually(t, func() bool {
			stats := processor.Stats()
			t.Logf("Stats: Processed=%d, Failed=%d, Active=%d, HandledCount=%d",
				stats.EventsProcessed, stats.EventsFailed, stats.ActiveEvents, handledCount.Load())
			return handledCount.Load() == 1
		}, 2*time.Second, 50*time.Millisecond, "event should be processed")

		// Stop processor
		require.NoError(t, processor.Stop())

		// Verify processor stopped
		stats = processor.Stats()
		assert.False(t, stats.IsRunning)

		// Start() should return context.Canceled
		err := <-errCh
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestProcessor_DoubleStart(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx := context.Background()

	// Start processor
	go func() {
		_ = processor.Start(ctx)
	}()

	// Give processor time to start
	time.Sleep(50 * time.Millisecond)

	// Attempt to start again
	err := processor.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, event.ErrProcessorAlreadyStarted))

	// Cleanup
	require.NoError(t, processor.Stop())
}

func TestProcessor_StopBeforeStart(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	// Attempt to stop without starting
	err := processor.Stop()
	require.Error(t, err)
	assert.True(t, errors.Is(err, event.ErrProcessorNotStarted))
}

func TestProcessor_NoEventSource(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	processor := event.NewProcessor(
		event.WithHandler(handler),
		// No event source
	)

	ctx := context.Background()
	err := processor.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, event.ErrEventSourceNil))
}

func TestProcessor_NoHandlers(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		// No handlers
	)

	ctx := context.Background()
	err := processor.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, event.ErrNoHandlers))
}

func TestProcessor_NoHandlers_WithFallback(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	var fallbackCalled atomic.Bool
	fallback := func(ctx context.Context, evt event.Event) error {
		fallbackCalled.Store(true)
		return nil
	}

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithFallbackHandler(fallback),
		// No regular handlers
	)

	ctx := context.Background()

	// Start processor
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	publisher := event.NewPublisher(bus)
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}))

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Verify fallback was called
	assert.True(t, fallbackCalled.Load())

	require.NoError(t, processor.Stop())
}

func TestProcessor_ConcurrencyControl(t *testing.T) {
	t.Parallel()

	t.Run("limit enforced", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		maxConcurrent := 3
		var currentConcurrent atomic.Int32
		var maxObserved atomic.Int32

		// Handler that tracks concurrency
		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			current := currentConcurrent.Add(1)
			defer currentConcurrent.Add(-1)

			// Update max observed
			for {
				max := maxObserved.Load()
				if current <= max || maxObserved.CompareAndSwap(max, current) {
					break
				}
			}

			// Simulate work
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
			event.WithMaxConcurrentHandlers(maxConcurrent),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish many events
		publisher := event.NewPublisher(bus)
		for i := 0; i < 10; i++ {
			require.NoError(t, publisher.Publish(ctx, UserCreated{
				UserID: fmt.Sprintf("user-%d", i),
				Email:  fmt.Sprintf("user%d@example.com", i),
			}))
		}

		// Wait for processing
		time.Sleep(500 * time.Millisecond)

		// Verify concurrency limit was respected
		assert.LessOrEqual(t, maxObserved.Load(), int32(maxConcurrent),
			"max concurrent handlers should not exceed limit")

		require.NoError(t, processor.Stop())
	})

	t.Run("unlimited concurrency when zero", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		var currentConcurrent atomic.Int32
		var maxObserved atomic.Int32

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			current := currentConcurrent.Add(1)
			defer currentConcurrent.Add(-1)

			for {
				max := maxObserved.Load()
				if current <= max || maxObserved.CompareAndSwap(max, current) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
			event.WithMaxConcurrentHandlers(0), // Unlimited
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish many events
		publisher := event.NewPublisher(bus)
		eventCount := 20
		for i := 0; i < eventCount; i++ {
			require.NoError(t, publisher.Publish(ctx, UserCreated{
				UserID: fmt.Sprintf("user-%d", i),
				Email:  fmt.Sprintf("user%d@example.com", i),
			}))
		}

		// Wait for processing to start
		time.Sleep(100 * time.Millisecond)

		// With unlimited concurrency, we should see many concurrent handlers
		assert.Greater(t, maxObserved.Load(), int32(5),
			"should allow high concurrency with unlimited setting")

		require.NoError(t, processor.Stop())
	})
}

func TestProcessor_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	t.Run("timeout exceeded", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		// Handler that takes too long
		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			time.Sleep(5 * time.Second)
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
			event.WithShutdownTimeout(100*time.Millisecond),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish event
		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		// Give handler time to start
		time.Sleep(50 * time.Millisecond)

		// Stop should timeout
		err := processor.Stop()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown timeout exceeded")
	})

	t.Run("completes within timeout", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		// Handler that completes quickly
		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
			event.WithShutdownTimeout(1*time.Second),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish event
		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		// Give handler time to start
		time.Sleep(50 * time.Millisecond)

		// Stop should succeed
		err := processor.Stop()
		require.NoError(t, err)
	})
}

func TestProcessor_Run_ErrgroupPattern(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	var handledCount atomic.Int32
	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handledCount.Add(1)
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Use Run() method
	errCh := make(chan error, 1)
	go func() {
		errCh <- processor.Run(ctx)()
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	publisher := event.NewPublisher(bus)
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}))

	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Should return without error (context cancellation is normal shutdown)
	err := <-errCh
	require.NoError(t, err)

	// Verify event was processed
	assert.Equal(t, int32(1), handledCount.Load())
}

func TestProcessor_Stats(t *testing.T) {
	t.Parallel()

	t.Run("tracks events processed", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Initial stats
		stats := processor.Stats()
		assert.True(t, stats.IsRunning)
		assert.Equal(t, int64(0), stats.EventsProcessed)
		assert.Equal(t, int64(0), stats.EventsFailed)

		// Publish events
		publisher := event.NewPublisher(bus)
		for i := 0; i < 5; i++ {
			require.NoError(t, publisher.Publish(ctx, UserCreated{
				UserID: fmt.Sprintf("user-%d", i),
				Email:  fmt.Sprintf("user%d@example.com", i),
			}))
		}

		time.Sleep(100 * time.Millisecond)

		// Verify stats
		stats = processor.Stats()
		assert.Equal(t, int64(5), stats.EventsProcessed)
		assert.Equal(t, int64(0), stats.EventsFailed)
		assert.False(t, stats.LastActivityAt.IsZero())

		require.NoError(t, processor.Stop())
	})

	t.Run("tracks failed events", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		testErr := errors.New("handler error")
		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return testErr
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish events
		publisher := event.NewPublisher(bus)
		for i := 0; i < 3; i++ {
			require.NoError(t, publisher.Publish(ctx, UserCreated{
				UserID: fmt.Sprintf("user-%d", i),
				Email:  fmt.Sprintf("user%d@example.com", i),
			}))
		}

		time.Sleep(100 * time.Millisecond)

		// Verify stats
		stats := processor.Stats()
		assert.Equal(t, int64(0), stats.EventsProcessed)
		assert.Equal(t, int64(3), stats.EventsFailed)

		require.NoError(t, processor.Stop())
	})

	t.Run("tracks active events", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		started := make(chan struct{})
		block := make(chan struct{})

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			started <- struct{}{}
			<-block
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish event
		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		// Wait for handler to start
		<-started

		// Check active events
		stats := processor.Stats()
		assert.Equal(t, int32(1), stats.ActiveEvents)

		// Unblock handler
		close(block)

		// Wait for completion
		time.Sleep(50 * time.Millisecond)

		// Active events should be back to zero
		stats = processor.Stats()
		assert.Equal(t, int32(0), stats.ActiveEvents)

		require.NoError(t, processor.Stop())
	})
}

func TestProcessor_Healthcheck(t *testing.T) {
	t.Parallel()

	t.Run("healthy processor", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish event to create activity
		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		time.Sleep(50 * time.Millisecond)

		// Health check should pass
		err := processor.Healthcheck(ctx)
		require.NoError(t, err)

		require.NoError(t, processor.Stop())
	})

	t.Run("not running", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		ctx := context.Background()

		// Don't start processor
		err := processor.Healthcheck(ctx)
		require.Error(t, err)
		assert.True(t, errors.Is(err, event.ErrHealthcheckFailed))
		assert.True(t, errors.Is(err, event.ErrProcessorNotRunning))
	})

	t.Run("stale processor", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		// Note: lastActivityAt uses Unix() which only has second precision
		// so we need a threshold > 1 second to avoid flaky tests
		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
			event.WithStaleThreshold(2*time.Second),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish event to create initial activity
		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		// Wait a short time for processing
		time.Sleep(50 * time.Millisecond)

		// Verify event was processed
		stats := processor.Stats()
		require.Equal(t, int64(1), stats.EventsProcessed, "event should be processed")

		// Should be healthy right after processing
		err := processor.Healthcheck(ctx)
		require.NoError(t, err, "processor should be healthy after recent activity")

		// Now wait past the stale threshold (2 seconds)
		time.Sleep(3 * time.Second)

		// Should now be stale
		err = processor.Healthcheck(ctx)
		require.Error(t, err)
		assert.True(t, errors.Is(err, event.ErrHealthcheckFailed))
		assert.True(t, errors.Is(err, event.ErrProcessorStale))

		require.NoError(t, processor.Stop())
	})

	t.Run("stuck processor", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		block := make(chan struct{})

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			<-block
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
			event.WithStuckThreshold(2),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish multiple events that will block
		publisher := event.NewPublisher(bus)
		for i := 0; i < 3; i++ {
			require.NoError(t, publisher.Publish(ctx, UserCreated{
				UserID: fmt.Sprintf("user-%d", i),
				Email:  fmt.Sprintf("user%d@example.com", i),
			}))
		}

		// Wait for handlers to start
		time.Sleep(100 * time.Millisecond)

		// Should be stuck
		err := processor.Healthcheck(ctx)
		require.Error(t, err)
		assert.True(t, errors.Is(err, event.ErrHealthcheckFailed))
		assert.True(t, errors.Is(err, event.ErrProcessorStuck))

		// Unblock
		close(block)

		require.NoError(t, processor.Stop())
	})
}

func TestProcessor_MultipleHandlersPerEvent(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	var handler1Count atomic.Int32
	var handler2Count atomic.Int32
	var handler3Count atomic.Int32

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handler1Count.Add(1)
		return nil
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handler2Count.Add(1)
		return nil
	})

	handler3 := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handler3Count.Add(1)
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler1, handler2, handler3),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish single event
	publisher := event.NewPublisher(bus)
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}))

	time.Sleep(100 * time.Millisecond)

	// All handlers should have been called
	assert.Equal(t, int32(1), handler1Count.Load())
	assert.Equal(t, int32(1), handler2Count.Load())
	assert.Equal(t, int32(1), handler3Count.Load())

	// Stats should show 3 processed events (one per handler)
	stats := processor.Stats()
	assert.Equal(t, int64(3), stats.EventsProcessed)

	require.NoError(t, processor.Stop())
}

func TestProcessor_HandlerContext(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	var mu sync.Mutex
	var capturedEventID string
	var capturedEventName string
	var capturedEventTime time.Time
	var capturedStartTime time.Time

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		mu.Lock()
		defer mu.Unlock()
		capturedEventID = event.EventID(ctx)
		capturedEventName = event.EventName(ctx)
		capturedEventTime = event.EventTime(ctx)
		capturedStartTime = event.StartProcessingTime(ctx)
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	publisher := event.NewPublisher(bus)
	payload := UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}
	require.NoError(t, publisher.Publish(ctx, payload))

	// Wait for event to be processed
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed == 1
	}, time.Second, 10*time.Millisecond)

	// Verify context values
	mu.Lock()
	defer mu.Unlock()
	assert.NotEmpty(t, capturedEventID, "event ID should be captured")
	assert.Equal(t, "UserCreated", capturedEventName)
	assert.False(t, capturedEventTime.IsZero(), "event time should be captured")
	assert.False(t, capturedStartTime.IsZero(), "processing start time should be captured")

	require.NoError(t, processor.Stop())
}

func TestProcessor_UnmarshalErrors(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish malformed JSON directly
	err := bus.Publish(ctx, []byte("invalid json"))
	require.NoError(t, err)

	// Give processor time to attempt processing
	time.Sleep(100 * time.Millisecond)

	// Processor should continue running despite unmarshal error
	stats := processor.Stats()
	assert.True(t, stats.IsRunning)
	assert.Equal(t, int64(0), stats.EventsProcessed)

	require.NoError(t, processor.Stop())
}

func TestProcessor_EventBusClosed(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Close the bus
	require.NoError(t, bus.Close())

	// Processor should exit cleanly when event source closes
	err := <-errCh
	require.NoError(t, err)
}

func TestProcessor_DifferentEventTypes(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	var userCreatedCount atomic.Int32
	var orderPlacedCount atomic.Int32
	var paymentProcessedCount atomic.Int32

	userHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		userCreatedCount.Add(1)
		return nil
	})

	orderHandler := event.NewHandlerFunc(func(ctx context.Context, evt OrderPlaced) error {
		orderPlacedCount.Add(1)
		return nil
	})

	paymentHandler := event.NewHandlerFunc(func(ctx context.Context, evt PaymentProcessed) error {
		paymentProcessedCount.Add(1)
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(userHandler, orderHandler, paymentHandler),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish different event types
	publisher := event.NewPublisher(bus)
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}))
	require.NoError(t, publisher.Publish(ctx, OrderPlaced{
		OrderID: "order-1",
		Amount:  99.99,
	}))
	require.NoError(t, publisher.Publish(ctx, PaymentProcessed{
		PaymentID: "payment-1",
		Status:    "completed",
	}))

	// Wait for all events to be processed
	require.Eventually(t, func() bool {
		return userCreatedCount.Load() == 1 &&
			orderPlacedCount.Load() == 1 &&
			paymentProcessedCount.Load() == 1
	}, 2*time.Second, 50*time.Millisecond, "all events should be processed")

	stats := processor.Stats()
	assert.Equal(t, int64(3), stats.EventsProcessed)

	require.NoError(t, processor.Stop())
}

func TestProcessor_FallbackHandler(t *testing.T) {
	t.Parallel()

	t.Run("fallback called for unhandled events", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		var fallbackEvents []event.Event
		var mu sync.Mutex

		fallback := func(ctx context.Context, evt event.Event) error {
			mu.Lock()
			defer mu.Unlock()
			fallbackEvents = append(fallbackEvents, evt)
			return nil
		}

		// Only handle UserCreated
		userHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(userHandler),
			event.WithFallbackHandler(fallback),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		publisher := event.NewPublisher(bus)

		// Handled event
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		// Unhandled events
		require.NoError(t, publisher.Publish(ctx, OrderPlaced{
			OrderID: "order-1",
			Amount:  99.99,
		}))
		require.NoError(t, publisher.Publish(ctx, PaymentProcessed{
			PaymentID: "payment-1",
			Status:    "completed",
		}))

		time.Sleep(100 * time.Millisecond)

		// Verify fallback received unhandled events
		mu.Lock()
		assert.Equal(t, 2, len(fallbackEvents))
		// Events may arrive in any order due to concurrent processing
		eventNames := []string{fallbackEvents[0].Name, fallbackEvents[1].Name}
		assert.Contains(t, eventNames, "OrderPlaced")
		assert.Contains(t, eventNames, "PaymentProcessed")
		mu.Unlock()

		stats := processor.Stats()
		assert.Equal(t, int64(3), stats.EventsProcessed) // 1 handled + 2 fallback

		require.NoError(t, processor.Stop())
	})

	t.Run("fallback error handling", func(t *testing.T) {
		t.Parallel()

		bus := event.NewChannelBus()
		defer bus.Close()

		testErr := errors.New("fallback error")
		fallback := func(ctx context.Context, evt event.Event) error {
			return testErr
		}

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithFallbackHandler(fallback),
		)

		ctx := context.Background()
		go func() {
			_ = processor.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)

		publisher := event.NewPublisher(bus)
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "123",
			Email:  "test@example.com",
		}))

		time.Sleep(100 * time.Millisecond)

		stats := processor.Stats()
		assert.Equal(t, int64(0), stats.EventsProcessed)
		assert.Equal(t, int64(1), stats.EventsFailed)

		require.NoError(t, processor.Stop())
	})
}

func TestProcessor_HandlerPanic(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		panic("handler panic")
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	publisher := event.NewPublisher(bus)
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}))

	time.Sleep(100 * time.Millisecond)

	// Processor should continue running after panic
	stats := processor.Stats()
	assert.True(t, stats.IsRunning)
	assert.Equal(t, int64(0), stats.EventsProcessed)
	assert.Equal(t, int64(1), stats.EventsFailed)

	require.NoError(t, processor.Stop())
}

func TestProcessor_ContextCancellationDuringHandling(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	started := make(chan struct{})
	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_ = processor.Start(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	publisher := event.NewPublisher(bus)
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "123",
		Email:  "test@example.com",
	}))

	// Wait for handler to start
	<-started

	// Cancel context while handler is running
	cancel()

	// Wait for processor Start() to return
	select {
	case <-done:
		// Success - processor stopped
	case <-time.After(time.Second):
		t.Fatal("processor did not stop after context cancellation")
	}
}

func TestProcessor_ConcurrencyControl_Semaphore(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	maxConcurrent := 2
	block := make(chan struct{})
	started := make(chan struct{}, 10)

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		started <- struct{}{}
		<-block
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
		event.WithMaxConcurrentHandlers(maxConcurrent),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish more events than the concurrency limit
	publisher := event.NewPublisher(bus)
	for i := 0; i < 5; i++ {
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: fmt.Sprintf("user-%d", i),
			Email:  fmt.Sprintf("user%d@example.com", i),
		}))
	}

	// Wait for max concurrent to start processing (not just spawn goroutines)
	require.Eventually(t, func() bool {
		return len(started) == maxConcurrent
	}, time.Second, 10*time.Millisecond, "should start exactly maxConcurrent handlers")

	// Verify that no more than maxConcurrent have started processing
	// even though we published 5 events
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, maxConcurrent, len(started), "should have exactly maxConcurrent handlers processing")

	// ActiveEvents counts all spawned goroutines (including those waiting for semaphore)
	// so it should be 5 (all events create goroutines immediately)
	stats := processor.Stats()
	assert.Equal(t, int32(5), stats.ActiveEvents, "all 5 events should have spawned goroutines")

	// Unblock all handlers
	close(block)

	// Wait for all to complete
	time.Sleep(200 * time.Millisecond)

	// All should have been started eventually
	assert.Equal(t, 5, len(started))

	stats = processor.Stats()
	assert.Equal(t, int64(5), stats.EventsProcessed)
	assert.Equal(t, int32(0), stats.ActiveEvents)

	require.NoError(t, processor.Stop())
}

func TestProcessor_PayloadUnmarshalError(t *testing.T) {
	t.Parallel()

	bus := event.NewChannelBus()
	defer bus.Close()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	ctx := context.Background()
	go func() {
		_ = processor.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create event with payload that can't unmarshal to UserCreated
	// Use map[string]any with incompatible types
	evt := event.Event{
		ID:        "123",
		Name:      "UserCreated",
		Payload:   map[string]any{"user_id": 12345, "email": []string{"invalid"}}, // Wrong types
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(evt)
	require.NoError(t, err)

	err = bus.Publish(ctx, data)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Handler should have failed due to unmarshal error
	stats := processor.Stats()
	assert.Equal(t, int64(0), stats.EventsProcessed, "should not process events with unmarshal errors")
	assert.Equal(t, int64(1), stats.EventsFailed, "should track failed events")

	require.NoError(t, processor.Stop())
}
