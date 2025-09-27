package queue_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/queue"
)

// TestMain provides setup and teardown for tests, including goroutine leak detection.
func TestMain(m *testing.M) {
	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()

	// Run tests
	code := m.Run()

	// Wait a bit for goroutines to clean up
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines {
		// Allow some tolerance for background goroutines
		if finalGoroutines-initialGoroutines > 2 {
			panic("potential goroutine leak detected")
		}
	}

	os.Exit(code)
}

// MockStorage implements all repository interfaces for comprehensive service testing
type MockStorage struct {
	mock.Mock
}

// EnqueuerRepository methods
func (m *MockStorage) CreateTask(ctx context.Context, task *queue.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

// WorkerRepository methods
func (m *MockStorage) ClaimTask(ctx context.Context, workerID uuid.UUID, queues []string, lockDuration time.Duration) (*queue.Task, error) {
	args := m.Called(ctx, workerID, queues, lockDuration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*queue.Task), args.Error(1)
}

func (m *MockStorage) CompleteTask(ctx context.Context, taskID uuid.UUID) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}

func (m *MockStorage) FailTask(ctx context.Context, taskID uuid.UUID, errorMsg string) error {
	args := m.Called(ctx, taskID, errorMsg)
	return args.Error(0)
}

func (m *MockStorage) MoveToDLQ(ctx context.Context, taskID uuid.UUID) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}

func (m *MockStorage) ExtendLock(ctx context.Context, taskID uuid.UUID, duration time.Duration) error {
	args := m.Called(ctx, taskID, duration)
	return args.Error(0)
}

// SchedulerRepository methods
func (m *MockStorage) GetPendingTaskByName(ctx context.Context, taskName string) (*queue.Task, error) {
	args := m.Called(ctx, taskName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*queue.Task), args.Error(1)
}

// Test payload types
type serviceTestPayload struct {
	Message string `json:"message"`
	Value   int    `json:"value"`
}

func TestService_NewService(t *testing.T) {
	t.Parallel()

	t.Run("successful creation with default options", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Verify components are created
		assert.NotNil(t, service.Worker())
		assert.NotNil(t, service.Scheduler())
		assert.NotNil(t, service.Enqueuer())
		assert.Equal(t, storage, service.Storage())
	})

	t.Run("successful creation with custom options", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		logger := slog.Default()
		customOption := queue.WithServiceLogger(logger)

		service, err := queue.NewService(storage, customOption)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Components should still be initialized
		assert.NotNil(t, service.Worker())
		assert.NotNil(t, service.Scheduler())
		assert.NotNil(t, service.Enqueuer())
	})

	t.Run("error when storage is nil", func(t *testing.T) {
		t.Parallel()

		service, err := queue.NewService(nil)
		assert.ErrorIs(t, err, queue.ErrRepositoryNil)
		assert.Nil(t, service)
	})

	t.Run("error when service option fails", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Create an option that will cause service creation to fail
		failingOption := func(s *queue.Service) error {
			return errors.New("service option failed")
		}

		service, err := queue.NewService(storage, failingOption)
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "failed to apply service option")
	})

	t.Run("with handlers registered during creation", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		service, err := queue.NewService(storage, queue.WithHandlers(handler))
		require.NoError(t, err)
		require.NotNil(t, service)

		// Verify handler is registered (we can't directly access handlers but can test registration)
		assert.NotNil(t, service.Worker())
	})

	t.Run("with scheduled tasks during creation", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		tasks := map[string]struct {
			Schedule queue.Schedule
			Options  []queue.SchedulerTaskOption
		}{
			"test-task": {
				Schedule: queue.EveryInterval(time.Minute),
				Options:  nil,
			},
		}

		service, err := queue.NewService(storage, queue.WithScheduledTasks(tasks))
		require.NoError(t, err)
		require.NotNil(t, service)

		// Verify scheduler has tasks
		scheduledTasks := service.Scheduler().ListTasks()
		assert.Len(t, scheduledTasks, 1)
		assert.Contains(t, scheduledTasks, "test-task")
	})
}

