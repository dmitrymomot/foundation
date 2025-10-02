package event_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithEventID_EventID(t *testing.T) {
	t.Parallel()

	t.Run("store and retrieve event ID", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		expectedID := "evt_123456"

		ctx = event.WithEventID(ctx, expectedID)
		actualID := event.EventID(ctx)

		assert.Equal(t, expectedID, actualID)
	})

	t.Run("retrieve from context without event ID", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		actualID := event.EventID(ctx)

		assert.Equal(t, "", actualID, "should return empty string when event ID not present")
	})

	t.Run("overwrite existing event ID", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx = event.WithEventID(ctx, "evt_first")
		ctx = event.WithEventID(ctx, "evt_second")

		actualID := event.EventID(ctx)
		assert.Equal(t, "evt_second", actualID)
	})
}

func TestWithEventName_EventName(t *testing.T) {
	t.Parallel()

	t.Run("store and retrieve event name", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		expectedName := "UserCreated"

		ctx = event.WithEventName(ctx, expectedName)
		actualName := event.EventName(ctx)

		assert.Equal(t, expectedName, actualName)
	})

	t.Run("retrieve from context without event name", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		actualName := event.EventName(ctx)

		assert.Equal(t, "", actualName, "should return empty string when event name not present")
	})

	t.Run("overwrite existing event name", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx = event.WithEventName(ctx, "FirstEvent")
		ctx = event.WithEventName(ctx, "SecondEvent")

		actualName := event.EventName(ctx)
		assert.Equal(t, "SecondEvent", actualName)
	})
}

func TestWithEventTime_EventTime(t *testing.T) {
	t.Parallel()

	t.Run("store and retrieve event time", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		expectedTime := time.Date(2025, 10, 2, 12, 30, 0, 0, time.UTC)

		ctx = event.WithEventTime(ctx, expectedTime)
		actualTime := event.EventTime(ctx)

		assert.Equal(t, expectedTime, actualTime)
	})

	t.Run("retrieve from context without event time", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		actualTime := event.EventTime(ctx)

		assert.True(t, actualTime.IsZero(), "should return zero time when event time not present")
	})

	t.Run("overwrite existing event time", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		firstTime := time.Date(2025, 10, 2, 12, 0, 0, 0, time.UTC)
		secondTime := time.Date(2025, 10, 2, 13, 0, 0, 0, time.UTC)

		ctx = event.WithEventTime(ctx, firstTime)
		ctx = event.WithEventTime(ctx, secondTime)

		actualTime := event.EventTime(ctx)
		assert.Equal(t, secondTime, actualTime)
	})
}

func TestWithEventMeta(t *testing.T) {
	t.Parallel()

	t.Run("store all metadata at once", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		testEvent := event.Event{
			ID:        "evt_meta_123",
			Name:      "TestEvent",
			CreatedAt: time.Date(2025, 10, 2, 14, 30, 0, 0, time.UTC),
			Payload:   "test payload",
		}

		ctx = event.WithEventMeta(ctx, testEvent)

		assert.Equal(t, testEvent.ID, event.EventID(ctx))
		assert.Equal(t, testEvent.Name, event.EventName(ctx))
		assert.Equal(t, testEvent.CreatedAt, event.EventTime(ctx))
	})

	t.Run("overwrite existing metadata", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx = event.WithEventID(ctx, "old_id")
		ctx = event.WithEventName(ctx, "OldEvent")
		ctx = event.WithEventTime(ctx, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

		newEvent := event.Event{
			ID:        "new_id",
			Name:      "NewEvent",
			CreatedAt: time.Date(2025, 10, 2, 15, 0, 0, 0, time.UTC),
		}

		ctx = event.WithEventMeta(ctx, newEvent)

		assert.Equal(t, "new_id", event.EventID(ctx))
		assert.Equal(t, "NewEvent", event.EventName(ctx))
		assert.Equal(t, newEvent.CreatedAt, event.EventTime(ctx))
	})
}

