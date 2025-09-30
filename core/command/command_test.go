package command_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/command"
)

// Test command types
type (
	CreateUser struct {
		Email string
		Name  string
	}

	UpdateUser struct {
		ID    int
		Email string
	}

	DeleteUser struct {
		ID int
	}

	PanicCommand struct{}

	InvalidCommand struct{}
)

// TestHandlerRegistration tests handler registration and duplicate detection
func TestHandlerRegistration(t *testing.T) {
	t.Parallel()

	t.Run("successful registration", func(t *testing.T) {
		t.Parallel()

		d := command.NewDispatcher()
		h := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		require.NotPanics(t, func() {
			d.Register(h)
		})
	})

	t.Run("duplicate handler panics", func(t *testing.T) {
		t.Parallel()

		d := command.NewDispatcher()
		h1 := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		})
		h2 := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		})

		d.Register(h1)

		require.Panics(t, func() {
			d.Register(h2)
		}, "registering duplicate handler should panic")
	})

	t.Run("multiple different handlers", func(t *testing.T) {
		t.Parallel()

		d := command.NewDispatcher()
		h1 := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		})
		h2 := command.NewHandlerFunc(func(ctx context.Context, cmd UpdateUser) error {
			return nil
		})

		require.NotPanics(t, func() {
			d.Register(h1)
			d.Register(h2)
		})
	})
}

// TestSyncTransportExecution tests synchronous command execution
func TestSyncTransportExecution(t *testing.T) {
	t.Parallel()

	t.Run("successful execution", func(t *testing.T) {
		t.Parallel()

		executed := atomic.Bool{}
		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			executed.Store(true)
			return nil
		}))

		err := d.Dispatch(context.Background(), CreateUser{
			Email: "test@example.com",
			Name:  "Test User",
		})

		require.NoError(t, err)
		assert.True(t, executed.Load(), "handler should have executed")
	})

	t.Run("handler not found", func(t *testing.T) {
		t.Parallel()

		d := command.NewDispatcher(command.WithSyncTransport())

		err := d.Dispatch(context.Background(), InvalidCommand{})

		require.Error(t, err)
		assert.ErrorIs(t, err, command.ErrHandlerNotFound)
	})

	t.Run("handler error propagates", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("database error")
		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return expectedErr
		}))

		err := d.Dispatch(context.Background(), CreateUser{})

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before dispatch

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return ctx.Err()
		}))

		err := d.Dispatch(ctx, CreateUser{})

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("panic recovery", func(t *testing.T) {
		t.Parallel()

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd PanicCommand) error {
			panic("something went wrong")
		}))

		err := d.Dispatch(context.Background(), PanicCommand{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "panicked")
		assert.Contains(t, err.Error(), "something went wrong")
	})

	t.Run("context values propagate", func(t *testing.T) {
		t.Parallel()

		type ctxKey string
		key := ctxKey("test-key")
		expectedValue := "test-value"

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			value := ctx.Value(key)
			if value != expectedValue {
				return fmt.Errorf("expected %s, got %v", expectedValue, value)
			}
			return nil
		}))

		ctx := context.WithValue(context.Background(), key, expectedValue)
		err := d.Dispatch(ctx, CreateUser{})

		require.NoError(t, err)
	})
}