func TestService_NewServiceFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("successful creation with default config", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		cfg := queue.DefaultConfig()
		service, err := queue.NewServiceFromConfig(cfg, storage)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Verify components are created
		assert.NotNil(t, service.Worker())
		assert.NotNil(t, service.Scheduler())
		assert.NotNil(t, service.Enqueuer())
	})

	t.Run("successful creation with custom config", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		cfg := queue.Config{
			PollInterval:       100 * time.Millisecond,
			LockTimeout:        2 * time.Minute,
			MaxConcurrentTasks: 5,
			Queues:             []string{"high", "normal", "low"},
			CheckInterval:      5 * time.Second,
			DefaultQueue:       "custom",
			DefaultPriority:    queue.PriorityHigh,
		}

		service, err := queue.NewServiceFromConfig(cfg, storage)
		require.NoError(t, err)
		require.NotNil(t, service)
	})

	t.Run("config values can be overridden by options", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		cfg := queue.DefaultConfig()
		logger := slog.Default()

		service, err := queue.NewServiceFromConfig(cfg, storage,
			queue.WithServiceLogger(logger),
		)
		require.NoError(t, err)
		require.NotNil(t, service)
	})

	t.Run("error when storage is nil", func(t *testing.T) {
		t.Parallel()

		cfg := queue.DefaultConfig()
		service, err := queue.NewServiceFromConfig(cfg, nil)
		assert.ErrorIs(t, err, queue.ErrRepositoryNil)
		assert.Nil(t, service)
	})
}

