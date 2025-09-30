package queue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// SchedulerRepository defines the interface for scheduler operations
type SchedulerRepository interface {
	// CreateTask creates a new task in the storage
	CreateTask(ctx context.Context, task *Task) error

	// GetPendingTaskByName checks if a pending task with given name exists
	GetPendingTaskByName(ctx context.Context, taskName string) (*Task, error)
}

// Scheduler manages periodic task scheduling
type Scheduler struct {
	repo     SchedulerRepository
	tasks    map[string]*scheduledTask
	mu       sync.RWMutex
	ticker   *time.Ticker
	interval time.Duration
	logger   *slog.Logger

	// State management
	ctx             context.Context
	cancel          context.CancelFunc
	running         atomic.Bool
	wg              sync.WaitGroup
	shutdownTimeout time.Duration

	// Observability metrics
	tasksScheduled atomic.Int64
	activeChecks   atomic.Int32
}

// SchedulerStats provides observability metrics for monitoring and debugging
type SchedulerStats struct {
	TasksScheduled int64 // Total number of tasks created by the scheduler
	ActiveChecks   int32 // Number of check operations currently running
	IsRunning      bool  // Whether the scheduler is currently running
}

// scheduledTask holds configuration for a periodic task
type scheduledTask struct {
	name            string
	schedule        Schedule
	queue           string
	priority        Priority
	maxRetries      int8
	lastScheduledAt *time.Time // Track when we last created a task
}

// NewScheduler creates a new task scheduler
func NewScheduler(repo SchedulerRepository, opts ...SchedulerOption) (*Scheduler, error) {
	if repo == nil {
		return nil, ErrRepositoryNil
	}

	// Default options
	options := &schedulerOptions{
		checkInterval:   30 * time.Second,
		shutdownTimeout: 30 * time.Second,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)), // No-op logger by default
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	return &Scheduler{
		repo:            repo,
		tasks:           make(map[string]*scheduledTask),
		interval:        options.checkInterval,
		shutdownTimeout: options.shutdownTimeout,
		logger:          options.logger,
	}, nil
}

// NewSchedulerFromConfig creates a Scheduler from configuration.
// Repository must be provided. Additional options can override config values.
func NewSchedulerFromConfig(cfg Config, repo SchedulerRepository, opts ...SchedulerOption) (*Scheduler, error) {
	// Combine config options with user-provided options (user options override)
	// Option functions handle zero/empty values appropriately
	allOpts := append([]SchedulerOption{
		WithCheckInterval(cfg.CheckInterval),
		WithSchedulerShutdownTimeout(cfg.ShutdownTimeout),
	}, opts...)

	return NewScheduler(repo, allOpts...)
}

// AddTask registers a periodic task with the scheduler.
func (s *Scheduler) AddTask(name string, schedule Schedule, opts ...SchedulerTaskOption) error {
	taskOpts := &schedulerTaskOptions{
		queue:      DefaultQueueName,
		priority:   PriorityDefault,
		maxRetries: 3,
	}

	for _, opt := range opts {
		opt(taskOpts)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[name]; exists {
		return ErrTaskAlreadyRegistered
	}

	task := &scheduledTask{
		name:       name,
		schedule:   schedule,
		queue:      taskOpts.queue,
		priority:   taskOpts.priority,
		maxRetries: taskOpts.maxRetries,
	}

	s.tasks[name] = task

	s.logger.InfoContext(context.Background(), "registered periodic task",
		slog.String("task_name", name),
		slog.String("schedule", schedule.String()))

	return nil
}

// Start begins the scheduler's periodic task checking. This is a blocking operation
// that runs until the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already started")
	}

	taskCount := len(s.tasks)
	if taskCount == 0 {
		s.mu.Unlock()
		return ErrSchedulerNotConfigured
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.ticker = time.NewTicker(s.interval)
	s.mu.Unlock()

	// Reset running flag
	s.running.Store(true)

	defer s.ticker.Stop()

	s.logger.InfoContext(s.ctx, "scheduler started",
		slog.Int("task_count", taskCount),
		slog.Duration("check_interval", s.interval))

	s.checkTasksWithWait()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.InfoContext(context.Background(), "scheduler stopping")
			s.running.Store(false)
			return s.ctx.Err()
		case <-s.ticker.C:
			s.checkTasksWithWait()
		}
	}
}

