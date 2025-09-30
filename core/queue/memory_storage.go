package queue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// MemoryStorageStats provides observability metrics for monitoring and debugging
type MemoryStorageStats struct {
	ActiveTasks       int   // Current number of tasks in storage
	ExpiredLocksFreed int64 // Total number of expired locks freed
	IsRunning         bool  // Whether the lock expiration manager is running
}

// MemoryStorage implements all queue repository interfaces for testing and local development
type MemoryStorage struct {
	mu    sync.RWMutex
	tasks map[uuid.UUID]*Task
	dlq   map[uuid.UUID]*TasksDlq

	// Indexes for efficient queries
	byQueue  map[string][]uuid.UUID
	byStatus map[TaskStatus][]uuid.UUID

	// Configuration
	lockCheckInterval time.Duration
	shutdownTimeout   time.Duration
	logger            *slog.Logger

	// State management
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool
	wg      sync.WaitGroup

	// Observability metrics
	expiredLocksFreed atomic.Int64
}

// MemoryStorageOption configures a MemoryStorage.
type MemoryStorageOption func(*MemoryStorage)

// WithLockCheckInterval sets the interval for checking expired locks.
func WithLockCheckInterval(interval time.Duration) MemoryStorageOption {
	return func(ms *MemoryStorage) {
		if interval > 0 {
			ms.lockCheckInterval = interval
		}
	}
}

// WithMemoryStorageShutdownTimeout sets the graceful shutdown timeout.
func WithMemoryStorageShutdownTimeout(timeout time.Duration) MemoryStorageOption {
	return func(ms *MemoryStorage) {
		if timeout > 0 {
			ms.shutdownTimeout = timeout
		}
	}
}

// WithMemoryStorageLogger sets the logger for internal operations.
func WithMemoryStorageLogger(logger *slog.Logger) MemoryStorageOption {
	return func(ms *MemoryStorage) {
		if logger != nil {
			ms.logger = logger
		}
	}
}

// NewMemoryStorage creates a new in-memory storage implementation.
// Call Start() to begin the lock expiration manager.
func NewMemoryStorage(opts ...MemoryStorageOption) *MemoryStorage {
	ms := &MemoryStorage{
		tasks:             make(map[uuid.UUID]*Task),
		dlq:               make(map[uuid.UUID]*TasksDlq),
		byQueue:           make(map[string][]uuid.UUID),
		byStatus:          make(map[TaskStatus][]uuid.UUID),
		lockCheckInterval: time.Second,
		shutdownTimeout:   30 * time.Second,
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(ms)
	}

	return ms
}

// Close stops the background goroutines. Deprecated: Use Stop() instead.
func (ms *MemoryStorage) Close() error {
	return ms.Stop()
}

// CreateTask implements EnqueuerRepository and SchedulerRepository
func (ms *MemoryStorage) CreateTask(ctx context.Context, task *Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Check if task already exists
	if _, exists := ms.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	// Clone task to prevent external modifications
	taskCopy := *task
	ms.tasks[task.ID] = &taskCopy

	// Update indexes
	ms.byQueue[task.Queue] = append(ms.byQueue[task.Queue], task.ID)
	ms.byStatus[task.Status] = append(ms.byStatus[task.Status], task.ID)

	return nil
}

// ClaimTask implements WorkerRepository
func (ms *MemoryStorage) ClaimTask(ctx context.Context, workerID uuid.UUID, queues []string, lockDuration time.Duration) (*Task, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	var bestTask *Task
	var bestPriority Priority = -1

	// Task selection algorithm: Priority-first with time-based tiebreaking
	// Guarantees: Higher priority tasks always run before lower priority ones
	// Fairness: Within same priority, earliest scheduled tasks run first
	// This prevents starvation while ensuring critical tasks get precedence
	for _, taskID := range ms.byStatus[TaskStatusPending] {
		task := ms.tasks[taskID]

		// Queue filtering: Only process tasks from worker's registered queues
		if !slices.Contains(queues, task.Queue) {
			continue
		}

		// Scheduling constraint: Respect delayed execution times
		if task.ScheduledAt.After(now) {
			continue
		}

		// Lock safety: Skip tasks with unexpired locks (defensive programming)
		if task.LockedUntil != nil && task.LockedUntil.After(now) {
			continue
		}

		// Selection criteria: Priority first, then chronological order
		// This implements a stable priority queue with FIFO tiebreaking
		if bestTask == nil ||
			task.Priority > bestPriority ||
			(task.Priority == bestPriority && task.ScheduledAt.Before(bestTask.ScheduledAt)) {
			bestTask = task
			bestPriority = task.Priority
		}
	}

	if bestTask == nil {
		return nil, ErrNoTaskToClaim
	}

	// Claim the task
	lockUntil := now.Add(lockDuration)
	bestTask.Status = TaskStatusProcessing
	bestTask.LockedUntil = &lockUntil
	bestTask.LockedBy = &workerID

	// Update status index
	ms.removeFromStatusIndex(bestTask.ID, TaskStatusPending)
	ms.byStatus[TaskStatusProcessing] = append(ms.byStatus[TaskStatusProcessing], bestTask.ID)

	// Return a copy to prevent external modifications
	taskCopy := *bestTask
	return &taskCopy, nil
}