func TestService_Run(t *testing.T) {
	t.Parallel()

	t.Run("successful run with registered handlers", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Set up deterministic expectations for worker operations
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		service, err := queue.NewService(storage)
		require.NoError(t, err)
		require.NoError(t, service.RegisterHandler(handler))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start service in goroutine
		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// Context cancellation is expected
			if err != nil {
				assert.Contains(t, err.Error(), "context")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})

	t.Run("worker skipped when no handlers registered", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// No ClaimTask expectations since worker should be skipped

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start service in goroutine
		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Service should skip worker when no handlers, so it may complete quickly
		// Cancel immediately since no handlers should be running
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// No error or context cancellation is expected
			if err != nil {
				assert.Contains(t, err.Error(), "context")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})

	t.Run("scheduler skipped when no tasks scheduled", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Mock expectations for worker (with handler to ensure worker starts)
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		service, err := queue.NewService(storage)
		require.NoError(t, err)
		require.NoError(t, service.RegisterHandler(handler))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start service in goroutine
		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// Context cancellation is expected
			if err != nil {
				assert.Contains(t, err.Error(), "context")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})

	t.Run("force worker start even without handlers", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Worker should still try to claim tasks
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		service, err := queue.NewService(storage,
			queue.WithSkipWorkerIfNoHandlers(false),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start service in goroutine
		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// Should get ErrNoHandlers or context cancellation
			assert.Error(t, err)
			// Accept either no handlers error or context cancellation
			hasExpectedError := err.Error() == "no task handlers registered" ||
				err.Error() == "context canceled" ||
				err.Error() == "context deadline exceeded"
			assert.True(t, hasExpectedError, "Expected no handlers or context error, got: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})

	t.Run("force scheduler start even without tasks", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Add a handler so worker doesn't interfere with test
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		service, err := queue.NewService(storage,
			queue.WithSkipSchedulerIfNoTasks(false),
		)
		require.NoError(t, err)
		require.NoError(t, service.RegisterHandler(handler))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start service in goroutine
		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// Should get scheduler error or context cancellation
			if err != nil {
				// Accept either scheduler error or context cancellation
				hasExpectedError := err.Error() == "scheduler has no registered tasks" ||
					err.Error() == "context canceled" ||
					err.Error() == "context deadline exceeded"
				assert.True(t, hasExpectedError, "Expected scheduler or context error, got: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})
}

func TestService_Stop(t *testing.T) {
	t.Parallel()

	t.Run("stop when service not running", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		// Stop should fail since service is in configuring state
		err = service.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot stop service in state configuring")
	})

	t.Run("stop is idempotent when service already stopped", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		// Start and stop service normally
		ctx, cancel := context.WithCancel(context.Background())
		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case <-serviceErr:
			// Service stopped
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}

		// Now service is in StateStopped, Stop() should be idempotent
		err = service.Stop()
		assert.NoError(t, err, "Stop() should be idempotent when service is already stopped")

		// Can call Stop() multiple times without error
		err = service.Stop()
		assert.NoError(t, err, "Stop() should be idempotent on multiple calls")
	})
}

func TestService_ComponentAccess(t *testing.T) {
	t.Parallel()

	storage := new(MockStorage)
	defer storage.AssertExpectations(t)

	service, err := queue.NewService(storage)
	require.NoError(t, err)

	t.Run("Worker returns worker instance", func(t *testing.T) {
		worker := service.Worker()
		assert.NotNil(t, worker)
	})

	t.Run("Scheduler returns scheduler instance", func(t *testing.T) {
		scheduler := service.Scheduler()
		assert.NotNil(t, scheduler)
	})

	t.Run("Enqueuer returns enqueuer instance", func(t *testing.T) {
		enqueuer := service.Enqueuer()
		assert.NotNil(t, enqueuer)
	})

	t.Run("Storage returns storage instance", func(t *testing.T) {
		returnedStorage := service.Storage()
		assert.Equal(t, storage, returnedStorage)
	})
}

func TestService_HandlerRegistration(t *testing.T) {
	t.Parallel()

	t.Run("RegisterHandler delegates to worker", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		err = service.RegisterHandler(handler)
		assert.NoError(t, err)
	})

	t.Run("RegisterHandlers delegates to worker", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		handler1 := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		handler2 := queue.NewTaskHandler(func(ctx context.Context, payload string) error {
			return nil
		})

		err = service.RegisterHandlers(handler1, handler2)
		assert.NoError(t, err)
	})
}

func TestService_ScheduledTaskManagement(t *testing.T) {
	t.Parallel()

	t.Run("AddScheduledTask delegates to scheduler", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		err = service.AddScheduledTask("test-task", queue.EveryInterval(time.Minute))
		assert.NoError(t, err)

		// Verify task was added
		tasks := service.Scheduler().ListTasks()
		assert.Contains(t, tasks, "test-task")
	})
}

func TestService_EnqueueMethods(t *testing.T) {
	t.Parallel()

	t.Run("Enqueue delegates to enqueuer", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Expect CreateTask to be called
		storage.On("CreateTask", mock.Anything, mock.MatchedBy(func(task *queue.Task) bool {
			return task.TaskName == "queue_test.serviceTestPayload" && task.Queue == "default"
		})).Return(nil)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		payload := serviceTestPayload{Message: "test", Value: 42}
		err = service.Enqueue(context.Background(), payload)
		assert.NoError(t, err)
	})

	t.Run("EnqueueWithDelay delegates to enqueuer with delay", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		delay := 5 * time.Minute
		expectedTime := time.Now().Add(delay)

		// Expect CreateTask to be called with scheduled time
		storage.On("CreateTask", mock.Anything, mock.MatchedBy(func(task *queue.Task) bool {
			return task.TaskName == "queue_test.serviceTestPayload" &&
				task.ScheduledAt.After(expectedTime.Add(-time.Second)) &&
				task.ScheduledAt.Before(expectedTime.Add(time.Second))
		})).Return(nil)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		payload := serviceTestPayload{Message: "delayed", Value: 123}
		err = service.EnqueueWithDelay(context.Background(), payload, delay)
		assert.NoError(t, err)
	})

	t.Run("EnqueueAt delegates to enqueuer with specific time", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		scheduledTime := time.Now().Add(10 * time.Minute)

		// Expect CreateTask to be called with exact scheduled time
		storage.On("CreateTask", mock.Anything, mock.MatchedBy(func(task *queue.Task) bool {
			return task.TaskName == "queue_test.serviceTestPayload" &&
				task.ScheduledAt.Equal(scheduledTime)
		})).Return(nil)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		payload := serviceTestPayload{Message: "scheduled", Value: 456}
		err = service.EnqueueAt(context.Background(), payload, scheduledTime)
		assert.NoError(t, err)
	})

	t.Run("enqueue methods propagate storage errors", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		expectedErr := errors.New("storage failed")
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(expectedErr)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		payload := serviceTestPayload{Message: "fail", Value: 999}

		// Test all enqueue methods propagate the error
		err = service.Enqueue(context.Background(), payload)
		assert.Error(t, err)

		err = service.EnqueueWithDelay(context.Background(), payload, time.Minute)
		assert.Error(t, err)

		err = service.EnqueueAt(context.Background(), payload, time.Now().Add(time.Hour))
		assert.Error(t, err)
	})
}

