package event_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All event types are defined in test_types.go for consistency across test files

// TestUserRegistrationFlow tests the complete flow of user registration with multiple handlers
func TestUserRegistrationFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Track which handlers executed
	var (
		emailSent       atomic.Bool
		profileCreated  atomic.Bool
		analyticsLogged atomic.Bool
		mu              sync.Mutex
		executionOrder  []string
	)

	// Welcome email handler
	welcomeEmailHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		emailSent.Store(true)
		mu.Lock()
		executionOrder = append(executionOrder, "email")
		mu.Unlock()
		return nil
	})

	// Profile creation handler
	profileHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		profileCreated.Store(true)
		mu.Lock()
		executionOrder = append(executionOrder, "profile")
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return nil
	})

	// Analytics handler
	analyticsHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		analyticsLogged.Store(true)
		mu.Lock()
		executionOrder = append(executionOrder, "analytics")
		mu.Unlock()
		return nil
	})

	// Set up bus, publisher, and processor
	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(welcomeEmailHandler, profileHandler, analyticsHandler),
	)

	// Start processor in background
	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	// Give processor time to start
	time.Sleep(50 * time.Millisecond)

	// Publish user created event
	err := publisher.Publish(ctx, UserCreated{
		UserID: "user-123",
		Email:  "user@example.com",
	})
	require.NoError(t, err)

	// Wait for all handlers to complete
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		t.Logf("Stats: Processed=%d, Failed=%d, Active=%d", stats.EventsProcessed, stats.EventsFailed, stats.ActiveEvents)
		return stats.EventsProcessed == 3
	}, 2*time.Second, 50*time.Millisecond, "expected 3 events processed")

	// Verify all handlers executed
	assert.True(t, emailSent.Load(), "welcome email should be sent")
	assert.True(t, profileCreated.Load(), "profile should be created")
	assert.True(t, analyticsLogged.Load(), "analytics should be logged")

	// Verify all three handlers executed (order doesn't matter due to concurrency)
	mu.Lock()
	assert.Len(t, executionOrder, 3, "all handlers should execute")
	mu.Unlock()

	// Verify stats
	stats := processor.Stats()
	assert.Equal(t, int64(3), stats.EventsProcessed)
	assert.Equal(t, int64(0), stats.EventsFailed)
	assert.Equal(t, int32(0), stats.ActiveEvents)
	assert.True(t, stats.IsRunning)
	assert.False(t, stats.LastActivityAt.IsZero())

	// Stop processor
	require.NoError(t, processor.Stop())
}

// TestOrderProcessingFlow tests order processing with multiple handlers
func TestOrderProcessingFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var (
		inventoryValidated atomic.Bool
		paymentCharged     atomic.Bool
		confirmationSent   atomic.Bool
	)

	// Inventory validation handler
	inventoryHandler := event.NewHandlerFunc(func(ctx context.Context, evt OrderPlaced) error {
		if evt.Amount <= 0 {
			return errors.New("invalid amount")
		}
		inventoryValidated.Store(true)
		return nil
	})

	// Payment handler
	paymentHandler := event.NewHandlerFunc(func(ctx context.Context, evt OrderPlaced) error {
		time.Sleep(20 * time.Millisecond) // Simulate payment processing
		paymentCharged.Store(true)
		return nil
	})

	// Confirmation handler
	confirmationHandler := event.NewHandlerFunc(func(ctx context.Context, evt OrderPlaced) error {
		confirmationSent.Store(true)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(inventoryHandler, paymentHandler, confirmationHandler),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish order
	err := publisher.Publish(ctx, OrderPlaced{
		OrderID: "order-123",
		UserID:  "user-123",
		Total:   99.99,
		Amount:  99.99,
	})
	require.NoError(t, err)

	// Wait for processing
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed == 3
	}, 2*time.Second, 10*time.Millisecond)

	assert.True(t, inventoryValidated.Load())
	assert.True(t, paymentCharged.Load())
	assert.True(t, confirmationSent.Load())

	require.NoError(t, processor.Stop())
}

