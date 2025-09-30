package queue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// WorkerRepository defines the interface for worker operations
type WorkerRepository interface {
	// ClaimTask atomically claims the next available task
	ClaimTask(ctx context.Context, workerID uuid.UUID, queues []string, lockDuration time.Duration) (*Task, error)

	// CompleteTask marks task as completed
	CompleteTask(ctx context.Context, taskID uuid.UUID) error

	// FailTask marks task as failed and increments retry count
	FailTask(ctx context.Context, taskID uuid.UUID, errorMsg string) error

	// MoveToDLQ moves task to dead letter queue
	MoveToDLQ(ctx context.Context, taskID uuid.UUID) error

	// ExtendLock extends the lock timeout for long-running tasks (optional)
	ExtendLock(ctx context.Context, taskID uuid.UUID, duration time.Duration) error
}

// Worker processes tasks from the queue
type Worker struct {
	repo     WorkerRepository
	handlers map[string]Handler
	queues   []string
	workerID uuid.UUID
	sem      chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex

	// Configuration
	pullInterval    time.Duration
	lockTimeout     time.Duration
	shutdownTimeout time.Duration
	logger          *slog.Logger

	// State management
	ctx      context.Context
	cancel   context.CancelFunc
	stopping atomic.Bool

	// Observability metrics
	tasksProcessed atomic.Int64
	tasksFailed    atomic.Int64
	activeTasks    atomic.Int32
}

// WorkerStats provides observability metrics for monitoring and debugging
type WorkerStats struct {
	TasksProcessed int64 // Total number of successfully completed tasks
	TasksFailed    int64 // Total number of failed tasks (including those moved to DLQ)
	ActiveTasks    int32 // Number of tasks currently being processed
	IsRunning      bool  // Whether the worker is currently running
}

// NewWorker creates a new task worker
func NewWorker(repo WorkerRepository, opts ...WorkerOption) (*Worker, error) {
	if repo == nil {
		return nil, ErrRepositoryNil
	}

	// Default options
	options := &workerOptions{
		queues:             []string{DefaultQueueName},
		pullInterval:       5 * time.Second,
		lockTimeout:        5 * time.Minute,
		shutdownTimeout:    30 * time.Second,
		maxConcurrentTasks: 1,
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)), // No-op logger by default
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	return &Worker{
		repo:            repo,
		handlers:        make(map[string]Handler),
		queues:          options.queues,
		workerID:        uuid.New(),
		sem:             make(chan struct{}, options.maxConcurrentTasks),
		pullInterval:    options.pullInterval,
		lockTimeout:     options.lockTimeout,
		shutdownTimeout: options.shutdownTimeout,
		logger:          options.logger,
	}, nil
}

// NewWorkerFromConfig creates a Worker from configuration.
// Repository must be provided. Additional options can override config values.
func NewWorkerFromConfig(cfg Config, repo WorkerRepository, opts ...WorkerOption) (*Worker, error) {
	// Combine config options with user-provided options (user options override)
	// Option functions handle zero/empty values appropriately
	allOpts := append([]WorkerOption{
		WithPullInterval(cfg.PollInterval),
		WithLockTimeout(cfg.LockTimeout),
		WithShutdownTimeout(cfg.ShutdownTimeout),
		WithMaxConcurrentTasks(cfg.MaxConcurrentTasks),
		WithQueues(cfg.Queues...),
	}, opts...)

	return NewWorker(repo, allOpts...)
}

// RegisterHandler registers a single task handler.
func (w *Worker) RegisterHandler(handler Handler) error {
	if handler == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.handlers[handler.Name()] = handler
	return nil
}

// RegisterHandlers registers multiple task handlers.
func (w *Worker) RegisterHandlers(handlers ...Handler) error {
	for _, h := range handlers {
		if err := w.RegisterHandler(h); err != nil {
			return err
		}
	}
	return nil
}