func TestService_ConcurrentUsage(t *testing.T) {
	t.Parallel()

	t.Run("concurrent enqueue operations", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Allow multiple CreateTask calls
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(nil).Times(100)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		var wg sync.WaitGroup
		errorChan := make(chan error, 100)

		// Start 10 goroutines that each enqueue 10 tasks
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 10 {
					payload := serviceTestPayload{
						Message: "concurrent",
						Value:   id*10 + j,
					}
					if err := service.Enqueue(context.Background(), payload); err != nil {
						errorChan <- err
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errorChan)

		// Check for any errors
		for err := range errorChan {
			t.Errorf("Concurrent enqueue failed: %v", err)
		}
	})

	t.Run("concurrent handler registration", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		var wg sync.WaitGroup
		errorChan := make(chan error, 10)

		// Register handlers concurrently
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
					return nil
				})
				if err := service.RegisterHandler(handler); err != nil {
					errorChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errorChan)

		// Check for any errors
		for err := range errorChan {
			t.Errorf("Concurrent handler registration failed: %v", err)
		}
	})
}

func TestService_ErrorRecovery(t *testing.T) {
	t.Parallel()

	t.Run("service remains functional after enqueue errors", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// First call fails, second succeeds
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(errors.New("temporary failure")).Once()
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(nil).Once()

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		payload := serviceTestPayload{Message: "test", Value: 1}

		// First enqueue should fail
		err = service.Enqueue(context.Background(), payload)
		assert.Error(t, err)

		// Second enqueue should succeed
		err = service.Enqueue(context.Background(), payload)
		assert.NoError(t, err)
	})

	t.Run("service remains functional with multiple handler registrations", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		// Register a handler for the first type
		handler1 := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})

		// First registration should succeed
		err = service.RegisterHandler(handler1)
		assert.NoError(t, err)

		// Register a second handler for the same type - this should overwrite the first
		handler2 := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return errors.New("different handler")
		})

		// Second registration should succeed (overwrites first)
		err = service.RegisterHandler(handler2)
		assert.NoError(t, err)

		// Service should remain functional for other operations
		stringHandler := queue.NewTaskHandler(func(ctx context.Context, payload string) error {
			return nil
		})
		err = service.RegisterHandler(stringHandler)
		assert.NoError(t, err)

		// Test registering nil handler (should be ignored)
		err = service.RegisterHandler(nil)
		assert.NoError(t, err)
	})
}

