package queue_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example payload types for testing
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type ReportPayload struct {
	Type   string    `json:"type"`
	Date   time.Time `json:"date"`
	UserID string    `json:"user_id"`
}

func TestService_BasicWorkflow(t *testing.T) {
	t.Parallel()

	// Create in-memory storage
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	// Create service with configuration
	service, err := queue.NewService(storage,
		queue.WithServiceLogger(slog.Default()),
		queue.WithSkipWorkerIfNoHandlers(false),
	)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Create a counter to track processed tasks
	var processedCount atomic.Int32

	// Register a task handler
	emailHandler := queue.NewTaskHandler(func(ctx context.Context, payload EmailPayload) error {
		processedCount.Add(1)
		assert.Equal(t, "test@example.com", payload.To)
		assert.Equal(t, "Test Subject", payload.Subject)
		return nil
	})

	err = service.RegisterHandler(emailHandler)
	require.NoError(t, err)

	// Start the service in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- service.Run(ctx)
	}()

	// Give the service time to start
	time.Sleep(100 * time.Millisecond)

	// Enqueue a task
	err = service.Enqueue(context.Background(), EmailPayload{
		To:      "test@example.com",
		Subject: "Test Subject",
		Body:    "Test body",
	})
	require.NoError(t, err)

	// Wait for task to be processed
	require.Eventually(t, func() bool {
		return processedCount.Load() == 1
	}, 2*time.Second, 100*time.Millisecond)

	// Stop the service
	cancel()

	// Wait for service to stop
	select {
	case err := <-done:
		// Context cancelled is expected
		assert.True(t, err == nil || errors.Is(err, context.Canceled))
	case <-time.After(3 * time.Second):
		t.Fatal("service did not stop in time")
	}
}

func TestService_FromConfig(t *testing.T) {
	t.Parallel()

	// Create configuration
	cfg := queue.Config{
		// Worker configuration
		PollInterval:       100 * time.Millisecond,
		LockTimeout:        time.Minute,
		MaxConcurrentTasks: 5,
		Queues:             []string{"emails", "reports"},

		// Scheduler configuration
		CheckInterval: 500 * time.Millisecond,

		// Enqueuer configuration
		DefaultQueue:    "emails",
		DefaultPriority: queue.PriorityHigh,
	}

	// Create storage and service
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	service, err := queue.NewServiceFromConfig(cfg, storage)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Verify components are accessible
	assert.NotNil(t, service.Worker())
	assert.NotNil(t, service.Scheduler())
	assert.NotNil(t, service.Enqueuer())
	assert.NotNil(t, service.Storage())
}

func TestService_MultipleHandlers(t *testing.T) {
	t.Parallel()

	storage := queue.NewMemoryStorage()
	defer storage.Close()

	// Track processed tasks
	var emailCount atomic.Int32
	var reportCount atomic.Int32

	// Create handlers
	emailHandler := queue.NewTaskHandler(func(ctx context.Context, payload EmailPayload) error {
		emailCount.Add(1)
		return nil
	})

	reportHandler := queue.NewTaskHandler(func(ctx context.Context, payload ReportPayload) error {
		reportCount.Add(1)
		return nil
	})

	// Create service with handlers
	service, err := queue.NewService(storage,
		queue.WithHandlers(emailHandler, reportHandler),
		queue.WithSkipWorkerIfNoHandlers(false),
	)
	require.NoError(t, err)

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go service.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Enqueue different task types
	err = service.Enqueue(context.Background(), EmailPayload{
		To:      "user@example.com",
		Subject: "Hello",
		Body:    "World",
	})
	require.NoError(t, err)

	err = service.Enqueue(context.Background(), ReportPayload{
		Type:   "monthly",
		Date:   time.Now(),
		UserID: "user123",
	})
	require.NoError(t, err)

	// Wait for processing
	require.Eventually(t, func() bool {
		return emailCount.Load() == 1 && reportCount.Load() == 1
	}, 2*time.Second, 100*time.Millisecond)
}