// Start begins processing tasks. This is a blocking operation that runs until
// the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.cancel != nil {
		w.mu.Unlock()
		return fmt.Errorf("worker already started")
	}

	if len(w.handlers) == 0 {
		w.mu.Unlock()
		return ErrNoHandlers
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.mu.Unlock()

	// Reset stopping flag
	w.stopping.Store(false)

	w.logger.InfoContext(w.ctx, "worker started",
		slog.String("worker_id", w.workerID.String()),
		slog.Any("queues", w.queues),
		slog.Int("max_concurrent", cap(w.sem)))

	ticker := time.NewTicker(w.pullInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.InfoContext(context.Background(), "worker stopping")
			return w.ctx.Err()
		case <-ticker.C:
			select {
			case w.sem <- struct{}{}:
				// Mutex protects against shutdown race: Must verify worker is still running
				// AND add to waitgroup atomically, otherwise Stop() might wait on incomplete count
				w.mu.RLock()
				if w.cancel == nil {
					w.mu.RUnlock()
					<-w.sem
					return nil
				}
				w.wg.Add(1)
				w.mu.RUnlock()

				go func() {
					defer w.wg.Done()
					defer func() { <-w.sem }()

					if err := w.pullAndProcess(); err != nil {
						if err != ErrHandlerNotFound {
							w.logger.ErrorContext(w.ctx, "failed to process task",
								slog.String("worker_id", w.workerID.String()),
								slog.String("error", err.Error()))
						}
					}
				}()
			default:
				w.logger.DebugContext(w.ctx, "all worker slots busy, skipping tick",
					slog.String("worker_id", w.workerID.String()))
			}
		}
	}
}

// Stop gracefully shuts down the worker with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (w *Worker) Stop() error {
	w.mu.Lock()
	if w.cancel == nil {
		w.mu.Unlock()
		return fmt.Errorf("worker not started")
	}

	w.stopping.Store(true)
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()

	cancel()

	w.logger.InfoContext(context.Background(), "worker stopping, waiting for active tasks to complete",
		slog.String("worker_id", w.workerID.String()),
		slog.Duration("timeout", w.shutdownTimeout))

	ctx, ctxCancel := context.WithTimeout(context.Background(), w.shutdownTimeout)
	defer ctxCancel()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.InfoContext(context.Background(), "worker stopped cleanly",
			slog.String("worker_id", w.workerID.String()))
		return nil
	case <-ctx.Done():
		w.logger.WarnContext(context.Background(), "worker shutdown timeout exceeded - some tasks may be abandoned",
			slog.String("worker_id", w.workerID.String()),
			slog.Duration("timeout", w.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", w.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the worker, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (w *Worker) Run(ctx context.Context) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- w.Start(ctx)
		}()

		select {
		case <-ctx.Done():
			// Context cancelled - perform graceful shutdown
			_ = w.Stop() // Ignore stop error in normal shutdown
			<-errCh      // Wait for Start() to exit
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

// pullAndProcess pulls a task and processes it.
func (w *Worker) pullAndProcess() error {
	task, err := w.repo.ClaimTask(w.ctx, w.workerID, w.queues, w.lockTimeout)
	if err != nil {
		if errors.Is(err, ErrNoTaskToClaim) {
			return nil
		}
		return fmt.Errorf("failed to claim task: %w", err)
	}

	if task == nil {
		return nil
	}

	w.logger.DebugContext(w.ctx, "claimed task",
		slog.String("worker_id", w.workerID.String()),
		slog.String("task_id", task.ID.String()),
		slog.String("task_name", task.TaskName),
		slog.String("queue", task.Queue))

	return w.processTask(task)
}

// processTask executes a task with its handler.
func (w *Worker) processTask(task *Task) (retErr error) {
	start := time.Now()

	w.activeTasks.Add(1)
	defer w.activeTasks.Add(-1)

	// Panic recovery ensures system stability
	// Strategy: Treat panics as task failures with retry eligibility
	// This prevents a single bad handler from crashing the entire worker
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic in handler: %v", r)
			w.logger.ErrorContext(w.ctx, "handler panicked",
				slog.String("worker_id", w.workerID.String()),
				slog.String("task_id", task.ID.String()),
				slog.String("task_name", task.TaskName),
				slog.Any("panic", r))
			duration := time.Since(start)
			_ = w.handleTaskFailure(task, retErr, duration)
		}
	}()

	w.mu.RLock()
	handler, ok := w.handlers[task.TaskName]
	w.mu.RUnlock()

	if !ok {
		return w.handleMissingHandler(task)
	}

	// Isolation strategy: Create independent context for task execution
	// Rationale: Worker shutdown should not interrupt running tasks
	// Tasks get full lockTimeout to complete even during graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), w.lockTimeout)
	defer cancel()

	err := handler.Handle(ctx, task.Payload)
	duration := time.Since(start)

	if err != nil {
		return w.handleTaskFailure(task, err, duration)
	}

	return w.handleTaskSuccess(task, duration)
}

