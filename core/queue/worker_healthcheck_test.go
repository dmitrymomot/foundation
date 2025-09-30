package queue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/queue"
)

func TestWorker_Healthcheck(t *testing.T) {
	t.Parallel()

	t.Run("healthy worker", func(t *testing.T) {
		t.Parallel()

		mockRepo := new(MockWorkerRepository)
		defer mockRepo.AssertExpectations(t)

		worker, err := queue.NewWorker(mockRepo,
			queue.WithMaxConcurrentTasks(5),
		)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload testPayload) error {
			return nil
		})
		require.NoError(t, worker.RegisterHandler(handler))

		mockRepo.On("ClaimTask", mock.Anything, mock.Anything, []string{queue.DefaultQueueName}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_ = worker.Start(ctx)
		}()

		// Wait for worker to start
		deadline := time.Now().Add(100 * time.Millisecond)
		for !worker.Stats().IsRunning && time.Now().Before(deadline) {
			time.Sleep(1 * time.Millisecond)
		}

		// Check health
		err = worker.Healthcheck(context.Background())
		assert.NoError(t, err, "healthy worker should pass healthcheck")

		_ = worker.Stop()
	})

	t.Run("worker not running", func(t *testing.T) {
		t.Parallel()

		mockRepo := new(MockWorkerRepository)
		defer mockRepo.AssertExpectations(t)

		worker, err := queue.NewWorker(mockRepo)
		require.NoError(t, err)

		handler := queue.NewTaskHandler(func(ctx context.Context, payload testPayload) error {
			return nil
		})
		require.NoError(t, worker.RegisterHandler(handler))

		// Worker not started yet
		err = worker.Healthcheck(context.Background())
		assert.Error(t, err)
		assert.ErrorIs(t, err, queue.ErrHealthcheckFailed)
		assert.ErrorIs(t, err, queue.ErrWorkerNotRunning)
	})

	t.Run("worker overloaded", func(t *testing.T) {
		t.Parallel()

		mockRepo := new(MockWorkerRepository)
		defer mockRepo.AssertExpectations(t)

		// Create worker with max 2 concurrent tasks
		worker, err := queue.NewWorker(mockRepo,
			queue.WithMaxConcurrentTasks(2),
			queue.WithPullInterval(5*time.Millisecond),
		)
		require.NoError(t, err)

		// Create 2 tasks that will block
		tasks := make([]*queue.Task, 2)
		for i := range 2 {
			payload := testPayload{Message: "block", Value: i}
			payloadBytes, _ := json.Marshal(payload)
			tasks[i] = &queue.Task{
				ID:          uuid.New(),
				Queue:       queue.DefaultQueueName,
				TaskType:    queue.TaskTypeOneTime,
				TaskName:    "queue_test.testPayload",
				Payload:     payloadBytes,
				Status:      queue.TaskStatusPending,
				Priority:    queue.PriorityMedium,
				MaxRetries:  3,
				ScheduledAt: time.Now().Add(-time.Minute),
				CreatedAt:   time.Now(),
			}
			mockRepo.On("ClaimTask", mock.Anything, mock.Anything, []string{queue.DefaultQueueName}, mock.Anything).
				Return(tasks[i], nil).Once()
		}

		mockRepo.On("ClaimTask", mock.Anything, mock.Anything, []string{queue.DefaultQueueName}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()

		for _, task := range tasks {
			mockRepo.On("CompleteTask", mock.Anything, task.ID).Return(nil).Maybe()
		}

		// Handler blocks until released
		block := make(chan struct{})
		handler := queue.NewTaskHandler(func(ctx context.Context, payload testPayload) error {
			<-block // Block here
			return nil
		})
		require.NoError(t, worker.RegisterHandler(handler))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_ = worker.Start(ctx)
		}()

		// Wait for both tasks to be active (all slots busy)
		deadline := time.Now().Add(2 * time.Second)
		for worker.Stats().ActiveTasks < 2 && time.Now().Before(deadline) {
			time.Sleep(1 * time.Millisecond)
		}

		// Check health - should fail because overloaded
		err = worker.Healthcheck(context.Background())
		assert.Error(t, err)
		assert.ErrorIs(t, err, queue.ErrHealthcheckFailed)
		assert.ErrorIs(t, err, queue.ErrWorkerOverloaded)
		assert.Contains(t, err.Error(), "slots busy")

		// Release tasks
		close(block)

		// Wait for tasks to complete
		deadline = time.Now().Add(2 * time.Second)
		for worker.Stats().ActiveTasks > 0 && time.Now().Before(deadline) {
			time.Sleep(1 * time.Millisecond)
		}

		// Now health should be OK
		err = worker.Healthcheck(context.Background())
		assert.NoError(t, err)

		_ = worker.Stop()
	})
}
