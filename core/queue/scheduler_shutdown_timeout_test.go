package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/queue"
)

// slowCheckRepo simulates a repository with slow operations
type slowCheckRepo struct {
	sync.Mutex
	tasks    map[string]*queue.Task
	slowDown time.Duration
}

func newSlowCheckRepo(slowDown time.Duration) *slowCheckRepo {
	return &slowCheckRepo{
		tasks:    make(map[string]*queue.Task),
		slowDown: slowDown,
	}
}

func (r *slowCheckRepo) CreateTask(ctx context.Context, task *queue.Task) error {
	r.Lock()
	defer r.Unlock()
	// Simulate slow operation
	time.Sleep(r.slowDown)
	r.tasks[task.TaskName] = task
	return nil
}

func (r *slowCheckRepo) GetPendingTaskByName(ctx context.Context, taskName string) (*queue.Task, error) {
	r.Lock()
	defer r.Unlock()
	// Simulate slow operation
	time.Sleep(r.slowDown)
	return r.tasks[taskName], nil
}

func TestScheduler_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	t.Run("returns error when shutdown timeout exceeded", func(t *testing.T) {
		t.Parallel()

		// Use a slow repository that takes longer than shutdown timeout
		repo := newSlowCheckRepo(150 * time.Millisecond)
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Millisecond),
			queue.WithSchedulerShutdownTimeout(50*time.Millisecond),
		)
		require.NoError(t, err)

		err = scheduler.AddTask("slow-task", queue.EveryInterval(10*time.Millisecond))
		require.NoError(t, err)

		ctx := context.Background()

		started := make(chan struct{})
		go func() {
			close(started)
			_ = scheduler.Start(ctx)
		}()

		<-started
		time.Sleep(30 * time.Millisecond) // Let it start checking

		// Stop should timeout because repository operations are slow
		err = scheduler.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown timeout")
	})

	t.Run("completes successfully with sufficient timeout", func(t *testing.T) {
		t.Parallel()

		// Use a slow repository but with enough timeout
		repo := newSlowCheckRepo(20 * time.Millisecond)
		scheduler, err := queue.NewScheduler(repo,
			queue.WithCheckInterval(10*time.Millisecond),
			queue.WithSchedulerShutdownTimeout(500*time.Millisecond), // Long enough
		)
		require.NoError(t, err)

		err = scheduler.AddTask("normal-task", queue.EveryInterval(10*time.Millisecond))
		require.NoError(t, err)

		ctx := context.Background()

		started := make(chan struct{})
		go func() {
			close(started)
			_ = scheduler.Start(ctx)
		}()

		<-started
		time.Sleep(30 * time.Millisecond) // Let it start checking

		// Stop should complete successfully
		err = scheduler.Stop()
		assert.NoError(t, err)
	})
}