// Stop gracefully shuts down the scheduler with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	if s.cancel == nil {
		s.mu.Unlock()
		return fmt.Errorf("scheduler not started")
	}

	s.running.Store(false)

	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	cancel()

	s.logger.InfoContext(context.Background(), "scheduler stopping, waiting for active checks to complete",
		slog.Duration("timeout", s.shutdownTimeout))

	ctx, ctxCancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer ctxCancel()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.InfoContext(context.Background(), "scheduler stopped cleanly")
		return nil
	case <-ctx.Done():
		s.logger.WarnContext(context.Background(), "scheduler shutdown timeout exceeded - some checks may be abandoned",
			slog.Duration("timeout", s.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", s.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the scheduler, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (s *Scheduler) Run(ctx context.Context) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Start(ctx)
		}()

		select {
		case <-ctx.Done():
			// Context cancelled - perform graceful shutdown
			_ = s.Stop() // Ignore stop error in normal shutdown
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

// checkTasksWithWait is a wrapper around checkTasks that tracks the operation with WaitGroup
func (s *Scheduler) checkTasksWithWait() {
	// Mutex protects against shutdown race: Must verify scheduler is still running
	// AND add to waitgroup atomically, otherwise Stop() might wait on incomplete count
	s.mu.RLock()
	if s.cancel == nil {
		s.mu.RUnlock()
		return
	}
	s.wg.Add(1)
	s.mu.RUnlock()

	defer s.wg.Done()

	// Track active checks for metrics
	s.activeChecks.Add(1)
	defer s.activeChecks.Add(-1)

	// Use context.Background() to avoid issues during shutdown when s.ctx is cancelled
	s.checkTasks(context.Background())
}

// checkTasks checks all registered tasks and creates any that are due.
func (s *Scheduler) checkTasks(ctx context.Context) {
	s.mu.RLock()
	i := 0
	tasks := make([]*scheduledTask, len(s.tasks))
	for _, task := range s.tasks {
		tasks[i] = task
		i++
	}
	s.mu.RUnlock()

	now := time.Now()

	for _, task := range tasks {
		nextRun := s.calculateNextRun(task, now)
		if err := s.scheduleTaskIfNeeded(ctx, task, now); err != nil {
			s.logger.ErrorContext(ctx, "failed to schedule task",
				slog.String("task_name", task.name),
				slog.Time("next_run", nextRun),
				slog.String("schedule", task.schedule.String()),
				slog.String("error", err.Error()))
		}
	}
}

// scheduleTaskIfNeeded checks if a task should be scheduled and creates it if needed.
func (s *Scheduler) scheduleTaskIfNeeded(ctx context.Context, task *scheduledTask, now time.Time) error {
	nextRun := s.calculateNextRun(task, now)

	// Scheduling decision: Respect schedule timing - don't create tasks before they're due
	// This prevents scheduler check frequency from affecting actual schedule accuracy
	if !s.shouldScheduleTask(task, nextRun, now) {
		return nil
	}

	// Idempotency check: Prevent duplicate tasks for same schedule period
	// Critical for reliability - ensures scheduler restarts don't create duplicates
	// Also protects against race conditions when multiple scheduler instances run
	existing, err := s.repo.GetPendingTaskByName(ctx, task.name)
	if err == nil && existing != nil {
		s.updateTaskState(task.name, &existing.ScheduledAt)
		s.logger.DebugContext(ctx, "periodic task already pending",
			slog.String("task_name", task.name),
			slog.Time("scheduled_for", existing.ScheduledAt))
		return nil
	}

	if err := s.createTask(ctx, task, nextRun); err != nil {
		return fmt.Errorf("failed to create periodic task: %w", err)
	}

	s.updateTaskState(task.name, &nextRun)

	if task.lastScheduledAt == nil {
		s.logger.InfoContext(ctx, "created periodic task (first run)",
			slog.String("task_name", task.name),
			slog.Time("scheduled_for", nextRun))
	} else {
		s.logger.InfoContext(ctx, "created periodic task",
			slog.String("task_name", task.name),
			slog.Time("scheduled_for", nextRun))
	}

	return nil
}

// calculateNextRun determines when the task should run next.
func (s *Scheduler) calculateNextRun(task *scheduledTask, now time.Time) time.Time {
	if task.lastScheduledAt == nil {
		return task.schedule.Next(now)
	}
	return task.schedule.Next(*task.lastScheduledAt)
}

// shouldScheduleTask determines if a task is due to be scheduled.
func (s *Scheduler) shouldScheduleTask(task *scheduledTask, nextRun, now time.Time) bool {
	if task.lastScheduledAt == nil {
		return true
	}

	if nextRun.After(now) {
		s.logger.DebugContext(context.Background(), "periodic task not due yet",
			slog.String("task_name", task.name),
			slog.Time("next_run", nextRun))
		return false
	}

	return true
}

// updateTaskState updates the lastScheduledAt time for a task.
func (s *Scheduler) updateTaskState(taskName string, scheduledAt *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.tasks[taskName]; ok {
		t.lastScheduledAt = scheduledAt
	}
}

// createTask creates a new task instance in the repository.
func (s *Scheduler) createTask(ctx context.Context, task *scheduledTask, scheduledAt time.Time) error {
	newTask := &Task{
		ID:          uuid.New(),
		Queue:       task.queue,
		TaskType:    TaskTypePeriodic,
		TaskName:    task.name,
		Payload:     nil,
		Status:      TaskStatusPending,
		Priority:    task.priority,
		RetryCount:  0,
		MaxRetries:  task.maxRetries,
		ScheduledAt: scheduledAt,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.CreateTask(ctx, newTask); err != nil {
		return err
	}

	s.tasksScheduled.Add(1)

	return nil
}

// RemoveTask removes a periodic task from the scheduler.
func (s *Scheduler) RemoveTask(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tasks, name)

	s.logger.InfoContext(context.Background(), "removed periodic task",
		slog.String("task_name", name))
}

// ListTasks returns all registered periodic task names.
func (s *Scheduler) ListTasks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.tasks))
	for name := range s.tasks {
		names = append(names, name)
	}
	return names
}

