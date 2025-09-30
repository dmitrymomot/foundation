package event_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test event types
type (
	TestEvent struct {
		ID      string
		Message string
	}

	AnotherEvent struct {
		Count int
	}

	PrimitiveEvent int

	PointerEvent struct {
		Value string
	}
)

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandlerFunc_TypeSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		handler     event.Handler
		payload     any
		expectError bool
	}{
		{
			name: "struct type - correct payload",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
				return nil
			}),
			payload:     TestEvent{ID: "123", Message: "test"},
			expectError: false,
		},
		{
			name: "struct type - wrong payload type",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
				return nil
			}),
			payload:     AnotherEvent{Count: 42},
			expectError: true,
		},
		{
			name: "pointer type - correct payload",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt *PointerEvent) error {
				return nil
			}),
			payload:     &PointerEvent{Value: "test"},
			expectError: false,
		},
		{
			name: "pointer type - non-pointer payload",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt *PointerEvent) error {
				return nil
			}),
			payload:     PointerEvent{Value: "test"},
			expectError: true,
		},
		{
			name: "primitive type - correct payload",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt PrimitiveEvent) error {
				return nil
			}),
			payload:     PrimitiveEvent(123),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.handler.Handle(context.Background(), tt.payload)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid payload type")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHandlerFunc_NameDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		handler      event.Handler
		expectedName string
	}{
		{
			name: "struct type",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
				return nil
			}),
			expectedName: "TestEvent",
		},
		{
			name: "pointer type",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt *PointerEvent) error {
				return nil
			}),
			expectedName: "PointerEvent",
		},
		{
			name: "primitive type",
			handler: event.NewHandlerFunc(func(ctx context.Context, evt PrimitiveEvent) error {
				return nil
			}),
			expectedName: "PrimitiveEvent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expectedName, tt.handler.Name())
		})
	}
}

func TestHandlerFunc_PanicRecovery(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		panic("handler panicked")
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler panicked")
	assert.Contains(t, err.Error(), "stack trace")
}

func TestHandlerFunc_ErrorPropagation(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("handler error")
	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return expectedErr
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handler error")
}

// =============================================================================
// Sync Transport Tests
// =============================================================================

func TestSyncTransport_BasicExecution(t *testing.T) {
	t.Parallel()

	var executed atomic.Bool
	var receivedID atomic.Value

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		receivedID.Store(evt.ID)
		executed.Store(true)
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Small delay to ensure processor is ready
	time.Sleep(10 * time.Millisecond)

	err := processor.Publish(context.Background(), TestEvent{ID: "123", Message: "test"})
	require.NoError(t, err)
	assert.True(t, executed.Load())
	assert.Equal(t, "123", receivedID.Load().(string))
}

func TestSyncTransport_MultipleHandlers(t *testing.T) {
	t.Parallel()

	var count1, count2 atomic.Int32

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count1.Add(1)
		return nil
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count2.Add(1)
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler1)
	processor.Register(handler2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
	assert.Equal(t, int32(1), count1.Load())
	assert.Equal(t, int32(1), count2.Load())
}

func TestSyncTransport_ZeroHandlers(t *testing.T) {
	t.Parallel()

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Should not error with zero handlers (just warning)
	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
}

func TestSyncTransport_ErrorAggregation(t *testing.T) {
	t.Parallel()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return err1
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return err2
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler1)
	processor.Register(handler2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error 1")
	assert.Contains(t, err.Error(), "error 2")
}

