package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// bucket represents a token bucket state.
type bucket struct {
	tokens     int
	lastRefill time.Time
	lastAccess time.Time // Used by cleanup to identify stale buckets
}

// MemoryStore implements Store interface using in-memory storage.
type MemoryStore struct {
	mu      sync.RWMutex
	buckets map[string]*bucket

	// Configuration
	cleanupInterval time.Duration
	shutdownTimeout time.Duration
	logger          *slog.Logger

	// State management
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool
	wg      sync.WaitGroup

	// Observability metrics
	bucketsCreated atomic.Int64
	bucketsRemoved atomic.Int64
}

// MemoryStoreStats provides observability metrics for monitoring and debugging
type MemoryStoreStats struct {
	BucketsCreated int64 // Total number of buckets created
	BucketsRemoved int64 // Total number of stale buckets removed
	ActiveBuckets  int   // Current number of active buckets
	IsRunning      bool  // Whether the cleanup goroutine is running
}

// MemoryStoreOption configures a MemoryStore.
type MemoryStoreOption func(*MemoryStore)

// WithCleanupInterval sets the cleanup interval for removing stale buckets.
// Set to 0 to disable automatic cleanup.
func WithCleanupInterval(interval time.Duration) MemoryStoreOption {
	return func(ms *MemoryStore) {
		ms.cleanupInterval = interval
	}
}

// WithMemoryStoreShutdownTimeout sets the graceful shutdown timeout.
func WithMemoryStoreShutdownTimeout(timeout time.Duration) MemoryStoreOption {
	return func(ms *MemoryStore) {
		if timeout > 0 {
			ms.shutdownTimeout = timeout
		}
	}
}

// WithMemoryStoreLogger sets the logger for internal operations.
func WithMemoryStoreLogger(logger *slog.Logger) MemoryStoreOption {
	return func(ms *MemoryStore) {
		if logger != nil {
			ms.logger = logger
		}
	}
}

// NewMemoryStore creates a new in-memory store.
// Call Start() to begin background cleanup.
func NewMemoryStore(opts ...MemoryStoreOption) *MemoryStore {
	ms := &MemoryStore{
		buckets:         make(map[string]*bucket),
		cleanupInterval: 5 * time.Minute,
		shutdownTimeout: 30 * time.Second,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(ms)
	}

	return ms
}

// ConsumeTokens attempts to consume tokens from the bucket.
func (ms *MemoryStore) ConsumeTokens(ctx context.Context, key string, tokens int, config Config) (remaining int, resetAt time.Time, err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	b, exists := ms.buckets[key]

	if !exists {
		b = &bucket{
			tokens:     config.Capacity,
			lastRefill: now,
			lastAccess: now,
		}
		ms.buckets[key] = b
		ms.bucketsCreated.Add(1)
	}

	// Token bucket algorithm: calculate how many refill intervals have passed
	// and add the corresponding tokens, then consume the requested amount
	elapsed := now.Sub(b.lastRefill)
	// Cap intervals to prevent integer overflow in high-capacity/low-rate scenarios
	maxIntervals := int64(config.Capacity/config.RefillRate + 1)
	intervalsElapsed := int(min(int64(elapsed/config.RefillInterval), maxIntervals))

	if intervalsElapsed > 0 {
		tokensToAdd := intervalsElapsed * config.RefillRate
		b.tokens = min(b.tokens+tokensToAdd, config.Capacity)
		b.lastRefill = now // Prevent time drift accumulation
	}

	b.tokens -= tokens
	remaining = b.tokens
	b.lastAccess = now

	resetAt = b.lastRefill.Add(config.RefillInterval)

	return remaining, resetAt, nil
}

func (ms *MemoryStore) Reset(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.buckets, key)
	return nil
}

// Start begins the background cleanup goroutine. This is a blocking operation
// that runs until the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (ms *MemoryStore) Start(ctx context.Context) error {
	ms.mu.Lock()
	if ms.cancel != nil {
		ms.mu.Unlock()
		return fmt.Errorf("memory store already started")
	}

	// Skip starting if cleanup is disabled
	if ms.cleanupInterval <= 0 {
		ms.mu.Unlock()
		return fmt.Errorf("cleanup interval must be > 0, got %v (use WithCleanupInterval to configure)", ms.cleanupInterval)
	}

	ms.ctx, ms.cancel = context.WithCancel(ctx)
	ms.mu.Unlock()

	ms.running.Store(true)
	defer ms.running.Store(false)

	ms.logger.InfoContext(ms.ctx, "memory store cleanup started",
		slog.Duration("cleanup_interval", ms.cleanupInterval))

	ticker := time.NewTicker(ms.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ms.ctx.Done():
			ms.logger.InfoContext(context.Background(), "memory store cleanup stopping")
			return ms.ctx.Err()
		case <-ticker.C:
			ms.cleanupWithWait()
		}
	}
}

