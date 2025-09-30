package queue_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/queue"
)

// Mock repository for scheduler tests
type mockSchedulerRepo struct {
	mu         sync.Mutex
	tasks      map[uuid.UUID]*queue.Task
	createFunc func(ctx context.Context, task *queue.Task) error
}

func newMockSchedulerRepo() *mockSchedulerRepo {
	return &mockSchedulerRepo{
		tasks: make(map[uuid.UUID]*queue.Task),
	}
}

func (m *mockSchedulerRepo) CreateTask(ctx context.Context, task *queue.Task) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, task)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.ID] = task
	return nil
}

func (m *mockSchedulerRepo) GetPendingTaskByName(ctx context.Context, taskName string) (*queue.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.TaskName == taskName && task.Status == queue.TaskStatusPending {
			return task, nil
		}
	}
	return nil, nil
}

func (m *mockSchedulerRepo) countTasksByName(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, task := range m.tasks {
		if task.TaskName == name {
			count++
		}
	}
	return count
}

func (m *mockSchedulerRepo) getTasksByName(name string) []*queue.Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	var tasks []*queue.Task
	for _, task := range m.tasks {
		if task.TaskName == name {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func TestScheduler_NewScheduler(t *testing.T) {
	t.Parallel()

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)
		require.NotNil(t, scheduler)
	})

	t.Run("nil repository error", func(t *testing.T) {
		t.Parallel()

		scheduler, err := queue.NewScheduler(nil)
		assert.ErrorIs(t, err, queue.ErrRepositoryNil)
		assert.Nil(t, scheduler)
	})

	t.Run("with options", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Second),
		)
		require.NoError(t, err)
		require.NotNil(t, scheduler)
	})
}

func TestScheduler_AddTask(t *testing.T) {
	t.Parallel()

	t.Run("add periodic task successfully", func(t *testing.T) {

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		err = scheduler.AddTask("daily-report", queue.DailyAt(9, 0))
		assert.NoError(t, err)

		// Verify task is in list
		tasks := scheduler.ListTasks()
		assert.Contains(t, tasks, "daily-report")
	})

	t.Run("add task with options", func(t *testing.T) {

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		err = scheduler.AddTask("high-priority-task", queue.EveryMinute(),
			queue.WithTaskQueue("priority-queue"),
			queue.WithTaskPriority(queue.PriorityMax),
			queue.WithTaskMaxRetries(5),
		)
		assert.NoError(t, err)
	})

	t.Run("duplicate task error", func(t *testing.T) {

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		err = scheduler.AddTask("duplicate-task", queue.EveryHours(1))
		require.NoError(t, err)

		err = scheduler.AddTask("duplicate-task", queue.EveryHours(2))
		assert.ErrorIs(t, err, queue.ErrTaskAlreadyRegistered)
	})
}

func TestScheduler_Start(t *testing.T) {
	t.Parallel()

	t.Run("start without tasks error", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = scheduler.Start(ctx)
		assert.ErrorIs(t, err, queue.ErrSchedulerNotConfigured)
	})

	t.Run("creates task on schedule", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(20*time.Millisecond),
		)
		require.NoError(t, err)

		// Add a task that runs every 30ms
		err = scheduler.AddTask("frequent-task", queue.EveryInterval(30*time.Millisecond))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		// Start scheduler in background
		startDone := make(chan struct{})
		go func() {
			_ = scheduler.Start(ctx)
			close(startDone)
		}()

		// Wait for at least one task to be created
		deadline := time.Now().Add(120 * time.Millisecond)
		var count int
		for time.Now().Before(deadline) {
			count = repo.countTasksByName("frequent-task")
			if count >= 1 {
				// Success - at least the immediate task was created
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// If still no tasks, wait a bit more and check once more
		if count == 0 {
			time.Sleep(20 * time.Millisecond)
			count = repo.countTasksByName("frequent-task")
		}

		// Should have created at least 1 task (immediate)
		assert.GreaterOrEqual(t, count, 1, "should have created at least 1 task")

		// Wait for scheduler to finish
		<-startDone
	})

	t.Run("prevents duplicate pending tasks", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(50*time.Millisecond),
		)
		require.NoError(t, err)

		// Add a task
		err = scheduler.AddTask("no-duplicate", queue.EveryMinute())
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Start scheduler
		go func() {
			_ = scheduler.Start(ctx)
		}()

		// Wait for multiple check cycles
		time.Sleep(30 * time.Millisecond)

		// Should only have 1 task (no duplicates)
		count := repo.countTasksByName("no-duplicate")
		assert.Equal(t, 1, count, "should only create one task when pending exists")
	})

	t.Run("respects schedule timing", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(50*time.Millisecond),
		)
		require.NoError(t, err)

		// Add a daily task scheduled for far future
		tomorrow := time.Now().Add(24 * time.Hour)
		err = scheduler.AddTask("tomorrow-task", queue.DailyAt(tomorrow.Hour(), tomorrow.Minute()))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Start scheduler
		go func() {
			_ = scheduler.Start(ctx)
		}()

		// Wait for check cycles
		time.Sleep(30 * time.Millisecond)

		// Task should be created but scheduled for future
		tasks := repo.getTasksByName("tomorrow-task")
		require.Len(t, tasks, 1)
		assert.True(t, tasks[0].ScheduledAt.After(time.Now()), "task should be scheduled for future")
	})
}