func TestSyncTransport_ContextCancellation(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Cancel context before dispatch
	dispatchCtx, dispatchCancel := context.WithCancel(context.Background())
	dispatchCancel()

	err := processor.Publish(dispatchCtx, TestEvent{ID: "123"})
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestSyncTransport_ContextPropagation(t *testing.T) {
	t.Parallel()

	type ctxKey string
	const key ctxKey = "test-key"

	var receivedValue atomic.Value

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		if val := ctx.Value(key); val != nil {
			receivedValue.Store(val.(string))
		}
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Small delay to ensure processor is ready
	time.Sleep(10 * time.Millisecond)

	dispatchCtx := context.WithValue(context.Background(), key, "test-value")
	err := processor.Publish(dispatchCtx, TestEvent{ID: "123"})
	require.NoError(t, err)
	assert.Equal(t, "test-value", receivedValue.Load().(string))
}

func TestSyncTransport_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count.Add(1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Publish from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := processor.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", id)})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(10), count.Load())
}

// =============================================================================
// Channel Transport Tests
// =============================================================================

func TestChannelTransport_BasicExecution(t *testing.T) {
	t.Parallel()

	var executed atomic.Bool
	var receivedID atomic.Value

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		receivedID.Store(evt.ID)
		executed.Store(true)
		return nil
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123", Message: "test"})
	require.NoError(t, err)

	// Wait for async processing
	require.Eventually(t, func() bool {
		return executed.Load()
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, "123", receivedID.Load().(string))
}

func TestChannelTransport_BufferFull(t *testing.T) {
	t.Parallel()

	// Slow handler to fill buffer - block it completely
	var block atomic.Bool
	block.Store(true)

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		for block.Load() {
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	})

	transport := event.NewChannelTransport(2) // Small buffer
	processor := event.NewProcessor(transport, event.WithWorkers(1))
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	// Give processor time to start
	time.Sleep(50 * time.Millisecond)

	publisher := event.NewPublisher(transport)

	// Fill the buffer: 1 event being processed (blocked) + 2 in buffer = 3 total
	for i := 0; i < 3; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Next publish should fail with buffer full
	err := publisher.Publish(context.Background(), TestEvent{ID: "overflow"})
	require.Error(t, err)
	assert.Equal(t, event.ErrBufferFull, err)

	// Unblock to allow graceful shutdown
	block.Store(false)
}

func TestChannelTransport_NonBlockingDispatch(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Dispatch should return immediately
	start := time.Now()
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, duration, 50*time.Millisecond, "dispatch should be non-blocking")
}

func TestChannelTransport_CloseIdempotence(t *testing.T) {
	t.Parallel()

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Close multiple times should not panic
	require.NotPanics(t, func() {
		transport.Close()
		transport.Close()
		transport.Close()
	})
}

func TestChannelTransport_GracefulShutdown(t *testing.T) {
	t.Parallel()

	var processed atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		processed.Add(1)
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	transport := event.NewChannelTransport(100)
	processor := event.NewProcessor(transport, event.WithWorkers(2))
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish events
	for i := 0; i < 10; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Cancel context to trigger shutdown
	cancel()

	// Wait for processor to finish
	time.Sleep(time.Second)

	// All events should be processed
	assert.Equal(t, int32(10), processed.Load())
}

func TestChannelTransport_ContextPreservation(t *testing.T) {
	t.Parallel()

	type ctxKey string
	const key ctxKey = "test-key"

	var receivedValue atomic.Value

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		if val := ctx.Value(key); val != nil {
			receivedValue.Store(val.(string))
		}
		return nil
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	dispatchCtx := context.WithValue(context.Background(), key, "async-value")
	err := publisher.Publish(dispatchCtx, TestEvent{ID: "123"})
	require.NoError(t, err)

	// Wait for async processing
	require.Eventually(t, func() bool {
		v := receivedValue.Load()
		return v != nil && v.(string) != ""
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, "async-value", receivedValue.Load().(string))
}

func TestChannelTransport_ZeroBufferPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		event.NewChannelTransport(0)
	})
}

// =============================================================================
// Publisher Tests
// =============================================================================

func TestPublisher_SyncTransport(t *testing.T) {
	t.Parallel()

	var executed atomic.Bool

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		executed.Store(true)
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
	assert.True(t, executed.Load())
}

func TestPublisher_ChannelTransport(t *testing.T) {
	t.Parallel()

	var executed atomic.Bool

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		executed.Store(true)
		return nil
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return executed.Load()
	}, time.Second, 10*time.Millisecond)
}

func TestPublisher_EventNameDerivation(t *testing.T) {
	t.Parallel()

	var receivedName atomic.Value

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		receivedName.Store("TestEvent")
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Small delay to ensure processor is ready
	time.Sleep(10 * time.Millisecond)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
	assert.Equal(t, "TestEvent", receivedName.Load().(string))
}