// Stop gracefully shuts down the background cleanup with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (ms *MemoryStore) Stop() error {
	ms.mu.Lock()
	if ms.cancel == nil {
		ms.mu.Unlock()
		return fmt.Errorf("memory store not started")
	}

	cancel := ms.cancel
	ms.cancel = nil
	ms.mu.Unlock()

	// Cancel context to stop main loop
	cancel()

	// Wait for any in-progress cleanup to complete with timeout
	ms.logger.InfoContext(context.Background(), "memory store stopping, waiting for cleanup to complete",
		slog.Duration("timeout", ms.shutdownTimeout))

	ctx, ctxCancel := context.WithTimeout(context.Background(), ms.shutdownTimeout)
	defer ctxCancel()

	done := make(chan struct{})
	go func() {
		ms.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ms.logger.InfoContext(context.Background(), "memory store stopped cleanly")
		return nil
	case <-ctx.Done():
		ms.logger.WarnContext(context.Background(), "memory store shutdown timeout exceeded",
			slog.Duration("timeout", ms.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", ms.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the cleanup, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (ms *MemoryStore) Run(ctx context.Context) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- ms.Start(ctx)
		}()

		select {
		case <-ctx.Done():
			// Context cancelled - perform graceful shutdown
			_ = ms.Stop() // Ignore stop error in normal shutdown
			<-errCh       // Wait for Start() to exit
			return nil
		case err := <-errCh:
			// Start() returned - check if it's a normal shutdown
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
	}
}

// cleanupWithWait is a wrapper around removeStale that tracks the operation with WaitGroup
func (ms *MemoryStore) cleanupWithWait() {
	ms.mu.RLock()
	if ms.cancel == nil {
		ms.mu.RUnlock()
		return
	}
	ms.wg.Add(1)
	ms.mu.RUnlock()

	defer ms.wg.Done()
	ms.removeStale()
}

// removeStale removes buckets that haven't been accessed recently to prevent memory leaks.
// Buckets are considered stale if they haven't been accessed for 1 hour.
// This threshold balances memory efficiency with avoiding premature cleanup of
// infrequently-used but still active rate limit keys.
func (ms *MemoryStore) removeStale() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	const staleThreshold = 1 * time.Hour // Buckets unused for 1+ hour are cleaned up

	removed := 0
	for key, b := range ms.buckets {
		if now.Sub(b.lastAccess) > staleThreshold {
			delete(ms.buckets, key)
			removed++
		}
	}

	if removed > 0 {
		ms.bucketsRemoved.Add(int64(removed))
	}
}

// Stats returns current memory store statistics for observability and monitoring.
// This method is thread-safe and can be called at any time.
func (ms *MemoryStore) Stats() MemoryStoreStats {
	ms.mu.RLock()
	isRunning := ms.cancel != nil
	activeBuckets := len(ms.buckets)
	ms.mu.RUnlock()

	return MemoryStoreStats{
		BucketsCreated: ms.bucketsCreated.Load(),
		BucketsRemoved: ms.bucketsRemoved.Load(),
		ActiveBuckets:  activeBuckets,
		IsRunning:      isRunning,
	}
}

// Healthcheck validates that the memory store is operational.
// Returns nil if healthy, or an error describing the health issue.
// This method is thread-safe and suitable for use in health check endpoints.
func (ms *MemoryStore) Healthcheck(ctx context.Context) error {
	stats := ms.Stats()

	// If cleanup is configured but not running, it's unhealthy
	if ms.cleanupInterval > 0 && !stats.IsRunning {
		return fmt.Errorf("cleanup is configured but not running")
	}

	return nil
}

// Close stops the cleanup goroutine. Deprecated: Use Stop() instead.
func (ms *MemoryStore) Close() {
	_ = ms.Stop()
}