// TestChannelTransportExecution tests asynchronous channel-based execution
func TestChannelTransportExecution(t *testing.T) {
	t.Parallel()

	t.Run("successful async execution", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		executed := atomic.Bool{}
		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, 10),
		)
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			executed.Store(true)
			return nil
		}))

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err, "dispatch should not return error")

		// Wait for async execution
		require.Eventually(t, func() bool {
			return executed.Load()
		}, time.Second, 10*time.Millisecond, "handler should execute")
	})

	t.Run("buffer full error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		bufferSize := 2
		blockChan := make(chan struct{})
		startedProcessing := make(chan struct{})
		started := atomic.Bool{}

		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, bufferSize),
		)

		// Register a handler that blocks until we release it
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			if !started.Swap(true) {
				close(startedProcessing)
			}
			<-blockChan
			return nil
		}))

		// Fill the buffer
		for i := 0; i < bufferSize; i++ {
			err := d.Dispatch(context.Background(), CreateUser{})
			require.NoError(t, err)
		}

		// Wait for worker to start processing (blocking on blockChan)
		<-startedProcessing

		// Dispatch one more to fill the buffer while worker is blocked
		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err)

		// Now buffer should be full (one being processed, two in buffer)
		err = d.Dispatch(context.Background(), CreateUser{})
		require.Error(t, err)
		assert.ErrorIs(t, err, command.ErrBufferFull)

		// Cleanup: unblock handlers
		close(blockChan)
	})

	t.Run("handler not found before enqueue", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, 10),
		)

		err := d.Dispatch(context.Background(), InvalidCommand{})
		require.Error(t, err)
		assert.ErrorIs(t, err, command.ErrHandlerNotFound)
	})

	t.Run("error handler callback", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		expectedErr := errors.New("handler error")
		var mu sync.Mutex
		var callbackErr error
		var callbackCmd string
		var callbackCount atomic.Int32

		d := command.NewDispatcher(
			command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
				mu.Lock()
				callbackErr = err
				callbackCmd = cmdName
				mu.Unlock()
				callbackCount.Add(1)
			}),
			command.WithChannelTransport(ctx, 10),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return expectedErr
		}))

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err, "dispatch should not return handler error")

		require.Eventually(t, func() bool {
			return callbackCount.Load() > 0
		}, 2*time.Second, 10*time.Millisecond, "error handler callback should be called")

		mu.Lock()
		assert.ErrorIs(t, callbackErr, expectedErr)
		assert.Equal(t, "CreateUser", callbackCmd)
		mu.Unlock()
	})

	t.Run("multiple workers concurrent execution", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		workerCount := 5
		commandCount := 50
		executed := atomic.Int32{}

		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, commandCount, command.WithWorkers(workerCount)),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			executed.Add(1)
			time.Sleep(10 * time.Millisecond) // Simulate work
			return nil
		}))

		// Dispatch commands
		for i := 0; i < commandCount; i++ {
			err := d.Dispatch(context.Background(), CreateUser{})
			require.NoError(t, err)
		}

		// Wait for all to execute
		require.Eventually(t, func() bool {
			return int(executed.Load()) == commandCount
		}, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("graceful shutdown drains commands", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())

		executed := atomic.Int32{}
		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, 10),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			time.Sleep(20 * time.Millisecond)
			executed.Add(1)
			return nil
		}))

		// Dispatch several commands
		commandCount := 5
		for i := 0; i < commandCount; i++ {
			err := d.Dispatch(context.Background(), CreateUser{})
			require.NoError(t, err)
		}

		// Cancel context to trigger shutdown
		cancel()

		// Give time for draining
		time.Sleep(500 * time.Millisecond)

		// All commands should have been processed
		assert.Equal(t, int32(commandCount), executed.Load())
	})

	t.Run("dispatch context propagates to handler", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		type ctxKey string
		key := ctxKey("test-key")
		expectedValue := "test-value"
		valueReceived := make(chan string, 1)

		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, 10),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			if value := ctx.Value(key); value != nil {
				valueReceived <- value.(string)
			}
			return nil
		}))

		dispatchCtx := context.WithValue(context.Background(), key, expectedValue)
		err := d.Dispatch(dispatchCtx, CreateUser{})
		require.NoError(t, err)

		select {
		case value := <-valueReceived:
			assert.Equal(t, expectedValue, value)
		case <-time.After(time.Second):
			t.Fatal("context value not propagated")
		}
	})

	t.Run("panic in handler captured", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var mu sync.Mutex
		var panicErr error
		var panicCount atomic.Int32

		d := command.NewDispatcher(
			command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
				mu.Lock()
				panicErr = err
				mu.Unlock()
				panicCount.Add(1)
			}),
			command.WithChannelTransport(ctx, 10),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd PanicCommand) error {
			panic("handler panic")
		}))

		err := d.Dispatch(context.Background(), PanicCommand{})
		require.NoError(t, err, "dispatch should not return panic")

		require.Eventually(t, func() bool {
			return panicCount.Load() > 0
		}, 2*time.Second, 10*time.Millisecond, "panic should be captured")

		mu.Lock()
		require.Error(t, panicErr)
		assert.Contains(t, panicErr.Error(), "panicked")
		assert.Contains(t, panicErr.Error(), "handler panic")
		mu.Unlock()
	})
}