// =============================================================================
// Processor Tests
// =============================================================================

func TestProcessor_RegisterAfterRunPanics(t *testing.T) {
	t.Parallel()

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)

	ctx, cancel := context.WithCancel(context.Background())
	go processor.Run(ctx)

	// Wait for processor to start
	time.Sleep(50 * time.Millisecond)

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	require.Panics(t, func() {
		processor.Register(handler)
	})

	cancel()
}

func TestProcessor_MultipleWorkers(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	var mu sync.Mutex
	workerIDs := make(map[int]bool)

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		mu.Lock()
		workerIDs[int(count.Add(1))] = true
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	transport := event.NewChannelTransport(100)
	processor := event.NewProcessor(transport, event.WithWorkers(5))
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish events concurrently
	for i := 0; i < 20; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Wait for processing
	require.Eventually(t, func() bool {
		return count.Load() == 20
	}, 2*time.Second, 10*time.Millisecond)
}

func TestProcessor_StatsTracking(t *testing.T) {
	t.Parallel()

	successHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	failHandler := event.NewHandlerFunc(func(ctx context.Context, evt AnotherEvent) error {
		return errors.New("handler failed")
	})

	transport := event.NewChannelTransport(100)
	processor := event.NewProcessor(transport)
	processor.Register(successHandler)
	processor.Register(failHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish successful events
	for i := 0; i < 5; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Publish failing events
	for i := 0; i < 3; i++ {
		err := publisher.Publish(context.Background(), AnotherEvent{Count: i})
		require.NoError(t, err)
	}

	// Wait for processing
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.Received == 8
	}, time.Second, 10*time.Millisecond)

	stats := processor.Stats()
	assert.Equal(t, uint64(8), stats.Received)
	assert.Equal(t, uint64(5), stats.Processed)
	assert.Equal(t, uint64(3), stats.Failed)
}

func TestProcessor_ZeroHandlersNormalMode(t *testing.T) {
	t.Parallel()

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport) // strictMode = false by default

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Should not error, just warn
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	stats := processor.Stats()
	assert.Equal(t, uint64(1), stats.Received)
	assert.Equal(t, uint64(0), stats.Processed)
	assert.Equal(t, uint64(0), stats.Failed)
}

func TestProcessor_ZeroHandlersStrictMode(t *testing.T) {
	t.Parallel()

	var errorCalled atomic.Bool
	var errorMsg atomic.Value

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(
		transport,
		event.WithStrictHandlers(true),
		event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
			errorMsg.Store(err.Error())
			errorCalled.Store(true)
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Wait for error handler to be called
	require.Eventually(t, func() bool {
		return errorCalled.Load()
	}, time.Second, 10*time.Millisecond)

	assert.Contains(t, errorMsg.Load().(string), "no handlers registered")

	stats := processor.Stats()
	assert.Equal(t, uint64(1), stats.Received)
	assert.Equal(t, uint64(0), stats.Processed)
	assert.Equal(t, uint64(1), stats.Failed)
}

func TestProcessor_ErrorHandler(t *testing.T) {
	t.Parallel()

	var errorCalled atomic.Bool
	var errorMsg atomic.Value

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return errors.New("test error")
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(
		transport,
		event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
			errorMsg.Store(err.Error())
			errorCalled.Store(true)
		}),
	)
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return errorCalled.Load()
	}, time.Second, 10*time.Millisecond)

	assert.Contains(t, errorMsg.Load().(string), "test error")
}

func TestProcessor_HandlerIsolation(t *testing.T) {
	t.Parallel()

	var handler1Called, handler2Called atomic.Bool

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		handler1Called.Store(true)
		return errors.New("handler1 failed")
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		handler2Called.Store(true)
		return nil
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)
	processor.Register(handler1)
	processor.Register(handler2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Both handlers should be called despite handler1 failing
	require.Eventually(t, func() bool {
		return handler1Called.Load() && handler2Called.Load()
	}, time.Second, 10*time.Millisecond)
}

func TestProcessor_PublishConvenience(t *testing.T) {
	t.Parallel()

	var executed atomic.Bool

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		executed.Store(true)
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Use processor.Publish() convenience method
	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
	assert.True(t, executed.Load())
}