func TestScheduler_RemoveTask(t *testing.T) {
	t.Parallel()

	t.Run("remove task successfully", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		// Add and then remove task
		err = scheduler.AddTask("temp-task", queue.EveryHours(1))
		require.NoError(t, err)

		tasks := scheduler.ListTasks()
		assert.Contains(t, tasks, "temp-task")

		scheduler.RemoveTask("temp-task")

		tasks = scheduler.ListTasks()
		assert.NotContains(t, tasks, "temp-task")
	})

	t.Run("remove non-existent task", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		// Should not panic
		scheduler.RemoveTask("non-existent")
	})
}

func TestScheduler_ListTasks(t *testing.T) {
	t.Parallel()

	t.Run("list all registered tasks", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		// Add multiple tasks
		err = scheduler.AddTask("task1", queue.EveryMinute())
		require.NoError(t, err)
		err = scheduler.AddTask("task2", queue.Hourly())
		require.NoError(t, err)
		err = scheduler.AddTask("task3", queue.Daily())
		require.NoError(t, err)

		tasks := scheduler.ListTasks()
		assert.Len(t, tasks, 3)
		assert.Contains(t, tasks, "task1")
		assert.Contains(t, tasks, "task2")
		assert.Contains(t, tasks, "task3")
	})

	t.Run("empty list when no tasks", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		tasks := scheduler.ListTasks()
		assert.Empty(t, tasks)
	})
}

func TestScheduler_TaskCreation(t *testing.T) {
	t.Parallel()

	t.Run("creates periodic task with correct fields", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(50*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("test-periodic", queue.EveryMinute(),
			queue.WithTaskQueue("periodic-queue"),
			queue.WithTaskPriority(queue.PriorityHigh),
			queue.WithTaskMaxRetries(5),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		go func() {
			_ = scheduler.Start(ctx)
		}()

		// Wait for task creation
		time.Sleep(20 * time.Millisecond)

		// Verify created task
		tasks := repo.getTasksByName("test-periodic")
		require.Len(t, tasks, 1)

		task := tasks[0]
		assert.Equal(t, "periodic-queue", task.Queue)
		assert.Equal(t, queue.TaskTypePeriodic, task.TaskType)
		assert.Equal(t, "test-periodic", task.TaskName)
		assert.Nil(t, task.Payload) // Periodic tasks have no payload
		assert.Equal(t, queue.TaskStatusPending, task.Status)
		assert.Equal(t, queue.PriorityHigh, task.Priority)
		assert.Equal(t, int8(0), task.RetryCount)
		assert.Equal(t, int8(5), task.MaxRetries)
		assert.False(t, task.CreatedAt.IsZero())
	})

	t.Run("handles repository errors", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		repo.createFunc = func(ctx context.Context, task *queue.Task) error {
			return errors.New("database error")
		}

		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(50*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("error-task", queue.EveryMinute())
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start should handle errors gracefully
		go func() {
			_ = scheduler.Start(ctx)
		}()

		// Wait for error to occur
		time.Sleep(20 * time.Millisecond)

		// Should not crash, task count should be 0
		assert.Equal(t, 0, repo.countTasksByName("error-task"))
	})
}

func TestScheduler_ScheduleTypes(t *testing.T) {
	t.Parallel()

	t.Run("different schedule types", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		// Test various schedule types
		schedules := []struct {
			name     string
			schedule queue.Schedule
		}{
			{"every-minute", queue.EveryMinute()},
			{"every-5-minutes", queue.EveryMinutes(5)},
			{"hourly", queue.Hourly()},
			{"hourly-at-30", queue.HourlyAt(30)},
			{"daily", queue.Daily()},
			{"daily-at-14-30", queue.DailyAt(14, 30)},
			{"weekly-monday", queue.Weekly(time.Monday)},
			{"weekly-friday-17", queue.WeeklyOn(time.Friday, 17, 0)},
			{"monthly-1st", queue.Monthly(1)},
			{"monthly-15th-noon", queue.MonthlyOn(15, 12, 0)},
		}

		for _, tc := range schedules {
			err := scheduler.AddTask(tc.name, tc.schedule)
			assert.NoError(t, err, "failed to add task: %s", tc.name)
		}

		tasks := scheduler.ListTasks()
		assert.Len(t, tasks, len(schedules))
	})
}