// TestMiddlewareChaining tests middleware execution order and behavior
func TestMiddlewareChaining(t *testing.T) {
	t.Parallel()

	t.Run("middleware execution order", func(t *testing.T) {
		t.Parallel()

		var execOrder []string
		mu := sync.Mutex{}

		middleware1 := func(next command.Handler) command.Handler {
			return &testHandler{
				name: next.Name(),
				fn: func(ctx context.Context, payload any) error {
					mu.Lock()
					execOrder = append(execOrder, "before-1")
					mu.Unlock()
					err := next.Handle(ctx, payload)
					mu.Lock()
					execOrder = append(execOrder, "after-1")
					mu.Unlock()
					return err
				},
			}
		}

		middleware2 := func(next command.Handler) command.Handler {
			return &testHandler{
				name: next.Name(),
				fn: func(ctx context.Context, payload any) error {
					mu.Lock()
					execOrder = append(execOrder, "before-2")
					mu.Unlock()
					err := next.Handle(ctx, payload)
					mu.Lock()
					execOrder = append(execOrder, "after-2")
					mu.Unlock()
					return err
				},
			}
		}

		d := command.NewDispatcher(
			command.WithSyncTransport(),
			command.WithMiddleware(middleware1, middleware2),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			mu.Lock()
			execOrder = append(execOrder, "handler")
			mu.Unlock()
			return nil
		}))

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err)

		expected := []string{"before-1", "before-2", "handler", "after-2", "after-1"}
		assert.Equal(t, expected, execOrder, "middleware should execute in correct order")
	})

	t.Run("middleware can short-circuit", func(t *testing.T) {
		t.Parallel()

		handlerCalled := atomic.Bool{}
		expectedErr := errors.New("middleware blocked")

		blockingMiddleware := func(next command.Handler) command.Handler {
			return &testHandler{
				name: next.Name(),
				fn: func(ctx context.Context, payload any) error {
					return expectedErr
				},
			}
		}

		d := command.NewDispatcher(
			command.WithSyncTransport(),
			command.WithMiddleware(blockingMiddleware),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			handlerCalled.Store(true)
			return nil
		}))

		err := d.Dispatch(context.Background(), CreateUser{})
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, handlerCalled.Load(), "handler should not be called")
	})

	t.Run("logging middleware integration", func(t *testing.T) {
		t.Parallel()

		// Use default logger
		logger := slog.Default()

		d := command.NewDispatcher(
			command.WithSyncTransport(),
			command.WithLogger(logger),
			command.WithMiddleware(command.LoggingMiddleware(logger)),
		)

		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			return nil
		}))

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err)
	})
}