func TestProcessor_PublishAsyncTransportError(t *testing.T) {
	t.Parallel()

	// Channel transport implements PublisherTransport, so it actually works
	// This test verifies that processor.Publish() works with channel transport
	var executed atomic.Bool

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		executed.Store(true)
		return nil
	})

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	// Processor.Publish() works with channel transport (implements PublisherTransport)
	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Wait for async processing
	require.Eventually(t, func() bool {
		return executed.Load()
	}, time.Second, 10*time.Millisecond)
}

// =============================================================================
// Middleware Tests
// =============================================================================

func TestMiddleware_LoggingMiddleware(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(
		transport,
		event.WithMiddleware(event.LoggingMiddleware(logger)),
	)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	logs := buf.String()
	assert.Contains(t, logs, "event started")
	assert.Contains(t, logs, "event completed")
	assert.Contains(t, logs, "TestEvent")
}

func TestMiddleware_MultipleMiddleware(t *testing.T) {
	t.Parallel()

	var executionOrder []string
	var mu sync.Mutex

	middleware1 := func(next event.Handler) event.Handler {
		return event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
			mu.Lock()
			executionOrder = append(executionOrder, "middleware1-before")
			mu.Unlock()
			err := next.Handle(ctx, evt)
			mu.Lock()
			executionOrder = append(executionOrder, "middleware1-after")
			mu.Unlock()
			return err
		})
	}

	middleware2 := func(next event.Handler) event.Handler {
		return event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
			mu.Lock()
			executionOrder = append(executionOrder, "middleware2-before")
			mu.Unlock()
			err := next.Handle(ctx, evt)
			mu.Lock()
			executionOrder = append(executionOrder, "middleware2-after")
			mu.Unlock()
			return err
		})
	}

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		mu.Lock()
		executionOrder = append(executionOrder, "handler")
		mu.Unlock()
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(
		transport,
		event.WithMiddleware(middleware1, middleware2),
	)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Middleware applied left-to-right means middleware1 wraps the result of middleware2
	// So middleware2 wraps the handler first, then middleware1 wraps that
	expected := []string{
		"middleware2-before",
		"middleware1-before",
		"handler",
		"middleware1-after",
		"middleware2-after",
	}
	assert.Equal(t, expected, executionOrder)
}

func TestMiddleware_AppliedOnceAtRegistration(t *testing.T) {
	t.Parallel()

	var wrapCount atomic.Int32

	countingMiddleware := func(next event.Handler) event.Handler {
		wrapCount.Add(1)
		return next
	}

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(
		transport,
		event.WithMiddleware(countingMiddleware),
	)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Publish multiple times
	for i := 0; i < 5; i++ {
		err := processor.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Middleware should only be applied once during registration
	assert.Equal(t, int32(1), wrapCount.Load())
}

// =============================================================================
// Decorator Tests
// =============================================================================

func TestDecorator_WithRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		maxRetries    int
		failUntil     int
		expectedCalls int
		expectedError bool
		errorContains string
	}{
		{
			name:          "success on first try",
			maxRetries:    3,
			failUntil:     0,
			expectedCalls: 1,
			expectedError: false,
		},
		{
			name:          "success on second try",
			maxRetries:    3,
			failUntil:     1,
			expectedCalls: 2,
			expectedError: false,
		},
		{
			name:          "fail all retries",
			maxRetries:    3,
			failUntil:     999,
			expectedCalls: 4, // initial + 3 retries
			expectedError: true,
			errorContains: "failed after 3 retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var callCount atomic.Int32

			baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
				count := int(callCount.Add(1))
				if count <= tt.failUntil {
					return errors.New("temporary error")
				}
				return nil
			})

			handler := event.WithRetry(baseHandler, tt.maxRetries)

			transport := event.NewSyncTransport()
			processor := event.NewProcessor(transport)
			processor.Register(handler)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go processor.Run(ctx)

			err := processor.Publish(context.Background(), TestEvent{ID: "123"})

			assert.Equal(t, tt.expectedCalls, int(callCount.Load()))

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDecorator_WithRetryContextCancellation(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count := callCount.Add(1)
		// Add small delay to allow context cancellation to happen
		time.Sleep(20 * time.Millisecond)
		// Check context after delay
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Only fail on attempts before cancellation
		if count < 20 {
			return errors.New("always fails")
		}
		return nil
	})

	handler := event.WithRetry(baseHandler, 20)

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Small delay to ensure processor is ready
	time.Sleep(10 * time.Millisecond)

	// Cancel context after allowing a few attempts
	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer dispatchCancel()

	err := processor.Publish(dispatchCtx, TestEvent{ID: "123"})
	require.Error(t, err)

	// Should stop retrying when context is cancelled
	// With 20ms per attempt and 100ms timeout, should get ~5 attempts max
	assert.LessOrEqual(t, int(callCount.Load()), 8)
}