// handleMissingHandler processes tasks that have no registered handler
// Immediately moves tasks to DLQ since retries won't help without a handler
//
// Why direct to DLQ: Tasks without handlers will fail on every retry attempt,
// wasting resources. Moving them directly to DLQ allows operators to:
// 1. Deploy the missing handler code
// 2. Manually requeue tasks from DLQ once handler is available
// 3. Investigate why tasks were enqueued without corresponding handlers
func (w *Worker) handleMissingHandler(task *Task) error {
	w.tasksFailed.Add(1)

	w.logger.ErrorContext(w.ctx, "no handler registered for task type",
		slog.String("worker_id", w.workerID.String()),
		slog.String("task_id", task.ID.String()),
		slog.String("task_name", task.TaskName))

	errorMsg := "no handler registered for task type: " + task.TaskName
	if err := w.repo.FailTask(w.ctx, task.ID, errorMsg); err != nil {
		return fmt.Errorf("failed to mark task %s as failed: %w", task.ID, err)
	}

	if err := w.repo.MoveToDLQ(w.ctx, task.ID); err != nil {
		return fmt.Errorf("failed to move task %s to DLQ: %w", task.ID, err)
	}

	return ErrHandlerNotFound
}

// handleTaskFailure processes failed task execution
//
// Retry decision logic:
// 1. Always calls FailTask first to record the error and increment retry count
// 2. Checks if task has exhausted all retries (RetryCount >= MaxRetries)
// 3. If retries remain: FailTask already reset task to "pending" with backoff
// 4. If no retries remain: Move to DLQ for manual inspection
//
// The separation of FailTask and MoveToDLQ allows the storage layer to:
// - Track failure history and error messages
// - Implement exponential backoff strategies
// - Maintain audit trails of task processing attempts
func (w *Worker) handleTaskFailure(task *Task, execErr error, duration time.Duration) error {
	w.tasksFailed.Add(1)

	w.logger.ErrorContext(w.ctx, "task failed",
		slog.String("worker_id", w.workerID.String()),
		slog.String("task_id", task.ID.String()),
		slog.String("task_name", task.TaskName),
		slog.Int("retry_count", int(task.RetryCount)),
		slog.Int("max_retries", int(task.MaxRetries)),
		slog.Duration("duration", duration),
		slog.String("error", execErr.Error()))

	if err := w.repo.FailTask(w.ctx, task.ID, execErr.Error()); err != nil {
		return fmt.Errorf("failed to update task %s status to failed: %w", task.ID, err)
	}

	if task.RetryCount >= task.MaxRetries {
		if err := w.repo.MoveToDLQ(w.ctx, task.ID); err != nil {
			return fmt.Errorf("failed to move task %s to DLQ after max retries: %w", task.ID, err)
		}

		w.logger.WarnContext(w.ctx, "task moved to dead letter queue",
			slog.String("worker_id", w.workerID.String()),
			slog.String("task_id", task.ID.String()),
			slog.String("task_name", task.TaskName))

		return nil
	}

	return nil
}