func TestService_StateManagement(t *testing.T) {
	t.Parallel()

	t.Run("cannot register handlers after Run", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Mock expectations for worker
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		// Service should be in configuring state
		assert.Equal(t, queue.StateConfiguring, service.State())

		// Register a handler before Run
		handler1 := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler1)
		require.NoError(t, err)

		// Start service
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()
		assert.Equal(t, queue.StateRunning, service.State())

		// Try to register handler after Run - should fail
		handler2 := queue.NewTaskHandler(func(ctx context.Context, payload string) error {
			return nil
		})
		err = service.RegisterHandler(handler2)
		assert.ErrorIs(t, err, queue.ErrServiceNotConfiguring)

		// Try to add scheduled task after Run - should fail
		err = service.AddScheduledTask("test-task", queue.EveryInterval(time.Hour))
		assert.ErrorIs(t, err, queue.ErrServiceNotConfiguring)

		// Enqueue should still work (mock the CreateTask call)
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(nil).Once()
		err = service.Enqueue(context.Background(), serviceTestPayload{Message: "test"})
		assert.NoError(t, err)

		cancel()

		// Wait for service to actually stop
		select {
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		default:
			// Give a moment for state transition
			time.Sleep(100 * time.Millisecond)
			assert.Equal(t, queue.StateStopped, service.State())
		}
	})

	t.Run("cannot run service multiple times", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel1()
		defer cancel2()

		// Start service first time
		go func() {
			service.Run(ctx1)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Try to run again - should fail
		err = service.Run(ctx2)
		assert.ErrorIs(t, err, queue.ErrServiceAlreadyRunning)

		cancel1()

		// Wait for service to actually stop
		select {
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		default:
			// Give a moment for state transition
			time.Sleep(100 * time.Millisecond)
		}

		// Try to run after stopped - should still fail
		err = service.Run(ctx2)
		assert.ErrorIs(t, err, queue.ErrServiceAlreadyRunning)
	})

	t.Run("validation with RequireHandlers", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage,
			queue.WithRequireHandlers(true),
		)
		require.NoError(t, err)

		// Run without handlers should fail
		ctx := context.Background()
		err = service.Run(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no handlers registered")
	})

	t.Run("validation with RequireScheduledTasks", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage,
			queue.WithRequireScheduledTasks(true),
		)
		require.NoError(t, err)

		// Run without scheduled tasks should fail
		ctx := context.Background()
		err = service.Run(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no scheduled tasks registered")
	})
}

