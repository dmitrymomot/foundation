package async

import (
	"context"
	"sync"
	"time"
)

// ExecFuture represents the result of an asynchronous computation that only returns an error.
type ExecFuture struct {
	err  error
	once sync.Once
	done chan struct{}
}

// Await waits for the asynchronous function to complete and returns its error.
func (f *ExecFuture) Await() error {
	<-f.done
	return f.err
}

// AwaitWithTimeout waits for the asynchronous function to complete with a timeout.
// Returns the error if the function completes before the timeout.
// If the timeout occurs before completion, returns a timeout error.
func (f *ExecFuture) AwaitWithTimeout(timeout time.Duration) error {
	select {
	case <-f.done:
		return f.err
	case <-time.After(timeout):
		return ErrTimeout
	}
}

// IsComplete checks if the asynchronous function is complete without blocking.
// Returns true if the function has completed, false otherwise.
func (f *ExecFuture) IsComplete() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// Exec executes a function asynchronously that only returns an error.
// The function accepts a context.Context and a parameter of any type T, and returns error.
func Exec[T any](ctx context.Context, param T, fn func(context.Context, T) error) *ExecFuture {
	f := &ExecFuture{done: make(chan struct{})}

	go func() {
		defer close(f.done)

		// Early exit prevents goroutine leak when context is pre-canceled
		select {
		case <-ctx.Done():
			f.err = ctx.Err()
			return
		default:
		}

		err := fn(ctx, param)

		// Use sync.Once to prevent race conditions on multiple goroutine completions
		f.once.Do(func() {
			f.err = err
		})
	}()

	return f
}

// ExecAll waits for all futures to complete and returns an error
// if any of the futures returned an error.
func ExecAll(futures ...*ExecFuture) error {
	for _, future := range futures {
		if err := future.Await(); err != nil {
			return err
		}
	}
	return nil
}

// ExecAny waits for any of the futures to complete and returns the index of the completed future
// and any error it might have returned.
// Note: This function spawns one goroutine per future. All goroutines will complete naturally
// when their respective futures finish.
func ExecAny(futures ...*ExecFuture) (int, error) {
	if len(futures) == 0 {
		return -1, ErrNoFutures
	}

	done := make(chan struct {
		index int
		err   error
	})

	for i, future := range futures {
		go func(index int, f *ExecFuture) {
			err := f.Await()
			select {
			case done <- struct {
				index int
				err   error
			}{index, err}:
			default:
				// Prevents race condition where multiple futures complete simultaneously
			}
		}(i, future)
	}

	res := <-done
	return res.index, res.err
}