// TestNotificationSystem tests multiple event types with routing handlers
func TestNotificationSystem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var (
		emailCount atomic.Int32
		smsCount   atomic.Int32
	)

	// Email notification handler
	emailHandler := event.NewHandlerFunc(func(ctx context.Context, evt EmailSent) error {
		emailCount.Add(1)
		return nil
	})

	// SMS notification handler
	smsHandler := event.NewHandlerFunc(func(ctx context.Context, evt NotificationSent) error {
		smsCount.Add(1)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(emailHandler, smsHandler),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish different event types
	require.NoError(t, publisher.Publish(ctx, EmailSent{To: "user1@example.com", Subject: "Welcome"}))
	require.NoError(t, publisher.Publish(ctx, EmailSent{To: "user2@example.com", Subject: "Newsletter"}))
	require.NoError(t, publisher.Publish(ctx, NotificationSent{UserID: "user-1", Message: "Code: 1234"}))
	require.NoError(t, publisher.Publish(ctx, NotificationSent{UserID: "user-2", Message: "Alert"}))

	// Wait for all events
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed == 4
	}, 1*time.Second, 10*time.Millisecond)

	assert.Equal(t, int32(2), emailCount.Load())
	assert.Equal(t, int32(2), smsCount.Load())

	require.NoError(t, processor.Stop())
}

// TestConcurrentPublishing tests multiple goroutines publishing events simultaneously
func TestConcurrentPublishing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var processedCount atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		processedCount.Add(1)
		time.Sleep(5 * time.Millisecond) // Simulate work
		return nil
	})

	bus := event.NewChannelBus(event.WithBufferSize(200))
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
		event.WithMaxConcurrentHandlers(10), // Limit concurrency
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish from multiple goroutines
	const numGoroutines = 10
	const eventsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				err := publisher.Publish(ctx, UserCreated{
					UserID: "user-123",
					Email:  "user@example.com",
				})
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all events to be processed
	require.Eventually(t, func() bool {
		return processedCount.Load() == numGoroutines*eventsPerGoroutine
	}, 5*time.Second, 50*time.Millisecond)

	// Wait for all handler goroutines to complete
	time.Sleep(100 * time.Millisecond)

	stats := processor.Stats()
	assert.Equal(t, int64(numGoroutines*eventsPerGoroutine), stats.EventsProcessed, "all events should be processed successfully")
	assert.Equal(t, int64(0), stats.EventsFailed, "no events should fail")
	assert.Equal(t, int32(0), stats.ActiveEvents, "no events should be active after completion")

	require.NoError(t, processor.Stop())
}

// TestGracefulShutdown tests that events in flight during Stop() complete or timeout properly
func TestGracefulShutdown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var (
		handlerStarted  atomic.Int32
		handlerFinished atomic.Int32
	)

	// Handler that takes some time
	slowHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handlerStarted.Add(1)
		time.Sleep(200 * time.Millisecond)
		handlerFinished.Add(1)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(slowHandler),
		event.WithShutdownTimeout(500*time.Millisecond), // Enough time to complete
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish 3 events
	for i := 0; i < 3; i++ {
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "user-123",
			Email:  "user@example.com",
		}))
	}

	// Wait for handlers to start
	time.Sleep(100 * time.Millisecond)

	// Stop processor - should wait for handlers to complete
	err := processor.Stop()
	require.NoError(t, err)

	// All handlers should have finished
	assert.Equal(t, handlerStarted.Load(), handlerFinished.Load())
	assert.Equal(t, int32(3), handlerFinished.Load())
}

// TestGracefulShutdownTimeout tests shutdown timeout when handlers take too long
func TestGracefulShutdownTimeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var handlerStarted atomic.Int32

	// Handler that takes longer than shutdown timeout
	verySlowHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handlerStarted.Add(1)
		time.Sleep(2 * time.Second)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(verySlowHandler),
		event.WithShutdownTimeout(100*time.Millisecond), // Short timeout
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "user-123",
		Email:  "user@example.com",
	}))

	// Wait for handler to start
	time.Sleep(100 * time.Millisecond)

	// Stop should timeout
	err := processor.Stop()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown timeout exceeded")
}

// TestFallbackHandler tests that events with no registered handlers go to fallback
func TestFallbackHandler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	type UnknownEvent struct {
		Data string
	}

	var (
		fallbackCalled atomic.Bool
		receivedEvent  event.Event
		mu             sync.Mutex
	)

	fallbackHandler := func(ctx context.Context, evt event.Event) error {
		fallbackCalled.Store(true)
		mu.Lock()
		receivedEvent = evt
		mu.Unlock()
		return nil
	}

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithFallbackHandler(fallbackHandler),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish unknown event
	err := publisher.Publish(ctx, UnknownEvent{Data: "test-data"})
	require.NoError(t, err)

	// Wait for fallback to be called
	require.Eventually(t, func() bool {
		return fallbackCalled.Load()
	}, 1*time.Second, 10*time.Millisecond)

	// Verify event details
	mu.Lock()
	assert.Equal(t, "UnknownEvent", receivedEvent.Name)
	assert.NotEmpty(t, receivedEvent.ID)
	mu.Unlock()

	stats := processor.Stats()
	assert.Equal(t, int64(1), stats.EventsProcessed)

	require.NoError(t, processor.Stop())
}