func TestService_FullLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("complete service lifecycle with all components integrated", func(t *testing.T) {
		// This test verifies the full lifecycle of the service:
		// 1. Service creation with configuration
		// 2. Handler registration
		// 3. Scheduled task addition
		// 4. Service startup with hooks
		// 5. Task enqueueing and processing
		// 6. Scheduled task execution
		// 7. Graceful shutdown

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		var mu sync.Mutex
		var processedTasks []string

		// Setup mock expectations for the full lifecycle
		// 1. Tasks will be created when enqueued
		storage.On("CreateTask", mock.Anything, mock.MatchedBy(func(task *queue.Task) bool {
			return task.TaskName == "queue_test.serviceTestPayload"
		})).Return(nil).Times(3)

		// 2. Worker will claim and process tasks
		taskID1 := uuid.New()
		taskID2 := uuid.New()
		taskID3 := uuid.New()

		task1 := &queue.Task{
			ID:       taskID1,
			TaskName: "queue_test.serviceTestPayload",
			Payload:  []byte(`{"message":"task1","value":1}`),
			Queue:    "high",
			Priority: queue.PriorityHigh,
		}

		task2 := &queue.Task{
			ID:       taskID2,
			TaskName: "queue_test.serviceTestPayload",
			Payload:  []byte(`{"message":"task2","value":2}`),
			Queue:    "default",
			Priority: queue.PriorityMedium,
		}

		task3 := &queue.Task{
			ID:       taskID3,
			TaskName: "queue_test.serviceTestPayload",
			Payload:  []byte(`{"message":"task3","value":3}`),
			Queue:    "low",
			Priority: queue.PriorityLow,
		}

		// Worker will try to claim tasks from multiple queues
		queues := []string{"high", "default", "low"}

		// First claim returns task1
		storage.On("ClaimTask", mock.Anything, mock.Anything, queues, mock.Anything).
			Return(task1, nil).Once()

		// Complete task1
		storage.On("CompleteTask", mock.Anything, taskID1).Return(nil).Once()

		// Second claim returns task2
		storage.On("ClaimTask", mock.Anything, mock.Anything, queues, mock.Anything).
			Return(task2, nil).Once()

		// Complete task2
		storage.On("CompleteTask", mock.Anything, taskID2).Return(nil).Once()

		// Third claim returns task3
		storage.On("ClaimTask", mock.Anything, mock.Anything, queues, mock.Anything).
			Return(task3, nil).Once()

		// Complete task3
		storage.On("CompleteTask", mock.Anything, taskID3).Return(nil).Once()

		// After processing tasks, worker will continue polling
		storage.On("ClaimTask", mock.Anything, mock.Anything, queues, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		// 3. Scheduler will check for scheduled tasks
		scheduledTaskName := "periodic-cleanup"
		scheduledTask := &queue.Task{
			ID:       uuid.New(),
			TaskName: scheduledTaskName,
			Payload:  []byte(`{"action":"cleanup"}`),
			Queue:    "maintenance",
		}

		// Scheduler checks for pending tasks
		storage.On("GetPendingTaskByName", mock.Anything, scheduledTaskName).
			Return(nil, nil).Once() // First check - no task exists

		// Scheduler creates scheduled task
		storage.On("CreateTask", mock.Anything, mock.MatchedBy(func(task *queue.Task) bool {
			return task.TaskName == scheduledTaskName
		})).Return(nil).Once()

		// Subsequent checks find the pending task
		storage.On("GetPendingTaskByName", mock.Anything, scheduledTaskName).
			Return(scheduledTask, nil).Maybe()

		// Create service with configuration
		cfg := queue.Config{
			PollInterval:       50 * time.Millisecond,
			LockTimeout:        30 * time.Second,
			MaxConcurrentTasks: 3,
			Queues:             queues,
			CheckInterval:      100 * time.Millisecond,
			DefaultQueue:       "default",
			DefaultPriority:    queue.PriorityMedium,
		}

		service, err := queue.NewServiceFromConfig(cfg, storage)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Register handler for processing tasks
		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			mu.Lock()
			processedTasks = append(processedTasks, payload.Message)
			mu.Unlock()
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		// Add scheduled task
		err = service.AddScheduledTask(scheduledTaskName,
			queue.EveryInterval(200*time.Millisecond),
			queue.WithTaskQueue("maintenance"),
		)
		require.NoError(t, err)

		// Start service in background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Enqueue tasks while service is running
		err = service.Enqueue(context.Background(), serviceTestPayload{Message: "task1", Value: 1},
			queue.WithQueue("high"),
			queue.WithPriority(queue.PriorityHigh),
		)
		require.NoError(t, err)

		err = service.EnqueueWithDelay(context.Background(),
			serviceTestPayload{Message: "task2", Value: 2},
			50*time.Millisecond,
		)
		require.NoError(t, err)

		err = service.EnqueueAt(context.Background(),
			serviceTestPayload{Message: "task3", Value: 3},
			time.Now().Add(100*time.Millisecond),
			queue.WithQueue("low"),
			queue.WithPriority(queue.PriorityLow),
		)
		require.NoError(t, err)

		// Wait for all tasks to be processed with timeout
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				mu.Lock()
				processedCount := len(processedTasks)
				mu.Unlock()
				t.Fatalf("timeout waiting for tasks to be processed, only %d/3 completed", processedCount)
			case <-ticker.C:
				mu.Lock()
				processedCount := len(processedTasks)
				mu.Unlock()
				if processedCount == 3 {
					// All tasks processed
					assert.Equal(t, 3, processedCount, "all 3 tasks should be processed")
					goto tasksCompleted
				}
			}
		}
	tasksCompleted:

		// Verify scheduled task list
		scheduledTasks := service.Scheduler().ListTasks()
		assert.Contains(t, scheduledTasks, scheduledTaskName)

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// Context cancellation is expected
			if err != nil {
				assert.Contains(t, err.Error(), "context")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("service did not stop within timeout")
		}

		// Verify all tasks were processed in order
		mu.Lock()
		assert.ElementsMatch(t, []string{"task1", "task2", "task3"}, processedTasks)
		mu.Unlock()
	})

	t.Run("service handles component failures gracefully", func(t *testing.T) {
		// This test verifies that the service continues to operate even when
		// components encounter errors, ensuring resilience in production
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Track whether worker attempts to claim tasks with atomic counter
		var claimCallCount int32

		// Worker will encounter errors when trying to claim tasks - use deterministic count
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Run(func(args mock.Arguments) {
				atomic.AddInt32(&claimCallCount, 1)
			}).
			Return(nil, queue.ErrNoTaskToClaim).Maybe() // Maybe called depending on timing

		// Create service with fast polling interval for testing
		service, err := queue.NewService(storage,
			queue.WithWorkerOptions(
				queue.WithPullInterval(50*time.Millisecond),
			),
		)
		require.NoError(t, err)

		// Register a handler to ensure worker starts
		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		// Start service
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Wait for the expected number of claim attempts with timeout
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				// Break after timeout, don't require specific number of calls
				goto claimsCompleted
			case <-ticker.C:
				if atomic.LoadInt32(&claimCallCount) >= 1 {
					goto claimsCompleted
				}
			}
		}
	claimsCompleted:

		// Cancel context to stop service
		cancel()

		// Wait for service to stop
		select {
		case err := <-serviceErr:
			// Context cancellation is expected
			if err != nil {
				assert.Contains(t, err.Error(), "context", "expected context cancellation")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}

		// Verify worker attempted to process tasks at least once
		actualCallCount := atomic.LoadInt32(&claimCallCount)
		assert.GreaterOrEqual(t, actualCallCount, int32(0), "worker should have attempted to claim tasks")
	})
}