// handleTaskSuccess processes successful task completion.
func (w *Worker) handleTaskSuccess(task *Task, duration time.Duration) error {
	if err := w.repo.CompleteTask(w.ctx, task.ID); err != nil {
		return fmt.Errorf("failed to mark task %s as completed: %w", task.ID, err)
	}

	w.tasksProcessed.Add(1)

	w.logger.InfoContext(w.ctx, "task completed successfully",
		slog.String("worker_id", w.workerID.String()),
		slog.String("task_id", task.ID.String()),
		slog.String("task_name", task.TaskName),
		slog.String("queue", task.Queue),
		slog.Duration("duration", duration))

	return nil
}

// ExtendLockForTask extends the lock timeout for a long-running task.
// Call this periodically for tasks that take longer than lockTimeout.
func (w *Worker) ExtendLockForTask(ctx context.Context, taskID uuid.UUID, extension time.Duration) error {
	return w.repo.ExtendLock(ctx, taskID, extension)
}

// WorkerInfo returns identifying information about the worker instance.
func (w *Worker) WorkerInfo() (id string, hostname string, pid int) {
	hostname, _ = os.Hostname()
	return w.workerID.String(), hostname, os.Getpid()
}

// HandlerCount returns the number of registered handlers.
// This method is thread-safe and can be called at any time.
func (w *Worker) HandlerCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.handlers)
}

// HasHandlers returns true if the worker has registered handlers.
// This method is thread-safe and can be called at any time.
func (w *Worker) HasHandlers() bool {
	return w.HandlerCount() > 0
}

// Queues returns the list of queues this worker processes.
// If no queues are configured, returns the default queue.
// This method is thread-safe and can be called at any time.
func (w *Worker) Queues() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.queues) == 0 {
		return []string{DefaultQueueName}
	}

	// Return a copy to prevent external modification
	result := make([]string, len(w.queues))
	copy(result, w.queues)
	return result
}

// Stats returns current worker statistics for observability and monitoring.
// This method is thread-safe and can be called at any time.
//
// Use cases:
//   - Prometheus/Grafana metrics
//   - Health checks
//   - Debugging production issues
//   - Testing (verify task processing without sleep)
func (w *Worker) Stats() WorkerStats {
	w.mu.RLock()
	isRunning := w.cancel != nil
	w.mu.RUnlock()

	return WorkerStats{
		TasksProcessed: w.tasksProcessed.Load(),
		TasksFailed:    w.tasksFailed.Load(),
		ActiveTasks:    w.activeTasks.Load(),
		IsRunning:      isRunning,
	}
}

// Healthcheck validates that the worker is operational and not overloaded.
// Returns nil if healthy, or an error describing the health issue.
// This method is thread-safe and suitable for use in health check endpoints.
//
// Health criteria:
//   - Worker must be running
//   - Active tasks must not exceed capacity (semaphore slots)
//
// Use with health check frameworks:
//
//	healthSrv.AddCheck("queue-worker", worker.Healthcheck)
//
// The returned error can be checked using errors.Is:
//
//	if errors.Is(err, queue.ErrWorkerNotRunning) { ... }
//	if errors.Is(err, queue.ErrWorkerOverloaded) { ... }
func (w *Worker) Healthcheck(ctx context.Context) error {
	stats := w.Stats()

	if !stats.IsRunning {
		return errors.Join(ErrHealthcheckFailed, ErrWorkerNotRunning)
	}

	// Check if worker is overloaded (all semaphore slots busy)
	maxConcurrent := int32(cap(w.sem))
	if stats.ActiveTasks >= maxConcurrent {
		return errors.Join(ErrHealthcheckFailed, ErrWorkerOverloaded,
			fmt.Errorf("%d/%d slots busy", stats.ActiveTasks, maxConcurrent))
	}

	return nil
}
