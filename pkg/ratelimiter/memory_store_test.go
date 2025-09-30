package ratelimiter_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/pkg/ratelimiter"
)

func TestMemoryStore_ConsumeTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	config := ratelimiter.Config{
		Capacity:       10,
		RefillRate:     2,
		RefillInterval: 100 * time.Millisecond,
	}

	t.Run("creates new bucket with full capacity", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		remaining, resetAt, err := store.ConsumeTokens(ctx, "new-key", 3, config)
		assert.NoError(t, err)
		assert.Equal(t, 7, remaining)
		assert.NotZero(t, resetAt)
	})

	t.Run("consumes tokens correctly", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "test-consume"

		remaining, _, err := store.ConsumeTokens(ctx, key, 4, config)
		assert.NoError(t, err)
		assert.Equal(t, 6, remaining)

		remaining, _, err = store.ConsumeTokens(ctx, key, 3, config)
		assert.NoError(t, err)
		assert.Equal(t, 3, remaining)

		remaining, _, err = store.ConsumeTokens(ctx, key, 5, config)
		assert.NoError(t, err)
		assert.Equal(t, -2, remaining)
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "test-refill"

		remaining, _, err := store.ConsumeTokens(ctx, key, config.Capacity, config)
		assert.NoError(t, err)
		assert.Equal(t, 0, remaining)

		time.Sleep(config.RefillInterval + 10*time.Millisecond)

		remaining, _, err = store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, config.RefillRate, remaining)

		time.Sleep(config.RefillInterval)

		remaining, _, err = store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, config.RefillRate*2, remaining)
	})

	t.Run("caps tokens at capacity", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "test-cap"

		_, _, err := store.ConsumeTokens(ctx, key, 5, config)
		require.NoError(t, err)

		time.Sleep(config.RefillInterval * 10)

		remaining, _, err := store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, config.Capacity, remaining)
	})

	t.Run("handles zero token consumption", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "test-zero"

		remaining1, _, err := store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, config.Capacity, remaining1)

		remaining2, _, err := store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, remaining1, remaining2)
	})

	t.Run("handles negative remaining correctly", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "test-negative"

		remaining, _, err := store.ConsumeTokens(ctx, key, config.Capacity+5, config)
		assert.NoError(t, err)
		assert.Equal(t, -5, remaining)

		time.Sleep(config.RefillInterval + 10*time.Millisecond)

		remaining, _, err = store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, -5+config.RefillRate, remaining)
	})
}

func TestMemoryStore_Reset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	config := ratelimiter.Config{
		Capacity:       10,
		RefillRate:     1,
		RefillInterval: 100 * time.Millisecond,
	}

	t.Run("resets existing bucket", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "test-reset"

		_, _, err := store.ConsumeTokens(ctx, key, 8, config)
		require.NoError(t, err)

		err = store.Reset(ctx, key)
		assert.NoError(t, err)

		remaining, _, err := store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, config.Capacity, remaining)
	})

	t.Run("reset non-existent key succeeds", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		err := store.Reset(ctx, "non-existent")
		assert.NoError(t, err)
	})
}

func TestMemoryStore_StartStop(t *testing.T) {
	t.Parallel()

	t.Run("start and stop cleanup successfully", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(50 * time.Millisecond),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start in background
		go func() {
			_ = store.Start(ctx)
		}()

		// Wait for startup
		time.Sleep(10 * time.Millisecond)

		// Verify it's running
		stats := store.Stats()
		assert.True(t, stats.IsRunning)

		// Stop gracefully
		err := store.Stop()
		assert.NoError(t, err)

		// Verify it stopped
		stats = store.Stats()
		assert.False(t, stats.IsRunning)
	})

	t.Run("fails to start when already started", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(50 * time.Millisecond),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start first time
		go func() {
			_ = store.Start(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		// Try to start again
		err := store.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")

		_ = store.Stop()
	})

	t.Run("fails to stop when not started", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		err := store.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")
	})

	t.Run("fails to start with zero cleanup interval", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(0),
		)

		ctx := context.Background()
		err := store.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})
}

func TestMemoryStore_Run(t *testing.T) {
	t.Parallel()

	t.Run("run with errgroup pattern", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(50 * time.Millisecond),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Run in background
		errCh := make(chan error, 1)
		go func() {
			errCh <- store.Run(ctx)()
		}()

		// Wait for startup
		time.Sleep(10 * time.Millisecond)

		// Verify it's running
		stats := store.Stats()
		assert.True(t, stats.IsRunning)

		// Cancel context
		cancel()

		// Wait for graceful shutdown
		err := <-errCh
		assert.NoError(t, err)

		// Verify it stopped
		stats = store.Stats()
		assert.False(t, stats.IsRunning)
	})
}