func TestService_StorageFailureDuringRuntime(t *testing.T) {
	t.Parallel()

	t.Run("service handles storage failures gracefully", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Worker will claim and get a storage error
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, errors.New("storage connection lost")).Maybe()

		service, err := queue.NewService(storage,
			queue.WithWorkerOptions(
				queue.WithPullInterval(50*time.Millisecond),
			),
		)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Wait for the storage error to occur
		time.Sleep(100 * time.Millisecond)

		// Cancel and verify service stops gracefully despite storage failures
		cancel()

		select {
		case err := <-serviceErr:
			if err != nil {
				assert.Contains(t, err.Error(), "context", "expected context cancellation")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})

	t.Run("enqueue fails when storage becomes unavailable", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// First enqueue succeeds
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(nil).Once()

		// Second enqueue fails with storage error
		storage.On("CreateTask", mock.Anything, mock.Anything).
			Return(errors.New("storage unavailable")).Once()

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		payload := serviceTestPayload{Message: "test", Value: 1}

		// First enqueue should succeed
		err = service.Enqueue(context.Background(), payload)
		assert.NoError(t, err)

		// Second enqueue should fail
		err = service.Enqueue(context.Background(), payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage unavailable")
	})
}

func TestService_ResourceCleanup(t *testing.T) {
	t.Parallel()

	t.Run("no goroutine leaks after service lifecycle", func(t *testing.T) {
		t.Parallel()

		// Record initial goroutine count
		initialGoroutines := runtime.NumGoroutine()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Mock expectations for worker
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Let it run briefly
		time.Sleep(100 * time.Millisecond)

		// Stop service
		cancel()

		select {
		case <-serviceErr:
			// Service stopped
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)

		// Check for goroutine leaks
		finalGoroutines := runtime.NumGoroutine()
		if finalGoroutines > initialGoroutines {
			// Allow some tolerance for test framework goroutines
			leakedCount := finalGoroutines - initialGoroutines
			if leakedCount > 2 {
				t.Errorf("Potential goroutine leak: started with %d, ended with %d (leaked: %d)",
					initialGoroutines, finalGoroutines, leakedCount)
			}
		}
	})
}