func TestScheduler_ConcurrentOperations(t *testing.T) {
	t.Parallel()

	t.Run("concurrent task additions", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		const numGoroutines = 10
		errChan := make(chan error, numGoroutines)

		for i := range numGoroutines {
			go func(n int) {
				taskName := fmt.Sprintf("concurrent-task-%d", n)
				err := scheduler.AddTask(taskName, queue.EveryMinute())
				errChan <- err
			}(i)
		}

		// Collect errors
		for range numGoroutines {
			err := <-errChan
			assert.NoError(t, err)
		}

		// Verify all tasks added
		tasks := scheduler.ListTasks()
		assert.Len(t, tasks, numGoroutines)
	})

	t.Run("concurrent add and remove", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		// Pre-add some tasks
		for i := range 5 {
			err := scheduler.AddTask(fmt.Sprintf("task-%d", i), queue.EveryMinute())
			require.NoError(t, err)
		}

		// Concurrently add and remove
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 5; i < 10; i++ {
				_ = scheduler.AddTask(fmt.Sprintf("task-%d", i), queue.EveryMinute())
			}
		}()

		go func() {
			defer wg.Done()
			for i := range 5 {
				scheduler.RemoveTask(fmt.Sprintf("task-%d", i))
			}
		}()

		wg.Wait()

		// Should have tasks 5-9
		tasks := scheduler.ListTasks()
		assert.GreaterOrEqual(t, len(tasks), 5)
	})
}

func TestSchedulerWithLogger(t *testing.T) {
	t.Parallel()

	// Create a mock repo that implements SchedulerRepository
	repo := newMockSchedulerRepo()

	// Create a custom logger
	customLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	scheduler, err := queue.NewScheduler(repo, queue.WithSchedulerLogger(customLogger))
	require.NoError(t, err)

	// The scheduler should be created successfully with the custom logger
	assert.NotNil(t, scheduler)

	// Add a task to verify logger is used
	err = scheduler.AddTask("test-periodic", queue.EveryInterval(50*time.Millisecond))
	require.NoError(t, err)

	// Just verify it was created with the logger, don't need to run it
}

func TestScheduler_Stop(t *testing.T) {
	t.Parallel()

	t.Run("stops running scheduler", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("test-task", queue.EveryInterval(10*time.Millisecond))
		require.NoError(t, err)

		ctx := context.Background()

		// Start scheduler in background
		started := make(chan struct{})
		stopped := make(chan error, 1)
		go func() {
			close(started)
			err := scheduler.Start(ctx)
			stopped <- err
		}()

		// Wait for scheduler to start
		<-started
		time.Sleep(30 * time.Millisecond) // Let it run a bit

		// Stop the scheduler
		err = scheduler.Stop()
		require.NoError(t, err)

		// Start should return context canceled error
		select {
		case err := <-stopped:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("scheduler did not stop in time")
		}
	})

	t.Run("error when not started", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		err = scheduler.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")
	})

	t.Run("prevents multiple starts", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("test-task", queue.EveryInterval(10*time.Millisecond))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start scheduler
		started := make(chan struct{})
		go func() {
			close(started)
			_ = scheduler.Start(ctx)
		}()

		<-started
		time.Sleep(10 * time.Millisecond)

		// Try to start again - should fail
		ctx2 := context.Background()
		err = scheduler.Start(ctx2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")

		// Clean up
		_ = scheduler.Stop()
	})
}

func TestScheduler_Run(t *testing.T) {
	t.Parallel()

	t.Run("runs and stops with context", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("test-task", queue.EveryInterval(10*time.Millisecond))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Run scheduler
		runFunc := scheduler.Run(ctx)
		err = runFunc()

		// Should return nil after graceful shutdown via context
		assert.NoError(t, err)

		// Verify scheduler created tasks
		assert.Greater(t, repo.countTasksByName("test-task"), 0)
	})

	t.Run("returns error if start fails", func(t *testing.T) {
		t.Parallel()

		repo := newMockSchedulerRepo()
		scheduler, err := queue.NewScheduler(repo)
		require.NoError(t, err)

		// Don't add any tasks - Start should fail with ErrSchedulerNotConfigured

		ctx := context.Background()
		runFunc := scheduler.Run(ctx)
		err = runFunc()

		assert.ErrorIs(t, err, queue.ErrSchedulerNotConfigured)
	})

	t.Run("waits for in-progress checks", func(t *testing.T) {
		t.Parallel()

		checkStarted := make(chan struct{})
		checkCompleted := make(chan struct{})

		repo := newMockSchedulerRepo()
		repo.createFunc = func(ctx context.Context, task *queue.Task) error {
			// Signal that check started
			select {
			case <-checkStarted:
				// Already signaled
			default:
				close(checkStarted)
			}

			// Simulate slow operation
			time.Sleep(30 * time.Millisecond)

			// Signal completion
			select {
			case <-checkCompleted:
				// Already signaled
			default:
				close(checkCompleted)
			}

			return nil
		}

		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("slow-task", queue.EveryInterval(10*time.Millisecond))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Start scheduler
		runDone := make(chan error, 1)
		go func() {
			runFunc := scheduler.Run(ctx)
			runDone <- runFunc()
		}()

		// Wait for check to start
		<-checkStarted

		// Cancel context while check is in progress
		cancel()

		// Wait for Run to complete
		select {
		case err := <-runDone:
			assert.NoError(t, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("scheduler did not stop in time")
		}

		// Verify check was allowed to complete
		select {
		case <-checkCompleted:
			// Good, check completed
		default:
			t.Fatal("scheduler did not wait for in-progress check")
		}
	})
}
