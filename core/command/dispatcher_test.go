package command_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DispatcherTestCommand struct {
	ID    string
	Value string
}

type AnotherCommand struct {
	Data string
}

func TestNewDispatcher(t *testing.T) {
	t.Parallel()

	t.Run("creates dispatcher with defaults", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		assert.NotNil(t, dispatcher)
	})

	t.Run("panics on duplicate handler registration", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler1 := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})
		handler2 := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		assert.Panics(t, func() {
			command.NewDispatcher(
				command.WithCommandSource(bus),
				command.WithHandler(handler1, handler2),
			)
		})
	})

	t.Run("allows multiple handlers for different commands", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler1 := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})
		handler2 := command.NewHandlerFunc(func(ctx context.Context, cmd AnotherCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler1, handler2),
		)

		assert.NotNil(t, dispatcher)
	})
}

func TestDispatcherStart(t *testing.T) {
	t.Parallel()

	t.Run("starts and processes commands", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		processed := make(chan DispatcherTestCommand, 1)
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			processed <- cmd
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)

		sender := command.NewSender(bus)
		payload := DispatcherTestCommand{ID: "123", Value: "test"}
		err := sender.Send(context.Background(), payload)
		require.NoError(t, err)

		select {
		case received := <-processed:
			assert.Equal(t, payload.ID, received.ID)
			assert.Equal(t, payload.Value, received.Value)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for command processing")
		}

		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("returns error when command source is nil", func(t *testing.T) {
		t.Parallel()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithHandler(handler),
		)

		err := dispatcher.Start(context.Background())
		assert.ErrorIs(t, err, command.ErrCommandSourceNil)
	})

	t.Run("returns error when no handlers registered", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
		)

		err := dispatcher.Start(context.Background())
		assert.ErrorIs(t, err, command.ErrNoHandler)
	})

	t.Run("returns error when already started", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		err := dispatcher.Start(ctx)
		assert.ErrorIs(t, err, command.ErrDispatcherAlreadyStarted)

		cancel()
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			done <- dispatcher.Start(ctx)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		select {
		case err := <-done:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(1 * time.Second):
			t.Fatal("dispatcher did not stop")
		}
	})

	t.Run("stops when command source closes", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		done := make(chan error, 1)
		go func() {
			done <- dispatcher.Start(context.Background())
		}()

		time.Sleep(50 * time.Millisecond)
		bus.Close()

		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(1 * time.Second):
			t.Fatal("dispatcher did not stop")
		}
	})
}

func TestDispatcherStop(t *testing.T) {
	t.Parallel()

	t.Run("stops dispatcher gracefully", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		go dispatcher.Start(context.Background())
		time.Sleep(50 * time.Millisecond)

		err := dispatcher.Stop()
		require.NoError(t, err)
	})

	t.Run("waits for active handlers to complete", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handlerStarted := make(chan struct{})
		handlerCanComplete := make(chan struct{})

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			close(handlerStarted)
			<-handlerCanComplete
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
			command.WithShutdownTimeout(2*time.Second),
		)

		go dispatcher.Start(context.Background())

		sender := command.NewSender(bus)
		err := sender.Send(context.Background(), DispatcherTestCommand{})
		require.NoError(t, err)

		<-handlerStarted

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- dispatcher.Stop()
		}()

		// Stop should be waiting
		select {
		case <-stopDone:
			t.Fatal("Stop returned before handler completed")
		case <-time.After(100 * time.Millisecond):
		}

		close(handlerCanComplete)

		select {
		case err := <-stopDone:
			require.NoError(t, err)
		case <-time.After(1 * time.Second):
			t.Fatal("Stop did not complete")
		}
	})

	t.Run("returns error when not started", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		err := dispatcher.Stop()
		assert.ErrorIs(t, err, command.ErrDispatcherNotStarted)
	})

	t.Run("times out if handlers take too long", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handlerStarted := make(chan struct{})

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			close(handlerStarted)
			time.Sleep(5 * time.Second)
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
			command.WithShutdownTimeout(100*time.Millisecond),
		)

		go dispatcher.Start(context.Background())

		sender := command.NewSender(bus)
		err := sender.Send(context.Background(), DispatcherTestCommand{})
		require.NoError(t, err)

		<-handlerStarted

		err = dispatcher.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown timeout exceeded")
	})
}

func TestDispatcherConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("processes multiple commands concurrently", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(100))
		defer bus.Close()

		var processed atomic.Int32
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			time.Sleep(10 * time.Millisecond)
			processed.Add(1)
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		const numCommands = 50

		for i := 0; i < numCommands; i++ {
			err := sender.Send(context.Background(), DispatcherTestCommand{ID: string(rune(i))})
			require.NoError(t, err)
		}

		// Wait for all commands to be processed
		assert.Eventually(t, func() bool {
			return processed.Load() == numCommands
		}, 2*time.Second, 50*time.Millisecond)

		cancel()
	})

	t.Run("respects max concurrent handlers limit", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(100))
		defer bus.Close()

		var activeHandlers atomic.Int32
		var maxConcurrent atomic.Int32

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			current := activeHandlers.Add(1)

			// Track maximum concurrent handlers
			for {
				max := maxConcurrent.Load()
				if current <= max || maxConcurrent.CompareAndSwap(max, current) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
			activeHandlers.Add(-1)
			return nil
		})

		const maxConcurrentLimit = 5
		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
			command.WithMaxConcurrentHandlers(maxConcurrentLimit),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		const numCommands = 20

		for i := 0; i < numCommands; i++ {
			err := sender.Send(context.Background(), DispatcherTestCommand{})
			require.NoError(t, err)
		}

		time.Sleep(500 * time.Millisecond)
		cancel()

		assert.LessOrEqual(t, maxConcurrent.Load(), int32(maxConcurrentLimit))
	})
}

func TestDispatcherErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("continues processing after handler error", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		var processed atomic.Int32
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			processed.Add(1)
			if cmd.ID == "fail" {
				return errors.New("handler error")
			}
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{ID: "success"})
		sender.Send(context.Background(), DispatcherTestCommand{ID: "fail"})
		sender.Send(context.Background(), DispatcherTestCommand{ID: "success"})

		assert.Eventually(t, func() bool {
			return processed.Load() == 3
		}, 1*time.Second, 50*time.Millisecond)

		stats := dispatcher.Stats()
		assert.Equal(t, int64(2), stats.CommandsProcessed)
		assert.Equal(t, int64(1), stats.CommandsFailed)

		cancel()
	})

	t.Run("recovers from handler panic", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		var processed atomic.Int32
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			processed.Add(1)
			if cmd.ID == "panic" {
				panic("handler panic")
			}
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{ID: "success"})
		sender.Send(context.Background(), DispatcherTestCommand{ID: "panic"})
		sender.Send(context.Background(), DispatcherTestCommand{ID: "success"})

		assert.Eventually(t, func() bool {
			return processed.Load() == 3
		}, 1*time.Second, 50*time.Millisecond)

		stats := dispatcher.Stats()
		assert.Equal(t, int64(2), stats.CommandsProcessed)
		assert.Equal(t, int64(1), stats.CommandsFailed)

		cancel()
	})

	t.Run("handles malformed JSON commands", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		var processed atomic.Int32
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			processed.Add(1)
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		// Publish malformed JSON directly to bus
		bus.Publish(context.Background(), []byte("{invalid json"))

		// Publish valid command after malformed one
		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{ID: "valid"})

		assert.Eventually(t, func() bool {
			return processed.Load() == 1
		}, 1*time.Second, 50*time.Millisecond)

		cancel()
	})
}

func TestDispatcherFallbackHandler(t *testing.T) {
	t.Parallel()

	t.Run("calls fallback for unregistered commands", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		fallbackCalled := make(chan command.Command, 1)
		fallbackHandler := func(ctx context.Context, cmd command.Command) error {
			fallbackCalled <- cmd
			return nil
		}

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
			command.WithFallbackHandler(fallbackHandler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		err := sender.Send(context.Background(), AnotherCommand{Data: "unhandled"})
		require.NoError(t, err)

		select {
		case cmd := <-fallbackCalled:
			assert.Equal(t, "AnotherCommand", cmd.Name)
		case <-time.After(1 * time.Second):
			t.Fatal("fallback handler not called")
		}

		cancel()
	})

	t.Run("starts with only fallback handler", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		fallbackCalled := make(chan struct{})
		fallbackHandler := func(ctx context.Context, cmd command.Command) error {
			close(fallbackCalled)
			return nil
		}

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithFallbackHandler(fallbackHandler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		err := sender.Send(context.Background(), DispatcherTestCommand{})
		require.NoError(t, err)

		select {
		case <-fallbackCalled:
		case <-time.After(1 * time.Second):
			t.Fatal("fallback handler not called")
		}

		cancel()
	})

	t.Run("tracks fallback handler stats", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		fallbackHandler := func(ctx context.Context, cmd command.Command) error {
			if cmd.Name == "FailCommand" {
				return errors.New("fallback error")
			}
			return nil
		}

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithFallbackHandler(fallbackHandler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{})
		sender.Send(context.Background(), AnotherCommand{Data: "test"})

		// Create a command that will fail
		bus.Publish(context.Background(), []byte(`{"id":"1","name":"FailCommand","payload":{},"created_at":"2025-01-01T00:00:00Z"}`))

		time.Sleep(200 * time.Millisecond)

		stats := dispatcher.Stats()
		assert.Equal(t, int64(2), stats.CommandsProcessed)
		assert.Equal(t, int64(1), stats.CommandsFailed)

		cancel()
	})
}