// TestHandlerPanicRecovery tests that handler panics are recovered and tracked
func TestHandlerPanicRecovery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var (
		panicHandlerCalled atomic.Bool
		goodHandlerCalled  atomic.Bool
	)

	// Handler that panics
	panicHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		panicHandlerCalled.Store(true)
		panic("something went wrong")
	})

	// Handler that works fine
	goodHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		goodHandlerCalled.Store(true)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(panicHandler, goodHandler),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	err := publisher.Publish(ctx, UserCreated{
		UserID: "user-123",
		Email:  "user@example.com",
	})
	require.NoError(t, err)

	// Wait for handlers to complete
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed+stats.EventsFailed == 2
	}, 1*time.Second, 10*time.Millisecond)

	// Both handlers should have been called
	assert.True(t, panicHandlerCalled.Load())
	assert.True(t, goodHandlerCalled.Load())

	// Check stats - panic should be tracked as failure
	stats := processor.Stats()
	assert.Equal(t, int64(1), stats.EventsProcessed) // Good handler
	assert.Equal(t, int64(1), stats.EventsFailed)    // Panic handler
	assert.Equal(t, int32(0), stats.ActiveEvents)

	require.NoError(t, processor.Stop())
}

// TestContextCancellation tests cleanup when context is cancelled during processing
func TestContextCancellation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var handlerCalled atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		handlerCalled.Add(1)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	procCtx, procCancel := context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		_ = processor.Start(procCtx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "user-123",
		Email:  "user@example.com",
	}))

	// Wait for handler to start
	require.Eventually(t, func() bool {
		return handlerCalled.Load() > 0
	}, time.Second, 10*time.Millisecond, "handler should be called")

	// Cancel context while handler is running
	procCancel()

	// Wait for processor Start() to return
	select {
	case <-done:
		// Success - processor stopped
	case <-time.After(2 * time.Second):
		t.Fatal("processor did not stop after context cancellation")
	}
}

// TestProcessorOptions tests various processor configuration options
func TestProcessorOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var processed atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		processed.Add(1)
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	bus := event.NewChannelBus(event.WithBufferSize(50))
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
		event.WithMaxConcurrentHandlers(2), // Only 2 concurrent handlers
		event.WithShutdownTimeout(5*time.Second),
		event.WithStaleThreshold(10*time.Second),
		event.WithStuckThreshold(100),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish multiple events
	for i := 0; i < 10; i++ {
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "user-123",
			Email:  "user@example.com",
		}))
	}

	// Due to concurrency limit of 2, not all events should be processed immediately
	// Check that we have some active events at any point
	time.Sleep(100 * time.Millisecond)
	stats := processor.Stats()

	// At least some events should be queued/processing
	assert.Greater(t, stats.ActiveEvents, int32(0), "should have active events due to concurrency limit")

	// Eventually all should be processed
	require.Eventually(t, func() bool {
		return processed.Load() == 10
	}, 5*time.Second, 50*time.Millisecond)

	require.NoError(t, processor.Stop())
}

// TestHealthcheck tests processor health check functionality
func TestHealthcheck(t *testing.T) {
	t.Parallel()

	t.Run("healthy processor", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		bus := event.NewChannelBus()
		defer bus.Close()

		publisher := event.NewPublisher(bus)

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		procCtx, procCancel := context.WithCancel(ctx)
		defer procCancel()

		go func() {
			_ = processor.Start(procCtx)
		}()

		time.Sleep(50 * time.Millisecond)

		// Publish event to update activity
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "user-123",
			Email:  "user@example.com",
		}))

		time.Sleep(100 * time.Millisecond)

		// Health check should pass
		err := processor.Healthcheck(ctx)
		assert.NoError(t, err)

		require.NoError(t, processor.Stop())
	})

	t.Run("not running processor", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			return nil
		})

		bus := event.NewChannelBus()
		defer bus.Close()

		processor := event.NewProcessor(
			event.WithEventSource(bus),
			event.WithHandler(handler),
		)

		// Health check should fail - processor not started
		err := processor.Healthcheck(ctx)
		require.Error(t, err)
		assert.ErrorIs(t, err, event.ErrHealthcheckFailed)
		assert.ErrorIs(t, err, event.ErrProcessorNotRunning)
	})
}