func TestService_DelayedExecution(t *testing.T) {
	t.Parallel()

	storage := queue.NewMemoryStorage()
	defer storage.Close()

	service, err := queue.NewService(storage,
		queue.WithSkipWorkerIfNoHandlers(false),
	)
	require.NoError(t, err)

	var processedAt atomic.Pointer[time.Time]

	// Register handler
	handler := queue.NewTaskHandler(func(ctx context.Context, payload EmailPayload) error {
		now := time.Now()
		processedAt.Store(&now)
		return nil
	})

	err = service.RegisterHandler(handler)
	require.NoError(t, err)

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go service.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Enqueue with delay
	enqueuedAt := time.Now()
	err = service.EnqueueWithDelay(context.Background(), EmailPayload{
		To: "delayed@example.com",
	}, 500*time.Millisecond)
	require.NoError(t, err)

	// Wait for processing
	require.Eventually(t, func() bool {
		return processedAt.Load() != nil
	}, 2*time.Second, 100*time.Millisecond)

	// Verify delay was respected
	actualDelay := processedAt.Load().Sub(enqueuedAt)
	assert.GreaterOrEqual(t, actualDelay, 500*time.Millisecond)
}

func TestService_ScheduledTask(t *testing.T) {
	t.Parallel()

	storage := queue.NewMemoryStorage()
	defer storage.Close()

	service, err := queue.NewService(storage,
		queue.WithSkipSchedulerIfNoTasks(false),
	)
	require.NoError(t, err)

	var executionCount atomic.Int32

	// Register periodic task handler
	handler := queue.NewPeriodicTaskHandler("test_periodic", func(ctx context.Context) error {
		executionCount.Add(1)
		return nil
	})

	err = service.RegisterHandler(handler)
	require.NoError(t, err)

	// Schedule task to run every 200ms
	err = service.AddScheduledTask("test_periodic",
		queue.EveryInterval(200*time.Millisecond),
		queue.WithTaskQueue("scheduled"),
		queue.WithTaskPriority(queue.PriorityHigh),
	)
	require.NoError(t, err)

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go service.Run(ctx)

	// Wait for multiple executions
	require.Eventually(t, func() bool {
		return executionCount.Load() >= 3
	}, 2*time.Second, 100*time.Millisecond)
}

func TestService_ConditionalStartup(t *testing.T) {
	t.Parallel()

	t.Run("skip worker when no handlers", func(t *testing.T) {
		storage := queue.NewMemoryStorage()
		defer storage.Close()

		service, err := queue.NewService(storage,
			queue.WithSkipWorkerIfNoHandlers(true),
		)
		require.NoError(t, err)

		// Start service without handlers
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err = service.Run(ctx)
		// Should complete without error when context times out
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	})

	t.Run("skip scheduler when no tasks", func(t *testing.T) {
		storage := queue.NewMemoryStorage()
		defer storage.Close()

		service, err := queue.NewService(storage,
			queue.WithSkipSchedulerIfNoTasks(true),
		)
		require.NoError(t, err)

		// Add a handler so worker can run
		handler := queue.NewTaskHandler(func(ctx context.Context, payload EmailPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		// Start service without scheduled tasks
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err = service.Run(ctx)
		// Should complete without error when context times out
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}

func TestService_Hooks(t *testing.T) {
	t.Parallel()

	storage := queue.NewMemoryStorage()
	defer storage.Close()

	var beforeStartCalled atomic.Bool
	var afterStopCalled atomic.Bool

	service, err := queue.NewService(storage,
		queue.WithBeforeStart(func(ctx context.Context) error {
			beforeStartCalled.Store(true)
			return nil
		}),
		queue.WithAfterStop(func() error {
			afterStopCalled.Store(true)
			return nil
		}),
	)
	require.NoError(t, err)

	// Add a handler
	handler := queue.NewTaskHandler(func(ctx context.Context, payload EmailPayload) error {
		return nil
	})
	err = service.RegisterHandler(handler)
	require.NoError(t, err)

	// Start and stop service
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = service.Run(ctx)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))

	// Verify hooks were called
	assert.True(t, beforeStartCalled.Load())
	assert.True(t, afterStopCalled.Load())
}