// TestDecoratorPatterns tests handler decorators for retry, timeout, etc.
func TestDecoratorPatterns(t *testing.T) {
	t.Parallel()

	t.Run("retry decorator success after failure", func(t *testing.T) {
		t.Parallel()

		attempts := atomic.Int32{}
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			count := attempts.Add(1)
			if count < 3 {
				return errors.New("temporary failure")
			}
			return nil
		})

		decorated := command.WithRetry(handler, 5)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err)
		assert.Equal(t, int32(3), attempts.Load())
	})

	t.Run("retry decorator exhausts retries", func(t *testing.T) {
		t.Parallel()

		attempts := atomic.Int32{}
		maxRetries := 3
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			attempts.Add(1)
			return errors.New("persistent failure")
		})

		decorated := command.WithRetry(handler, maxRetries)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		err := d.Dispatch(context.Background(), CreateUser{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed after")
		assert.Equal(t, int32(maxRetries+1), attempts.Load())
	})

	t.Run("retry respects context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		attempts := atomic.Int32{}

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			count := attempts.Add(1)
			if count == 2 {
				cancel() // Cancel on second attempt
			}
			return errors.New("failure")
		})

		decorated := command.WithRetry(handler, 10)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		err := d.Dispatch(ctx, CreateUser{})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.LessOrEqual(t, int32(2), attempts.Load())
	})

	t.Run("backoff decorator increases delay", func(t *testing.T) {
		t.Parallel()

		attempts := atomic.Int32{}
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			attempts.Add(1)
			return errors.New("failure")
		})

		decorated := command.WithBackoff(handler, 3, 10*time.Millisecond, 100*time.Millisecond)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		start := time.Now()
		err := d.Dispatch(context.Background(), CreateUser{})
		duration := time.Since(start)

		require.Error(t, err)
		assert.Equal(t, int32(4), attempts.Load()) // 1 initial + 3 retries
		// Should have waited: 10ms + 20ms + 40ms = 70ms minimum
		assert.GreaterOrEqual(t, duration, 70*time.Millisecond)
	})

	t.Run("timeout decorator enforces deadline", func(t *testing.T) {
		t.Parallel()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		decorated := command.WithTimeout(handler, 50*time.Millisecond)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		start := time.Now()
		err := d.Dispatch(context.Background(), CreateUser{})
		duration := time.Since(start)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
		assert.Less(t, duration, 150*time.Millisecond)
	})

	t.Run("timeout decorator passes through success", func(t *testing.T) {
		t.Parallel()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		decorated := command.WithTimeout(handler, 100*time.Millisecond)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err)
	})

	t.Run("combining decorators", func(t *testing.T) {
		t.Parallel()

		attempts := atomic.Int32{}
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			count := attempts.Add(1)
			if count < 2 {
				return errors.New("temporary failure")
			}
			time.Sleep(5 * time.Millisecond)
			return nil
		})

		// Wrap with timeout first, then retry
		decorated := command.WithRetry(
			command.WithTimeout(handler, 50*time.Millisecond),
			3,
		)

		d := command.NewDispatcher(command.WithSyncTransport())
		d.Register(decorated)

		err := d.Dispatch(context.Background(), CreateUser{})
		require.NoError(t, err)
		assert.Equal(t, int32(2), attempts.Load())
	})
}

// TestConcurrentDispatch tests race conditions and concurrent safety
func TestConcurrentDispatch(t *testing.T) {
	t.Parallel()

	t.Run("concurrent sync dispatches", func(t *testing.T) {
		t.Parallel()

		d := command.NewDispatcher(command.WithSyncTransport())

		executed := atomic.Int32{}
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			executed.Add(1)
			time.Sleep(time.Millisecond)
			return nil
		}))

		concurrency := 50
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				err := d.Dispatch(context.Background(), CreateUser{})
				require.NoError(t, err)
			}()
		}

		wg.Wait()
		assert.Equal(t, int32(concurrency), executed.Load())
	})

	t.Run("concurrent channel dispatches", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		d := command.NewDispatcher(
			command.WithChannelTransport(ctx, 100, command.WithWorkers(5)),
		)

		executed := atomic.Int32{}
		d.Register(command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
			executed.Add(1)
			time.Sleep(time.Millisecond)
			return nil
		}))

		concurrency := 50
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				err := d.Dispatch(context.Background(), CreateUser{})
				assert.NoError(t, err)
			}()
		}

		wg.Wait()

		require.Eventually(t, func() bool {
			return int(executed.Load()) == concurrency
		}, 5*time.Second, 10*time.Millisecond)
	})
}

// Test helper types
type testHandler struct {
	name string
	fn   func(context.Context, any) error
}

func (h *testHandler) Name() string {
	return h.name
}

func (h *testHandler) Handle(ctx context.Context, payload any) error {
	return h.fn(ctx, payload)
}