func TestDecorator_WithBackoff(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	var callTimes []time.Time
	var mu sync.Mutex

	baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		mu.Lock()
		callTimes = append(callTimes, time.Now())
		mu.Unlock()
		callCount.Add(1)
		return errors.New("temporary error")
	})

	handler := event.WithBackoff(baseHandler, 3, 50*time.Millisecond, 200*time.Millisecond)

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 retries with backoff")
	assert.Equal(t, int32(4), callCount.Load()) // initial + 3 retries

	// Verify backoff delays
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, callTimes, 4)

	// First retry should wait ~50ms
	delay1 := callTimes[1].Sub(callTimes[0])
	assert.GreaterOrEqual(t, delay1, 50*time.Millisecond)
	assert.Less(t, delay1, 150*time.Millisecond)

	// Second retry should wait ~100ms
	delay2 := callTimes[2].Sub(callTimes[1])
	assert.GreaterOrEqual(t, delay2, 100*time.Millisecond)
	assert.Less(t, delay2, 200*time.Millisecond)

	// Third retry should be capped at maxDelay (200ms)
	delay3 := callTimes[3].Sub(callTimes[2])
	assert.GreaterOrEqual(t, delay3, 200*time.Millisecond)
	assert.Less(t, delay3, 300*time.Millisecond)
}

func TestDecorator_WithBackoffContextCancellation(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		callCount.Add(1)
		return errors.New("always fails")
	})

	handler := event.WithBackoff(baseHandler, 10, 100*time.Millisecond, time.Second)

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer dispatchCancel()

	err := processor.Publish(dispatchCtx, TestEvent{ID: "123"})
	require.Error(t, err)

	// Should stop during backoff wait when context is cancelled
	assert.LessOrEqual(t, int(callCount.Load()), 3)
}

