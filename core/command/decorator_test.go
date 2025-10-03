package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DecoratorTestCommand struct {
	Value string
}

func TestApplyDecorators(t *testing.T) {
	t.Parallel()

	t.Run("executes handler without decorators", func(t *testing.T) {
		t.Parallel()

		executed := false
		fn := func(ctx context.Context, cmd DecoratorTestCommand) error {
			executed = true
			return nil
		}

		decorated := command.ApplyDecorators(fn)
		err := decorated(context.Background(), DecoratorTestCommand{})

		require.NoError(t, err)
		assert.True(t, executed)
	})

	t.Run("applies single decorator", func(t *testing.T) {
		t.Parallel()

		var executionOrder []string

		baseHandler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			executionOrder = append(executionOrder, "handler")
			return nil
		}

		decorator := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				executionOrder = append(executionOrder, "decorator-before")
				err := next(ctx, cmd)
				executionOrder = append(executionOrder, "decorator-after")
				return err
			}
		}

		decorated := command.ApplyDecorators(baseHandler, decorator)
		err := decorated(context.Background(), DecoratorTestCommand{})

		require.NoError(t, err)
		assert.Equal(t, []string{"decorator-before", "handler", "decorator-after"}, executionOrder)
	})

	t.Run("applies multiple decorators in correct order", func(t *testing.T) {
		t.Parallel()

		var executionOrder []string

		baseHandler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			executionOrder = append(executionOrder, "handler")
			return nil
		}

		decorator1 := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				executionOrder = append(executionOrder, "decorator1-before")
				err := next(ctx, cmd)
				executionOrder = append(executionOrder, "decorator1-after")
				return err
			}
		}

		decorator2 := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				executionOrder = append(executionOrder, "decorator2-before")
				err := next(ctx, cmd)
				executionOrder = append(executionOrder, "decorator2-after")
				return err
			}
		}

		decorator3 := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				executionOrder = append(executionOrder, "decorator3-before")
				err := next(ctx, cmd)
				executionOrder = append(executionOrder, "decorator3-after")
				return err
			}
		}

		// First decorator in list becomes outermost wrapper
		decorated := command.ApplyDecorators(baseHandler, decorator1, decorator2, decorator3)
		err := decorated(context.Background(), DecoratorTestCommand{})

		require.NoError(t, err)
		assert.Equal(t, []string{
			"decorator1-before",
			"decorator2-before",
			"decorator3-before",
			"handler",
			"decorator3-after",
			"decorator2-after",
			"decorator1-after",
		}, executionOrder)
	})

	t.Run("propagates errors through decorators", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("handler error")
		baseHandler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			return expectedErr
		}

		decorator := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				return next(ctx, cmd)
			}
		}

		decorated := command.ApplyDecorators(baseHandler, decorator)
		err := decorated(context.Background(), DecoratorTestCommand{})

		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("decorator can modify error", func(t *testing.T) {
		t.Parallel()

		baseErr := errors.New("base error")
		wrappedErr := errors.New("wrapped error")

		baseHandler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			return baseErr
		}

		decorator := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				err := next(ctx, cmd)
				if err != nil {
					return wrappedErr
				}
				return nil
			}
		}

		decorated := command.ApplyDecorators(baseHandler, decorator)
		err := decorated(context.Background(), DecoratorTestCommand{})

		assert.ErrorIs(t, err, wrappedErr)
		assert.NotErrorIs(t, err, baseErr)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	t.Run("completes within timeout", func(t *testing.T) {
		t.Parallel()

		handler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		decorated := command.ApplyDecorators(
			handler,
			command.WithTimeout[DecoratorTestCommand](100*time.Millisecond),
		)

		err := decorated(context.Background(), DecoratorTestCommand{})
		require.NoError(t, err)
	})

	t.Run("times out when handler exceeds timeout", func(t *testing.T) {
		t.Parallel()

		handler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		decorated := command.ApplyDecorators(
			handler,
			command.WithTimeout[DecoratorTestCommand](50*time.Millisecond),
		)

		err := decorated(context.Background(), DecoratorTestCommand{})
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("timeout context is cancelled after completion", func(t *testing.T) {
		t.Parallel()

		var ctxFromHandler context.Context
		handler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			ctxFromHandler = ctx
			return nil
		}

		decorated := command.ApplyDecorators(
			handler,
			command.WithTimeout[DecoratorTestCommand](1*time.Second),
		)

		err := decorated(context.Background(), DecoratorTestCommand{})
		require.NoError(t, err)

		// Context should be cancelled after handler returns
		time.Sleep(10 * time.Millisecond)
		assert.Error(t, ctxFromHandler.Err())
	})

	t.Run("respects parent context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		handler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			<-ctx.Done()
			return ctx.Err()
		}

		decorated := command.ApplyDecorators(
			handler,
			command.WithTimeout[DecoratorTestCommand](1*time.Second),
		)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := decorated(ctx, DecoratorTestCommand{})
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestDecoratorComposition(t *testing.T) {
	t.Parallel()

	t.Run("combines timeout with custom decorators", func(t *testing.T) {
		t.Parallel()

		var executionLog []string

		handler := func(ctx context.Context, cmd DecoratorTestCommand) error {
			executionLog = append(executionLog, "handler")
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		loggingDecorator := func(next command.HandlerFunc[DecoratorTestCommand]) command.HandlerFunc[DecoratorTestCommand] {
			return func(ctx context.Context, cmd DecoratorTestCommand) error {
				executionLog = append(executionLog, "logging-start")
				err := next(ctx, cmd)
				executionLog = append(executionLog, "logging-end")
				return err
			}
		}

		decorated := command.ApplyDecorators(
			handler,
			loggingDecorator,
			command.WithTimeout[DecoratorTestCommand](100*time.Millisecond),
		)

		err := decorated(context.Background(), DecoratorTestCommand{})
		require.NoError(t, err)
		assert.Equal(t, []string{"logging-start", "handler", "logging-end"}, executionLog)
	})
}