func TestDispatcherStats(t *testing.T) {
	t.Parallel()

	t.Run("tracks command statistics", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			if cmd.ID == "fail" {
				return errors.New("error")
			}
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{ID: "1"})
		sender.Send(context.Background(), DispatcherTestCommand{ID: "fail"})
		sender.Send(context.Background(), DispatcherTestCommand{ID: "2"})

		time.Sleep(200 * time.Millisecond)

		stats := dispatcher.Stats()
		assert.Equal(t, int64(2), stats.CommandsProcessed)
		assert.Equal(t, int64(1), stats.CommandsFailed)
		assert.Equal(t, int32(0), stats.ActiveCommands)
		assert.True(t, stats.IsRunning)
		assert.False(t, stats.LastActivityAt.IsZero())

		cancel()
	})

	t.Run("tracks active commands", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handlerStarted := make(chan struct{})
		handlerBlock := make(chan struct{})

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			close(handlerStarted)
			<-handlerBlock
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{})

		<-handlerStarted

		stats := dispatcher.Stats()
		assert.Equal(t, int32(1), stats.ActiveCommands)

		close(handlerBlock)
		time.Sleep(100 * time.Millisecond)

		stats = dispatcher.Stats()
		assert.Equal(t, int32(0), stats.ActiveCommands)

		cancel()
	})
}

func TestDispatcherHealthcheck(t *testing.T) {
	t.Parallel()

	t.Run("returns healthy when running", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		err := dispatcher.Healthcheck(context.Background())
		assert.NoError(t, err)

		cancel()
	})

	t.Run("returns error when not running", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		err := dispatcher.Healthcheck(context.Background())
		assert.ErrorIs(t, err, command.ErrHealthcheckFailed)
		assert.ErrorIs(t, err, command.ErrDispatcherNotRunning)
	})

	t.Run("detects stale dispatcher", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
			command.WithStaleThreshold(100*time.Millisecond),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		// Process a command to set last activity
		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{})
		time.Sleep(50 * time.Millisecond)

		// Should be healthy
		err := dispatcher.Healthcheck(context.Background())
		assert.NoError(t, err)

		// Wait for stale threshold
		time.Sleep(150 * time.Millisecond)

		// Should be stale
		err = dispatcher.Healthcheck(context.Background())
		assert.ErrorIs(t, err, command.ErrHealthcheckFailed)
		assert.ErrorIs(t, err, command.ErrDispatcherStale)

		cancel()
	})

	t.Run("detects stuck dispatcher", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(100))
		defer bus.Close()

		blockHandlers := make(chan struct{})

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			<-blockHandlers
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
			command.WithStuckThreshold(5),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		for i := 0; i < 10; i++ {
			sender.Send(context.Background(), DispatcherTestCommand{})
		}

		time.Sleep(100 * time.Millisecond)

		err := dispatcher.Healthcheck(context.Background())
		assert.ErrorIs(t, err, command.ErrHealthcheckFailed)
		assert.ErrorIs(t, err, command.ErrDispatcherStuck)

		close(blockHandlers)
		cancel()
	})
}

func TestDispatcherRun(t *testing.T) {
	t.Parallel()

	t.Run("integrates with errgroup pattern", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		processed := make(chan struct{})
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			close(processed)
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			runFunc := dispatcher.Run(ctx)
			err := runFunc()
			assert.NoError(t, err)
		}()

		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{})

		<-processed
		cancel()
		wg.Wait()
	})

	t.Run("handles graceful shutdown via context", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus()
		defer bus.Close()

		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			runFunc := dispatcher.Run(ctx)
			done <- runFunc()
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not complete")
		}
	})
}

func TestDispatcherContextPropagation(t *testing.T) {
	t.Parallel()

	t.Run("handler receives context with command metadata", func(t *testing.T) {
		t.Parallel()

		bus := command.NewChannelBus(command.WithBufferSize(10))
		defer bus.Close()

		metadataReceived := make(chan bool, 1)
		handler := command.NewHandlerFunc(func(ctx context.Context, cmd DispatcherTestCommand) error {
			commandID := command.CommandID(ctx)
			commandName := command.CommandName(ctx)
			commandTime := command.CommandTime(ctx)
			startTime := command.StartProcessingTime(ctx)

			metadataReceived <- (commandID != "" && commandName == "DispatcherTestCommand" &&
				!commandTime.IsZero() && !startTime.IsZero())
			return nil
		})

		dispatcher := command.NewDispatcher(
			command.WithCommandSource(bus),
			command.WithHandler(handler),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go dispatcher.Start(ctx)
		time.Sleep(50 * time.Millisecond)

		sender := command.NewSender(bus)
		sender.Send(context.Background(), DispatcherTestCommand{})

		select {
		case hasMetadata := <-metadataReceived:
			assert.True(t, hasMetadata)
		case <-time.After(1 * time.Second):
			t.Fatal("handler not called")
		}

		cancel()
	})
}