func TestWithStartProcessingTime_StartProcessingTime(t *testing.T) {
	t.Parallel()

	t.Run("store and retrieve processing start time", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		expectedTime := time.Date(2025, 10, 2, 16, 45, 30, 0, time.UTC)

		ctx = event.WithStartProcessingTime(ctx, expectedTime)
		actualTime := event.StartProcessingTime(ctx)

		assert.Equal(t, expectedTime, actualTime)
	})

	t.Run("retrieve from context without processing start time", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		actualTime := event.StartProcessingTime(ctx)

		assert.True(t, actualTime.IsZero(), "should return zero time when processing start time not present")
	})

	t.Run("overwrite existing processing start time", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		firstTime := time.Date(2025, 10, 2, 16, 0, 0, 0, time.UTC)
		secondTime := time.Date(2025, 10, 2, 17, 0, 0, 0, time.UTC)

		ctx = event.WithStartProcessingTime(ctx, firstTime)
		ctx = event.WithStartProcessingTime(ctx, secondTime)

		actualTime := event.StartProcessingTime(ctx)
		assert.Equal(t, secondTime, actualTime)
	})
}

func TestApplyDecorators(t *testing.T) {
	t.Parallel()

	type testPayload struct {
		Value string
	}

	t.Run("apply multiple decorators in order", func(t *testing.T) {
		t.Parallel()

		var executionOrder []string

		// Create decorators that track execution order
		decorator1 := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				executionOrder = append(executionOrder, "decorator1_before")
				err := next(ctx, payload)
				executionOrder = append(executionOrder, "decorator1_after")
				return err
			}
		}

		decorator2 := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				executionOrder = append(executionOrder, "decorator2_before")
				err := next(ctx, payload)
				executionOrder = append(executionOrder, "decorator2_after")
				return err
			}
		}

		decorator3 := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				executionOrder = append(executionOrder, "decorator3_before")
				err := next(ctx, payload)
				executionOrder = append(executionOrder, "decorator3_after")
				return err
			}
		}

		handler := func(ctx context.Context, payload testPayload) error {
			executionOrder = append(executionOrder, "handler")
			return nil
		}

		decorated := event.ApplyDecorators(handler, decorator1, decorator2, decorator3)
		err := decorated(context.Background(), testPayload{Value: "test"})

		require.NoError(t, err)
		assert.Equal(t, []string{
			"decorator1_before",
			"decorator2_before",
			"decorator3_before",
			"handler",
			"decorator3_after",
			"decorator2_after",
			"decorator1_after",
		}, executionOrder)
	})

	t.Run("empty decorators list", func(t *testing.T) {
		t.Parallel()

		var handlerCalled bool
		handler := func(ctx context.Context, payload testPayload) error {
			handlerCalled = true
			return nil
		}

		decorated := event.ApplyDecorators(handler)
		err := decorated(context.Background(), testPayload{Value: "test"})

		require.NoError(t, err)
		assert.True(t, handlerCalled)
	})

	t.Run("decorator can modify context", func(t *testing.T) {
		t.Parallel()

		type contextKey string
		const key contextKey = "test_key"

		decorator := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				ctx = context.WithValue(ctx, key, "decorator_value")
				return next(ctx, payload)
			}
		}

		var receivedValue string
		handler := func(ctx context.Context, payload testPayload) error {
			if val, ok := ctx.Value(key).(string); ok {
				receivedValue = val
			}
			return nil
		}

		decorated := event.ApplyDecorators(handler, decorator)
		err := decorated(context.Background(), testPayload{Value: "test"})

		require.NoError(t, err)
		assert.Equal(t, "decorator_value", receivedValue)
	})

	t.Run("decorator can handle errors from handler", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("handler error")
		var errorSeen error

		decorator := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				err := next(ctx, payload)
				errorSeen = err
				return err
			}
		}

		handler := func(ctx context.Context, payload testPayload) error {
			return expectedErr
		}

		decorated := event.ApplyDecorators(handler, decorator)
		err := decorated(context.Background(), testPayload{Value: "test"})

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, expectedErr, errorSeen)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	type testPayload struct {
		Value string
	}

	t.Run("handler completes within timeout", func(t *testing.T) {
		t.Parallel()

		var handlerCalled bool
		handler := func(ctx context.Context, payload testPayload) error {
			handlerCalled = true
			// Simulate quick work
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		decorator := event.WithTimeout[testPayload](100 * time.Millisecond)
		decorated := decorator(handler)

		err := decorated(context.Background(), testPayload{Value: "test"})

		require.NoError(t, err)
		assert.True(t, handlerCalled)
	})

	t.Run("handler exceeds timeout", func(t *testing.T) {
		t.Parallel()

		handler := func(ctx context.Context, payload testPayload) error {
			// Simulate slow work that respects context
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		decorator := event.WithTimeout[testPayload](50 * time.Millisecond)
		decorated := decorator(handler)

		err := decorated(context.Background(), testPayload{Value: "test"})

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("handler respects parent context cancellation", func(t *testing.T) {
		t.Parallel()

		parentCtx, cancel := context.WithCancel(context.Background())

		handler := func(ctx context.Context, payload testPayload) error {
			// Wait for context cancellation
			<-ctx.Done()
			return ctx.Err()
		}

		decorator := event.WithTimeout[testPayload](1 * time.Second)
		decorated := decorator(handler)

		// Cancel parent context immediately
		cancel()

		err := decorated(parentCtx, testPayload{Value: "test"})

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("multiple timeout decorators use innermost timeout", func(t *testing.T) {
		t.Parallel()

		handler := func(ctx context.Context, payload testPayload) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Outer timeout is longer, inner timeout is shorter
		outerDecorator := event.WithTimeout[testPayload](1 * time.Second)
		innerDecorator := event.WithTimeout[testPayload](50 * time.Millisecond)

		decorated := event.ApplyDecorators(handler, outerDecorator, innerDecorator)

		err := decorated(context.Background(), testPayload{Value: "test"})

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestDecoratorComposition(t *testing.T) {
	t.Parallel()

	type testPayload struct {
		Value string
	}

	t.Run("chain multiple decorators with timeout", func(t *testing.T) {
		t.Parallel()

		var executionLog []string

		loggingDecorator := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				executionLog = append(executionLog, "logging_start")
				err := next(ctx, payload)
				executionLog = append(executionLog, "logging_end")
				return err
			}
		}

		metricDecorator := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				executionLog = append(executionLog, "metric_start")
				err := next(ctx, payload)
				executionLog = append(executionLog, "metric_end")
				return err
			}
		}

		handler := func(ctx context.Context, payload testPayload) error {
			executionLog = append(executionLog, "handler")
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		decorated := event.ApplyDecorators(
			handler,
			loggingDecorator,
			event.WithTimeout[testPayload](100*time.Millisecond),
			metricDecorator,
		)

		err := decorated(context.Background(), testPayload{Value: "test"})

		require.NoError(t, err)
		assert.Equal(t, []string{
			"logging_start",
			"metric_start",
			"handler",
			"metric_end",
			"logging_end",
		}, executionLog)
	})

	t.Run("decorator chain with timeout exceeded", func(t *testing.T) {
		t.Parallel()

		var cleanupCalled bool

		cleanupDecorator := func(next event.HandlerFunc[testPayload]) event.HandlerFunc[testPayload] {
			return func(ctx context.Context, payload testPayload) error {
				defer func() {
					cleanupCalled = true
				}()
				return next(ctx, payload)
			}
		}

		handler := func(ctx context.Context, payload testPayload) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		decorated := event.ApplyDecorators(
			handler,
			cleanupDecorator,
			event.WithTimeout[testPayload](50*time.Millisecond),
		)

		err := decorated(context.Background(), testPayload{Value: "test"})

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.True(t, cleanupCalled, "cleanup decorator should execute even when timeout occurs")
	})
}