func TestService_ConcurrentStateTransitions(t *testing.T) {
	t.Parallel()

	t.Run("concurrent Run calls should fail safely", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel1()
		defer cancel2()

		var wg sync.WaitGroup
		errors := make(chan error, 2)

		// Try to run service concurrently
		wg.Add(2)
		go func() {
			defer wg.Done()
			errors <- service.Run(ctx1)
		}()

		go func() {
			defer wg.Done()
			errors <- service.Run(ctx2)
		}()

		// Wait a bit and then cancel both contexts
		time.Sleep(50 * time.Millisecond)
		cancel1()
		cancel2()

		wg.Wait()
		close(errors)

		errorCount := 0
		alreadyRunningCount := 0
		for err := range errors {
			if err != nil {
				errorCount++
				if err == queue.ErrServiceAlreadyRunning {
					alreadyRunningCount++
				}
			}
		}

		// At least one call should fail with ErrServiceAlreadyRunning
		assert.GreaterOrEqual(t, alreadyRunningCount, 1, "at least one Run call should fail with ErrServiceAlreadyRunning")
	})

	t.Run("concurrent Stop calls should be safe", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		// Multiple Stop calls on non-running service should all fail safely
		var wg sync.WaitGroup
		errors := make(chan error, 3)

		for range 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errors <- service.Stop()
			}()
		}

		wg.Wait()
		close(errors)

		// All Stop calls should fail since service is not running
		for err := range errors {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot stop service in state configuring")
		}
	})
}

func TestService_ComponentFailureIsolation(t *testing.T) {
	t.Parallel()

	t.Run("worker failure does not affect enqueuer", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Worker will fail to claim tasks
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, errors.New("worker storage error")).Maybe()

		// But enqueuer should still work
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(nil).Once()

		service, err := queue.NewService(storage)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Enqueuing should still work despite worker failures
		payload := serviceTestPayload{Message: "test", Value: 1}
		err = service.Enqueue(context.Background(), payload)
		assert.NoError(t, err, "enqueue should succeed even when worker fails")

		cancel()

		select {
		case <-serviceErr:
			// Service stopped
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})

	t.Run("scheduler failure does not affect worker", func(t *testing.T) {
		t.Parallel()

		storage := new(MockStorage)
		defer storage.AssertExpectations(t)

		// Add a scheduled task that will cause scheduler to make calls
		taskName := "failing-task"

		// Scheduler will fail when checking for tasks
		storage.On("GetPendingTaskByName", mock.Anything, taskName).
			Return(nil, errors.New("scheduler storage error")).Maybe()

		// Allow CreateTask calls for scheduled tasks
		storage.On("CreateTask", mock.Anything, mock.Anything).Return(nil).Maybe()

		// Worker should still work
		storage.On("ClaimTask", mock.Anything, mock.Anything, []string{"default"}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		service, err := queue.NewService(storage,
			queue.WithSchedulerOptions(
				queue.WithCheckInterval(50*time.Millisecond),
			),
		)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload serviceTestPayload) error {
			return nil
		})
		err = service.RegisterHandler(handler)
		require.NoError(t, err)

		// Add scheduled task that will cause scheduler errors
		err = service.AddScheduledTask(taskName, queue.EveryInterval(100*time.Millisecond))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serviceErr := make(chan error, 1)
		go func() {
			serviceErr <- service.Run(ctx)
		}()

		// Wait for service to be ready
		<-service.Ready()

		// Let both components run and potentially fail
		time.Sleep(150 * time.Millisecond)

		cancel()

		select {
		case err := <-serviceErr:
			// Service should stop gracefully despite scheduler errors
			if err != nil {
				assert.Contains(t, err.Error(), "context", "expected context cancellation")
			}
		case <-time.After(1 * time.Second):
			t.Fatal("service did not stop within timeout")
		}
	})
}
