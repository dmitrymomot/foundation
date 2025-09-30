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

func TestWorker_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	t.Run("returns error when shutdown timeout exceeded", func(t *testing.T) {
		t.Parallel()

		mockRepo := new(MockWorkerRepository)
		defer mockRepo.AssertExpectations(t)

		// Create a long-running task
		payload := testPayload{Message: "long-running", Value: 1}
		payloadBytes, _ := json.Marshal(payload)
		task := &queue.Task{
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
			Return(task, nil).Once()
		// Using .Maybe() for catch-all polls after task is claimed
		mockRepo.On("ClaimTask", mock.Anything, mock.Anything, []string{queue.DefaultQueueName}, mock.Anything).
			Return(nil, queue.ErrNoTaskToClaim).Maybe()
		// CompleteTask may or may not be called depending on timing (task completes after timeout)
		// Using .Maybe() because shutdown timeout may occur before task completes
		mockRepo.On("CompleteTask", mock.Anything, task.ID).Return(nil).Maybe()

		// Use short timeout
		worker, err := queue.NewWorker(mockRepo,
			queue.WithPullInterval(10*time.Millisecond),
			queue.WithShutdownTimeout(50*time.Millisecond),
		)
		require.NoError(t, err)

		taskStarted := make(chan struct{})
		handler := queue.NewTaskHandler(func(ctx context.Context, payload testPayload) error {
			close(taskStarted)
			time.Sleep(200 * time.Millisecond) // Longer than shutdown timeout
			return nil
		})
		err = worker.RegisterHandler(handler)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			if err := worker.Start(ctx); err != nil {
				t.Logf("worker error: %v", err)
			}
		}()

		// Wait for task to start
		<-taskStarted

		// Stop should timeout
		err = worker.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown timeout")
	})
}