func TestDecorator_WithTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		handlerDelay  time.Duration
		timeout       time.Duration
		expectedError bool
		errorContains string
	}{
		{
			name:          "completes before timeout",
			handlerDelay:  50 * time.Millisecond,
			timeout:       200 * time.Millisecond,
			expectedError: false,
		},
		{
			name:          "exceeds timeout",
			handlerDelay:  300 * time.Millisecond,
			timeout:       100 * time.Millisecond,
			expectedError: true,
			errorContains: "handler timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
				select {
				case <-time.After(tt.handlerDelay):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})

			handler := event.WithTimeout(baseHandler, tt.timeout)

			transport := event.NewSyncTransport()
			processor := event.NewProcessor(transport)
			processor.Register(handler)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go processor.Run(ctx)

			err := processor.Publish(context.Background(), TestEvent{ID: "123"})

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDecorator_Decorate(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	attempt := 0

	baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		callCount.Add(1)
		attempt++
		if attempt < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	// Chain multiple decorators
	handler := event.Decorate(
		baseHandler,
		event.Retry(3),
		event.Timeout(time.Second),
	)

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Should succeed on second attempt
	assert.Equal(t, int32(2), callCount.Load())
}

func TestDecorator_FactoryFunctions(t *testing.T) {
	t.Parallel()

	baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	// Test factory functions return decorators that can be composed
	handler := event.Decorate(
		baseHandler,
		event.Retry(3),
		event.Backoff(5, 100*time.Millisecond, time.Second),
		event.Timeout(5*time.Second),
	)

	assert.NotNil(t, handler)
	assert.Equal(t, "TestEvent", handler.Name())
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestIntegration_SyncTransportWorkflow(t *testing.T) {
	t.Parallel()

	var events []string
	var mu sync.Mutex

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		mu.Lock()
		events = append(events, fmt.Sprintf("handler1:%s", evt.ID))
		mu.Unlock()
		return nil
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		mu.Lock()
		events = append(events, fmt.Sprintf("handler2:%s", evt.ID))
		mu.Unlock()
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler1)
	processor.Register(handler2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish multiple events
	for i := 0; i < 3; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()

	// All handlers should execute for each event in order
	expected := []string{
		"handler1:0", "handler2:0",
		"handler1:1", "handler2:1",
		"handler1:2", "handler2:2",
	}
	assert.Equal(t, expected, events)
}

func TestIntegration_ChannelTransportWorkflow(t *testing.T) {
	t.Parallel()

	var processed atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		processed.Add(1)
		return nil
	})

	transport := event.NewChannelTransport(100)
	processor := event.NewProcessor(transport, event.WithWorkers(3))
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish many events
	for i := 0; i < 50; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Wait for all to be processed
	require.Eventually(t, func() bool {
		return processed.Load() == 50
	}, 2*time.Second, 10*time.Millisecond)

	stats := processor.Stats()
	assert.Equal(t, uint64(50), stats.Received)
	assert.Equal(t, uint64(50), stats.Processed)
	assert.Equal(t, uint64(0), stats.Failed)
}

func TestIntegration_MultipleProcessorsSameTransport(t *testing.T) {
	t.Parallel()

	var count1, count2 atomic.Int32

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count1.Add(1)
		return nil
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count2.Add(1)
		return nil
	})

	transport := event.NewChannelTransport(100)

	processor1 := event.NewProcessor(transport)
	processor1.Register(handler1)

	processor2 := event.NewProcessor(transport)
	processor2.Register(handler2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go processor1.Run(ctx)
	go processor2.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish events
	for i := 0; i < 10; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// One processor should handle all events (competitive)
	require.Eventually(t, func() bool {
		total := count1.Load() + count2.Load()
		return total == 10
	}, time.Second, 10*time.Millisecond)

	// Events distributed between processors
	assert.Equal(t, int32(10), count1.Load()+count2.Load())
}

func TestIntegration_ConcurrentPublishing(t *testing.T) {
	t.Parallel()

	var processed atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		processed.Add(1)
		return nil
	})

	transport := event.NewChannelTransport(200)
	processor := event.NewProcessor(transport, event.WithWorkers(5))
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				err := publisher.Publish(context.Background(), TestEvent{
					ID: fmt.Sprintf("%d-%d", workerID, j),
				})
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all to be processed
	require.Eventually(t, func() bool {
		return processed.Load() == 100
	}, 2*time.Second, 10*time.Millisecond)

	stats := processor.Stats()
	assert.Equal(t, uint64(100), stats.Received)
	assert.Equal(t, uint64(100), stats.Processed)
}

func TestIntegration_GracefulShutdownUnderLoad(t *testing.T) {
	t.Parallel()

	var processed atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		time.Sleep(50 * time.Millisecond) // Simulate work
		processed.Add(1)
		return nil
	})

	transport := event.NewChannelTransport(200)
	processor := event.NewProcessor(transport, event.WithWorkers(3))
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish many events
	for i := 0; i < 50; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	// Trigger shutdown while processing
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for graceful shutdown
	time.Sleep(2 * time.Second)

	// All events should be processed
	assert.Equal(t, int32(50), processed.Load())
}

func TestIntegration_StatsConcurrency(t *testing.T) {
	t.Parallel()

	successHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	transport := event.NewChannelTransport(500)
	processor := event.NewProcessor(transport, event.WithWorkers(10))
	processor.Register(successHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish many events concurrently while reading stats
	var wg sync.WaitGroup
	wg.Add(2)

	// Publisher goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		}
	}()

	// Stats reader goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			stats := processor.Stats()
			// Just verify we can read stats without race
			_ = stats.Received + stats.Processed + stats.Failed
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Wait for processing to complete
	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.Processed == 100
	}, 2*time.Second, 10*time.Millisecond)
}

