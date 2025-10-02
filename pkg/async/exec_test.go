package async_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/pkg/async"
)

func TestExecFunctionality(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	futureInt := async.Exec(ctx, 42, func(ctx context.Context, num int) error {
		time.Sleep(100 * time.Millisecond)
		if num != 42 {
			return errors.New("unexpected number")
		}
		return nil
	})

	futureString := async.Exec(ctx, "test", func(ctx context.Context, s string) error {
		time.Sleep(50 * time.Millisecond)
		if len(s) == 0 {
			return errors.New("empty string")
		}
		return nil
	})

	type MyStruct struct {
		A int
		B int
	}
	futureStruct := async.Exec(ctx, MyStruct{A: 10, B: 32}, func(ctx context.Context, data MyStruct) error {
		time.Sleep(70 * time.Millisecond)
		if data.A+data.B != 42 {
			return errors.New("sum is not 42")
		}
		return nil
	})

	errInt := futureInt.Await()
	errString := futureString.Await()
	errStruct := futureStruct.Await()

	if errInt != nil {
		t.Errorf("Unexpected error from futureInt: %v", errInt)
	}

	if errString != nil {
		t.Errorf("Unexpected error from futureString: %v", errString)
	}

	if errStruct != nil {
		t.Errorf("Unexpected error from futureStruct: %v", errStruct)
	}
}

func TestExecContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	future := async.Exec(ctx, 42, func(ctx context.Context, num int) error {
		select {
		case <-time.After(100 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	err := future.Await()

	if err == nil || err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestExecErrorPropagation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	expectedErr := errors.New("an error occurred in the exec function")

	future := async.Exec(ctx, 42, func(ctx context.Context, num int) error {
		time.Sleep(50 * time.Millisecond)
		return expectedErr
	})

	err := future.Await()

	if err == nil || err != expectedErr {
		t.Errorf("Expected error '%v', got: %v", expectedErr, err)
	}
}

func TestExecConcurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	startTime := time.Now()

	var mu sync.Mutex
	order := []string{}

	future1 := async.Exec(ctx, 1, func(ctx context.Context, num int) error {
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		order = append(order, "first")
		mu.Unlock()
		return nil
	})

	future2 := async.Exec(ctx, 2, func(ctx context.Context, num int) error {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		order = append(order, "second")
		mu.Unlock()
		return nil
	})

	future3 := async.Exec(ctx, 3, func(ctx context.Context, num int) error {
		time.Sleep(70 * time.Millisecond)
		mu.Lock()
		order = append(order, "third")
		mu.Unlock()
		return nil
	})

	_ = future1.Await()
	_ = future2.Await()
	_ = future3.Await()

	duration := time.Since(startTime)

	// Duration should be slightly longer than the longest sleep (100ms) since futures run concurrently
	if duration > 150*time.Millisecond || duration < 100*time.Millisecond {
		t.Errorf("Expected duration between 100-150ms, got %v", duration)
	}

	expectedOrder := []string{"second", "third", "first"}
	for i, v := range expectedOrder {
		if order[i] != v {
			t.Errorf("Expected order %v, got %v", expectedOrder, order)
			break
		}
	}
}

func TestExecWithCustomStruct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type Input struct {
		X int
		Y int
	}

	future := async.Exec(ctx, Input{X: 10, Y: 15}, func(ctx context.Context, in Input) error {
		time.Sleep(50 * time.Millisecond)
		if in.X+in.Y != 25 {
			return errors.New("sum is not 25")
		}
		return nil
	})

	err := future.Await()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestExecConcurrentIncrement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	counter := 0

	increment := func(_ context.Context, delta int) error {
		mu.Lock()
		defer mu.Unlock()
		counter += delta
		return nil
	}

	futures := make([]*async.ExecFuture, 0)
	for range 1000 {
		wg.Add(1)
		future := async.Exec(ctx, 1, func(ctx context.Context, delta int) error {
			defer wg.Done()
			return increment(ctx, delta)
		})
		futures = append(futures, future)
	}

	wg.Wait()

	if counter != 1000 {
		t.Errorf("Expected counter to be 1000, got %d", counter)
	}

	for _, future := range futures {
		err := future.Await()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestExecIsComplete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	future := async.Exec(ctx, 100, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	if future.IsComplete() {
		t.Error("Expected future to not be complete immediately")
	}

	err := future.Await()
	if err != nil {
		t.Errorf("Unexpected error waiting for future: %v", err)
	}

	if !future.IsComplete() {
		t.Error("Expected future to be complete after Await")
	}
}

func TestExecAwaitWithTimeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fastFuture := async.Exec(ctx, 50, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	err := fastFuture.AwaitWithTimeout(100 * time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error for fast future, got: %v", err)
	}

	slowFuture := async.Exec(ctx, 200, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	err = slowFuture.AwaitWithTimeout(100 * time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error for slow future")
	}
	if !errors.Is(err, async.ErrTimeout) {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestExecAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	future1 := async.Exec(ctx, 50, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	future2 := async.Exec(ctx, 100, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	future3 := async.Exec(ctx, 150, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	startTime := time.Now()
	err := async.ExecAll(future1, future2, future3)
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// ExecAll waits for the slowest future
	if duration < 150*time.Millisecond {
		t.Errorf("Expected duration to be at least 150ms, got %v", duration)
	}
}

func TestExecAllWithError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	expectedErr := errors.New("error from future2")

	future1 := async.Exec(ctx, 50, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	future2 := async.Exec(ctx, 100, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return expectedErr
	})

	future3 := async.Exec(ctx, 150, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	err := async.ExecAll(future1, future2, future3)

	if err == nil {
		t.Error("Expected error from ExecAll")
	}

	if err != expectedErr {
		t.Errorf("Expected error '%v', got: %v", expectedErr, err)
	}
}

func TestExecAny(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	future1 := async.Exec(ctx, 150, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	future2 := async.Exec(ctx, 50, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	future3 := async.Exec(ctx, 100, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	startTime := time.Now()
	index, err := async.ExecAny(future1, future2, future3)
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if index != 1 {
		t.Errorf("Expected index=1 (fastest future), got index=%d", index)
	}

	// ExecAny returns as soon as the fastest future completes
	if duration < 50*time.Millisecond || duration >= 100*time.Millisecond {
		t.Errorf("Expected duration to be around 50ms, got %v", duration)
	}
}

func TestExecAnyWithError(t *testing.T) {
	t.Parallel()

	// Test with empty futures list
	_, err := async.ExecAny()
	if err == nil {
		t.Error("Expected error when calling ExecAny with no futures")
	}
	if !errors.Is(err, async.ErrNoFutures) {
		t.Errorf("Expected ErrNoFutures, got: %v", err)
	}

	// Test with error returned from fastest future
	ctx := context.Background()
	expectedErr := errors.New("error from fast future")

	future1 := async.Exec(ctx, 150, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	future2 := async.Exec(ctx, 50, func(ctx context.Context, ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return expectedErr
	})

	index, err := async.ExecAny(future1, future2)

	if err != expectedErr {
		t.Errorf("Expected error '%v', got: %v", expectedErr, err)
	}

	if index != 1 {
		t.Errorf("Expected index=1, got index=%d", index)
	}
}