// CompleteTask implements WorkerRepository
func (ms *MemoryStorage) CompleteTask(ctx context.Context, taskID uuid.UUID) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	task, exists := ms.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusProcessing {
		return fmt.Errorf("task %s is not in processing state", taskID)
	}

	now := time.Now()
	task.Status = TaskStatusCompleted
	task.ProcessedAt = &now
	task.LockedUntil = nil
	task.LockedBy = nil

	// Update status index
	ms.removeFromStatusIndex(taskID, TaskStatusProcessing)
	ms.byStatus[TaskStatusCompleted] = append(ms.byStatus[TaskStatusCompleted], taskID)

	return nil
}

// FailTask implements WorkerRepository
func (ms *MemoryStorage) FailTask(ctx context.Context, taskID uuid.UUID, errorMsg string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	task, exists := ms.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusProcessing {
		return fmt.Errorf("task %s is not in processing state", taskID)
	}

	task.RetryCount++
	task.Error = &errorMsg
	task.LockedUntil = nil
	task.LockedBy = nil

	if task.RetryCount >= task.MaxRetries {
		task.Status = TaskStatusFailed
		ms.removeFromStatusIndex(taskID, TaskStatusProcessing)
		ms.byStatus[TaskStatusFailed] = append(ms.byStatus[TaskStatusFailed], taskID)
	} else {
		// Reset to pending for retry
		task.Status = TaskStatusPending
		ms.removeFromStatusIndex(taskID, TaskStatusProcessing)
		ms.byStatus[TaskStatusPending] = append(ms.byStatus[TaskStatusPending], taskID)

		// Retry backoff strategy: Linear progression prevents system overload
		// Formula: retryCount * 30s (30s, 60s, 90s, 120s...)
		// Rationale: Faster than exponential for transient issues,
		// but still protects against persistent failures causing thundering herd
		backoff := time.Duration(task.RetryCount) * 30 * time.Second
		task.ScheduledAt = time.Now().Add(backoff)
	}

	return nil
}

// MoveToDLQ implements WorkerRepository
func (ms *MemoryStorage) MoveToDLQ(ctx context.Context, taskID uuid.UUID) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	task, exists := ms.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Create DLQ entry
	dlqEntry := &TasksDlq{
		ID:         uuid.New(),
		TaskID:     task.ID,
		Queue:      task.Queue,
		TaskType:   task.TaskType,
		TaskName:   task.TaskName,
		Payload:    task.Payload,
		Priority:   task.Priority,
		Error:      "",
		RetryCount: task.RetryCount,
		FailedAt:   time.Now(),
		CreatedAt:  time.Now(),
	}

	if task.Error != nil {
		dlqEntry.Error = *task.Error
	}

	ms.dlq[dlqEntry.ID] = dlqEntry

	// Remove from main storage and indexes
	ms.removeFromStatusIndex(taskID, task.Status)
	ms.removeFromQueueIndex(taskID, task.Queue)
	delete(ms.tasks, taskID)

	return nil
}

// ExtendLock implements WorkerRepository
func (ms *MemoryStorage) ExtendLock(ctx context.Context, taskID uuid.UUID, duration time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	task, exists := ms.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusProcessing {
		return fmt.Errorf("task %s is not in processing state", taskID)
	}

	lockUntil := time.Now().Add(duration)
	task.LockedUntil = &lockUntil

	return nil
}

// GetPendingTaskByName implements SchedulerRepository
func (ms *MemoryStorage) GetPendingTaskByName(ctx context.Context, taskName string) (*Task, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Search through pending tasks for matching name
	for _, taskID := range ms.byStatus[TaskStatusPending] {
		task := ms.tasks[taskID]
		if task.TaskName == taskName {
			// Return a copy to prevent external modifications
			taskCopy := *task
			return &taskCopy, nil
		}
	}

	// No matching task found
	return nil, nil
}

// Helper methods