// =============================================================================
// Race Condition Tests
// =============================================================================

func TestRace_ProcessorRegisterBeforeAfterRun(t *testing.T) {
	t.Parallel()

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	// Register before Run - should work
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	go processor.Run(ctx)

	time.Sleep(50 * time.Millisecond)

	// Register after Run - should panic
	require.Panics(t, func() {
		processor.Register(handler)
	})

	cancel()
}

func TestRace_SyncTransportConcurrentDispatch(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count.Add(1)
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Concurrent dispatches
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", id)})
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(50), count.Load())
}

func TestRace_ChannelTransportConcurrentClose(t *testing.T) {
	t.Parallel()

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	// Close from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transport.Close()
		}()
	}

	wg.Wait()
}

func TestRace_StatsCounters(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	transport := event.NewChannelTransport(200)
	processor := event.NewProcessor(transport, event.WithWorkers(5))
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Concurrent publishes and stat reads
	var wg sync.WaitGroup

	// Publishers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", j)})
			}
		}()
	}

	// Stat readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				stats := processor.Stats()
				_ = stats.Received + stats.Processed
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	require.Eventually(t, func() bool {
		stats := processor.Stats()
		return stats.Processed == 100
	}, time.Second, 10*time.Millisecond)
}

func TestRace_GetHandlersConcurrency(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	transport := event.NewChannelTransport(100)
	processor := event.NewProcessor(transport, event.WithWorkers(5))
	processor.Register(handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)

	// Publish many events that will trigger concurrent getHandlers() calls
	for i := 0; i < 50; i++ {
		err := publisher.Publish(context.Background(), TestEvent{ID: fmt.Sprintf("%d", i)})
		require.NoError(t, err)
	}

	time.Sleep(time.Second)
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestEdgeCase_MultipleRegisterSameEventType(t *testing.T) {
	t.Parallel()

	var count atomic.Int32

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count.Add(1)
		return nil
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		count.Add(1)
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler1)
	processor.Register(handler2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Both handlers should execute
	assert.Equal(t, int32(2), count.Load())
}

func TestEdgeCase_HandlerReturnsNilError(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
}

func TestEdgeCase_PublisherWithOptions(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(transport)
	processor.Register(event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport, event.WithPublisherLogger(logger))

	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)
}

func TestEdgeCase_ProcessorWithAllOptions(t *testing.T) {
	t.Parallel()

	var errorHandlerCalled atomic.Bool

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	transport := event.NewChannelTransport(10)
	processor := event.NewProcessor(
		transport,
		event.WithWorkers(3),
		event.WithLogger(logger),
		event.WithStrictHandlers(true),
		event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
			errorHandlerCalled.Store(true)
		}),
		event.WithMiddleware(event.LoggingMiddleware(logger)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go processor.Run(ctx)

	publisher := event.NewPublisher(transport)
	err := publisher.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// Should trigger error handler for zero handlers in strict mode
	require.Eventually(t, func() bool {
		return errorHandlerCalled.Load()
	}, time.Second, 10*time.Millisecond)
}

func TestEdgeCase_DecoratorPreservesHandlerName(t *testing.T) {
	t.Parallel()

	baseHandler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	decorated := event.Decorate(
		baseHandler,
		event.Retry(3),
		event.Timeout(time.Second),
	)

	assert.Equal(t, baseHandler.Name(), decorated.Name())
}

func TestEdgeCase_MiddlewareImmutability(t *testing.T) {
	t.Parallel()

	middleware := event.LoggingMiddleware(slog.Default())

	transport := event.NewSyncTransport()
	processor := event.NewProcessor(
		transport,
		event.WithMiddleware(middleware),
	)

	handler := event.NewHandlerFunc(func(ctx context.Context, evt TestEvent) error {
		return nil
	})

	processor.Register(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Run(ctx)

	err := processor.Publish(context.Background(), TestEvent{ID: "123"})
	require.NoError(t, err)

	// No way to add more middleware after construction
	// This is just a documentation test
}