// Stats returns current scheduler statistics for observability and monitoring.
// This method is thread-safe and can be called at any time.
//
// Use cases:
//   - Prometheus/Grafana metrics
//   - Health checks
//   - Debugging production issues
//   - Testing (verify task scheduling without sleep)
func (s *Scheduler) Stats() SchedulerStats {
	s.mu.RLock()
	isRunning := s.cancel != nil
	s.mu.RUnlock()

	return SchedulerStats{
		TasksScheduled: s.tasksScheduled.Load(),
		ActiveChecks:   s.activeChecks.Load(),
		IsRunning:      isRunning,
	}
}

// Healthcheck validates that the scheduler is operational.
// Returns nil if healthy, or an error describing the health issue.
// This method is thread-safe and suitable for use in health check endpoints.
//
// Health criteria:
//   - Scheduler must be running
//   - Must have at least one registered task
//
// Use with health check frameworks:
//
//	healthSrv.AddCheck("queue-scheduler", scheduler.Healthcheck)
//
// The returned error can be checked using errors.Is:
//
//	if errors.Is(err, queue.ErrSchedulerNotRunning) { ... }
//	if errors.Is(err, queue.ErrNoTasksRegistered) { ... }
func (s *Scheduler) Healthcheck(ctx context.Context) error {
	stats := s.Stats()

	if !stats.IsRunning {
		return errors.Join(ErrHealthcheckFailed, ErrSchedulerNotRunning)
	}

	s.mu.RLock()
	taskCount := len(s.tasks)
	s.mu.RUnlock()

	if taskCount == 0 {
		return errors.Join(ErrHealthcheckFailed, ErrNoTasksRegistered)
	}

	return nil
}