// TestMultipleEventTypes tests handling multiple different event types
func TestMultipleEventTypes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var (
		userEvents  atomic.Int32
		orderEvents atomic.Int32
		emailEvents atomic.Int32
	)

	userHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		userEvents.Add(1)
		return nil
	})

	orderHandler := event.NewHandlerFunc(func(ctx context.Context, evt OrderPlaced) error {
		orderEvents.Add(1)
		return nil
	})

	emailHandler := event.NewHandlerFunc(func(ctx context.Context, evt EmailSent) error {
		emailEvents.Add(1)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(userHandler, orderHandler, emailHandler),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish mixed events
	require.NoError(t, publisher.Publish(ctx, UserCreated{UserID: "1", Email: "user1@example.com"}))
	require.NoError(t, publisher.Publish(ctx, OrderPlaced{OrderID: "1", UserID: "1", Amount: 100}))
	require.NoError(t, publisher.Publish(ctx, EmailSent{To: "user@example.com", Subject: "Test"}))
	require.NoError(t, publisher.Publish(ctx, UserCreated{UserID: "2", Email: "user2@example.com"}))
	require.NoError(t, publisher.Publish(ctx, OrderPlaced{OrderID: "2", UserID: "2", Amount: 200}))

	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed == 5
	}, 1*time.Second, 10*time.Millisecond)

	assert.Equal(t, int32(2), userEvents.Load())
	assert.Equal(t, int32(2), orderEvents.Load())
	assert.Equal(t, int32(1), emailEvents.Load())

	require.NoError(t, processor.Stop())
}

// TestHandlerErrors tests that handler errors are tracked properly
func TestHandlerErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	errorHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return errors.New("handler error")
	})

	successHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(errorHandler, successHandler),
	)

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish event - should trigger both handlers
	require.NoError(t, publisher.Publish(ctx, UserCreated{
		UserID: "user-123",
		Email:  "user@example.com",
	}))

	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed+stats.EventsFailed == 2
	}, 1*time.Second, 10*time.Millisecond)

	stats := processor.Stats()
	assert.Equal(t, int64(1), stats.EventsProcessed)
	assert.Equal(t, int64(1), stats.EventsFailed)

	require.NoError(t, processor.Stop())
}

// TestStatsTracking tests comprehensive stats tracking
func TestStatsTracking(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	})

	bus := event.NewChannelBus()
	defer bus.Close()

	publisher := event.NewPublisher(bus)

	processor := event.NewProcessor(
		event.WithEventSource(bus),
		event.WithHandler(handler),
	)

	// Stats before start
	stats := processor.Stats()
	assert.False(t, stats.IsRunning)
	assert.Equal(t, int64(0), stats.EventsProcessed)
	assert.Equal(t, int64(0), stats.EventsFailed)
	assert.Equal(t, int32(0), stats.ActiveEvents)
	assert.True(t, stats.LastActivityAt.IsZero())

	procCtx, procCancel := context.WithCancel(ctx)
	defer procCancel()

	go func() {
		_ = processor.Start(procCtx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Stats after start
	stats = processor.Stats()
	assert.True(t, stats.IsRunning)

	// Publish events
	for i := 0; i < 5; i++ {
		require.NoError(t, publisher.Publish(ctx, UserCreated{
			UserID: "user-123",
			Email:  "user@example.com",
		}))
	}

	// Check active events while processing
	time.Sleep(10 * time.Millisecond)
	stats = processor.Stats()
	assert.Greater(t, stats.ActiveEvents, int32(0), "should have active events during processing")

	// Wait for completion
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.EventsProcessed == 5 && stats.ActiveEvents == 0
	}, 2*time.Second, 10*time.Millisecond)

	stats = processor.Stats()
	assert.Equal(t, int64(5), stats.EventsProcessed)
	assert.Equal(t, int64(0), stats.EventsFailed)
	assert.Equal(t, int32(0), stats.ActiveEvents)
	assert.False(t, stats.LastActivityAt.IsZero())

	require.NoError(t, processor.Stop())

	// Stats after stop
	stats = processor.Stats()
	assert.False(t, stats.IsRunning)
}