func TestMemoryStore_Stats(t *testing.T) {
	t.Parallel()

	t.Run("tracks bucket creation and removal", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		ctx := context.Background()
		config := ratelimiter.Config{
			Capacity:       10,
			RefillRate:     1,
			RefillInterval: 100 * time.Millisecond,
		}

		// Create some buckets
		_, _, _ = store.ConsumeTokens(ctx, "key1", 1, config)
		_, _, _ = store.ConsumeTokens(ctx, "key2", 1, config)
		_, _, _ = store.ConsumeTokens(ctx, "key3", 1, config)

		stats := store.Stats()
		assert.Equal(t, int64(3), stats.BucketsCreated)
		assert.Equal(t, 3, stats.ActiveBuckets)
		assert.Equal(t, int64(0), stats.BucketsRemoved)
		assert.False(t, stats.IsRunning)
	})
}

func TestMemoryStore_Healthcheck(t *testing.T) {
	t.Parallel()

	t.Run("healthy when cleanup disabled", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(0),
		)

		err := store.Healthcheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("unhealthy when cleanup configured but not running", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(50 * time.Millisecond),
		)

		err := store.Healthcheck(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("healthy when cleanup running", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(50 * time.Millisecond),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start cleanup
		go func() {
			_ = store.Start(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		err := store.Healthcheck(context.Background())
		assert.NoError(t, err)

		_ = store.Stop()
	})
}

func TestMemoryStore_Close(t *testing.T) {
	t.Parallel()

	t.Run("close calls stop internally", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore(
			ratelimiter.WithCleanupInterval(50 * time.Millisecond),
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start cleanup
		go func() {
			_ = store.Start(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		// Close should stop it
		store.Close()

		stats := store.Stats()
		assert.False(t, stats.IsRunning)
	})

	t.Run("operations work without cleanup", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		ctx := context.Background()
		config := ratelimiter.Config{
			Capacity:       10,
			RefillRate:     1,
			RefillInterval: 100 * time.Millisecond,
		}

		remaining, _, err := store.ConsumeTokens(ctx, "test-key", 1, config)
		assert.NoError(t, err)
		assert.Equal(t, 9, remaining)
	})
}

func TestMemoryStore_IntegerOverflowPrevention(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("prevents overflow with large refill calculations", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		config := ratelimiter.Config{
			Capacity:       1000,
			RefillRate:     100,
			RefillInterval: time.Millisecond,
		}

		key := "overflow-test"

		_, _, err := store.ConsumeTokens(ctx, key, config.Capacity, config)
		require.NoError(t, err)

		// Sleep for 100ms to simulate many refill intervals passing
		time.Sleep(100 * time.Millisecond)

		remaining, _, err := store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		// Should be capped at capacity, not overflowed
		assert.Equal(t, config.Capacity, remaining)
	})

	t.Run("handles max int values", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		config := ratelimiter.Config{
			Capacity:       1<<31 - 1,
			RefillRate:     1000,
			RefillInterval: time.Millisecond,
		}

		key := "max-int"

		remaining, _, err := store.ConsumeTokens(ctx, key, 1, config)
		assert.NoError(t, err)
		assert.Equal(t, config.Capacity-1, remaining)
	})
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	config := ratelimiter.Config{
		Capacity:       100,
		RefillRate:     10,
		RefillInterval: 100 * time.Millisecond,
	}

	t.Run("concurrent consumption same key", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		key := "concurrent-same"
		goroutines := 10
		tokensPerGoroutine := 5

		var wg sync.WaitGroup
		wg.Add(goroutines)

		results := make([]int, goroutines)

		for i := range goroutines {
			go func(idx int) {
				defer wg.Done()
				remaining, _, err := store.ConsumeTokens(ctx, key, tokensPerGoroutine, config)
				if err == nil {
					results[idx] = remaining
				}
			}(i)
		}

		wg.Wait()

		finalRemaining, _, err := store.ConsumeTokens(ctx, key, 0, config)
		assert.NoError(t, err)
		assert.Equal(t, config.Capacity-(goroutines*tokensPerGoroutine), finalRemaining)
	})

	t.Run("concurrent different keys", func(t *testing.T) {
		store := ratelimiter.NewMemoryStore()

		goroutines := 20
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := range goroutines {
			go func(idx int) {
				defer wg.Done()
				key := "key-" + string(rune('a'+idx))

				for j := range 5 {
					_, _, err := store.ConsumeTokens(ctx, key, j+1, config)
					assert.NoError(t, err)
				}

				if idx%2 == 0 {
					err := store.Reset(ctx, key)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()
	})
}