func (ms *MemoryStorage) removeFromStatusIndex(taskID uuid.UUID, status TaskStatus) {
	ms.byStatus[status] = slices.DeleteFunc(ms.byStatus[status], func(id uuid.UUID) bool {
		return id == taskID
	})
}

func (ms *MemoryStorage) removeFromQueueIndex(taskID uuid.UUID, queue string) {
	ms.byQueue[queue] = slices.DeleteFunc(ms.byQueue[queue], func(id uuid.UUID) bool {
		return id == taskID
	})
}

// Start begins the lock expiration manager. This is a blocking operation
// that runs until the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (ms *MemoryStorage) Start(ctx context.Context) error {
	ms.mu.Lock()
	if ms.cancel != nil {
		ms.mu.Unlock()
		return fmt.Errorf("memory storage already started")
	}

	ms.ctx, ms.cancel = context.WithCancel(ctx)
	ms.mu.Unlock()

	ms.running.Store(true)
	defer ms.running.Store(false)

	ms.logger.InfoContext(ms.ctx, "memory storage lock expiration manager started",
		slog.Duration("check_interval", ms.lockCheckInterval))

	ticker := time.NewTicker(ms.lockCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ms.ctx.Done():
			ms.logger.InfoContext(context.Background(), "memory storage stopping")
			return ms.ctx.Err()
		case <-ticker.C:
			ms.expireLocksWithWait()
		}
	}
}

// Stop gracefully shuts down the lock expiration manager with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (ms *MemoryStorage) Stop() error {
	ms.mu.Lock()
	if ms.cancel == nil {
		ms.mu.Unlock()
		return fmt.Errorf("memory storage not started")
	}

	cancel := ms.cancel
	ms.cancel = nil
	ms.mu.Unlock()

	// Cancel context to stop main loop
	cancel()

	// Wait for any in-progress lock expiration to complete with timeout
	ms.logger.InfoContext(context.Background(), "memory storage stopping, waiting for lock expiration to complete",
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
		ms.logger.InfoContext(context.Background(), "memory storage stopped cleanly")
		return nil
	case <-ctx.Done():
		ms.logger.WarnContext(context.Background(), "memory storage shutdown timeout exceeded",
			slog.Duration("timeout", ms.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", ms.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the lock expiration manager, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (ms *MemoryStorage) Run(ctx context.Context) func() error {
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

// expireLocksWithWait is a wrapper around expireLocks that tracks the operation with WaitGroup
func (ms *MemoryStorage) expireLocksWithWait() {
	ms.mu.RLock()
	if ms.cancel == nil {
		ms.mu.RUnlock()
		return
	}
	ms.wg.Add(1)
	ms.mu.RUnlock()

	defer ms.wg.Done()
	ms.expireLocks()
}

// expireLocks scans all processing tasks and releases expired locks.
// This allows tasks to be retried if a worker crashes or becomes unresponsive.
func (ms *MemoryStorage) expireLocks() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	freed := 0
	for _, taskID := range ms.byStatus[TaskStatusProcessing] {
		task := ms.tasks[taskID]
		if task.LockedUntil != nil && task.LockedUntil.Before(now) {
			// Release expired lock and reset task to pending for retry
			task.Status = TaskStatusPending
			task.LockedUntil = nil
			task.LockedBy = nil

			// Update indexes to make task claimable again
			ms.removeFromStatusIndex(taskID, TaskStatusProcessing)
			ms.byStatus[TaskStatusPending] = append(ms.byStatus[TaskStatusPending], taskID)
			freed++
		}
	}

	if freed > 0 {
		ms.expiredLocksFreed.Add(int64(freed))
	}
}

// Stats returns current memory storage statistics for observability and monitoring.
// This method is thread-safe and can be called at any time.
func (ms *MemoryStorage) Stats() MemoryStorageStats {
	ms.mu.RLock()
	isRunning := ms.cancel != nil
	activeTasks := len(ms.tasks)
	ms.mu.RUnlock()

	return MemoryStorageStats{
		ActiveTasks:       activeTasks,
		ExpiredLocksFreed: ms.expiredLocksFreed.Load(),
		IsRunning:         isRunning,
	}
}

// Healthcheck validates that the memory storage is operational.
// Returns nil if healthy, or an error describing the health issue.
// This method is thread-safe and suitable for use in health check endpoints.
func (ms *MemoryStorage) Healthcheck(ctx context.Context) error {
	stats := ms.Stats()

	// Memory storage should always be running for proper task recovery
	if !stats.IsRunning {
		return fmt.Errorf("lock expiration manager is not running")
	}

	return nil
}
