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
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool
	wg      sync.WaitGroup
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
		checkInterval: 30 * time.Second,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)), // No-op logger by default
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	return &Scheduler{
		repo:     repo,
		tasks:    make(map[string]*scheduledTask),
		interval: options.checkInterval,
		logger:   options.logger,
	}, nil
}

// NewSchedulerFromConfig creates a Scheduler from configuration.
// Repository must be provided. Additional options can override config values.
func NewSchedulerFromConfig(cfg Config, repo SchedulerRepository, opts ...SchedulerOption) (*Scheduler, error) {
	// Combine config options with user-provided options (user options override)
	// Option functions handle zero/empty values appropriately
	allOpts := append([]SchedulerOption{
		WithCheckInterval(cfg.CheckInterval),
	}, opts...)

	return NewScheduler(repo, allOpts...)
}

// AddTask registers a periodic task
func (s *Scheduler) AddTask(name string, schedule Schedule, opts ...SchedulerTaskOption) error {
	// Default task options
	taskOpts := &schedulerTaskOptions{
		queue:      DefaultQueueName,
		priority:   PriorityDefault,
		maxRetries: 3,
	}

	// Apply options
	for _, opt := range opts {
		opt(taskOpts)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if task already registered
	if _, exists := s.tasks[name]; exists {
		return ErrTaskAlreadyRegistered
	}

	// Register the task
	task := &scheduledTask{
		name:       name,
		schedule:   schedule,
		queue:      taskOpts.queue,
		priority:   taskOpts.priority,
		maxRetries: taskOpts.maxRetries,
	}

	s.tasks[name] = task

	// Log registration
	// Use context.Background() since this is during registration
	s.logger.InfoContext(context.Background(), "registered periodic task",
		slog.String("task_name", name),
		slog.String("schedule", schedule.String()))

	return nil
}

// Start begins the scheduler's periodic task checking
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

	s.logger.InfoContext(s.ctx, "scheduler started",
		slog.Int("task_count", taskCount),
		slog.Duration("check_interval", s.interval))

	// Check immediately on start
	s.checkTasksWithWait()

	// Then check periodically
	for {
		select {
		case <-s.ctx.Done():
			s.logger.InfoContext(context.Background(), "scheduler stopping")
			s.running.Store(false)
			return s.ctx.Err()
		case <-s.ticker.C:
			if s.running.Load() {
				s.checkTasksWithWait()
			}
		}
	}
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	if s.cancel == nil {
		s.mu.Unlock()
		return fmt.Errorf("scheduler not started")
	}

	// Stop accepting new checks
	s.running.Store(false)

	// Stop the ticker
	if s.ticker != nil {
		s.ticker.Stop()
	}

	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	// Cancel context to stop the main loop
	cancel()

	// Wait for any in-progress checkTasks to complete
	s.logger.InfoContext(context.Background(), "scheduler stopping, waiting for active checks to complete")
	s.wg.Wait()

	s.logger.InfoContext(context.Background(), "scheduler stopped")
	return nil
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that:
// 1. Starts scheduler in background goroutine
// 2. Handles context cancellation gracefully
// 3. Distinguishes between normal shutdown and actual errors
//
// This pattern allows multiple components to shutdown together:
//
//	g.Go(worker.Run(ctx))
//	g.Go(scheduler.Run(ctx))
//	g.Wait() // All components stop when context cancels
func (s *Scheduler) Run(ctx context.Context) func() error {
	return func() error {
		// Channel coordination: Start() runs independently while we monitor context
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Start(ctx)
		}()

		// Race between context cancellation and Start() errors
		select {
		case <-ctx.Done():
			// Graceful shutdown sequence: Stop first, then wait for Start to exit
			_ = s.Stop() // Ignore stop error in normal shutdown
			<-errCh      // Ensure Start() completes before returning
			return nil   // Normal shutdown is not an error
		case err := <-errCh:
			// Start returned: differentiate normal vs error conditions
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil // Context-driven shutdown is expected
			}
			return err // Actual error (misconfiguration, etc.)
		}
	}
}

// checkTasksWithWait is a wrapper around checkTasks that tracks the operation with WaitGroup
func (s *Scheduler) checkTasksWithWait() {
	s.wg.Add(1)
	defer s.wg.Done()
	s.checkTasks(s.ctx)
}

// checkTasks checks all registered tasks and creates any that are due
func (s *Scheduler) checkTasks(ctx context.Context) {
	// Get a snapshot of tasks
	s.mu.RLock()
	tasks := make([]*scheduledTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.mu.RUnlock()

	now := time.Now()

	// Check each task
	for _, task := range tasks {
		if err := s.scheduleTaskIfNeeded(ctx, task, now); err != nil {
			s.logger.ErrorContext(ctx, "failed to schedule task",
				slog.String("task_name", task.name),
				slog.String("error", err.Error()))
		}
	}
}

// scheduleTaskIfNeeded checks if a task should be scheduled and creates it if needed
func (s *Scheduler) scheduleTaskIfNeeded(ctx context.Context, task *scheduledTask, now time.Time) error {
	nextRun := s.calculateNextRun(task, now)

	// Scheduling decision: Only create task if due and not already pending
	if !s.shouldScheduleTask(task, nextRun, now) {
		return nil
	}

	// Idempotency check: Prevent duplicate tasks for same schedule period
	// Critical for reliability - ensures scheduler restarts don't create duplicates
	existing, err := s.repo.GetPendingTaskByName(ctx, task.name)
	if err == nil && existing != nil {
		// Task already exists for this period - sync our state
		s.updateTaskState(task.name, &existing.ScheduledAt)
		s.logger.DebugContext(ctx, "periodic task already pending",
			slog.String("task_name", task.name),
			slog.Time("scheduled_for", existing.ScheduledAt))
		return nil
	}

	// Create the task
	if err := s.createTask(ctx, task, nextRun); err != nil {
		return fmt.Errorf("failed to create periodic task: %w", err)
	}

	// Update state
	s.updateTaskState(task.name, &nextRun)

	// Log success
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

// calculateNextRun determines when the task should run next
func (s *Scheduler) calculateNextRun(task *scheduledTask, now time.Time) time.Time {
	if task.lastScheduledAt == nil {
		// First run: next run from now
		return task.schedule.Next(now)
	}
	// Subsequent runs: next run from last scheduled
	return task.schedule.Next(*task.lastScheduledAt)
}

// shouldScheduleTask determines if a task is due to be scheduled
func (s *Scheduler) shouldScheduleTask(task *scheduledTask, nextRun, now time.Time) bool {
	// First run is always scheduled
	if task.lastScheduledAt == nil {
		return true
	}

	// Skip if not due yet
	if nextRun.After(now) {
		// Use context.Background() for debug logging in this utility method
		s.logger.DebugContext(context.Background(), "periodic task not due yet",
			slog.String("task_name", task.name),
			slog.Time("next_run", nextRun))
		return false
	}

	return true
}

// updateTaskState updates the lastScheduledAt time for a task
func (s *Scheduler) updateTaskState(taskName string, scheduledAt *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.tasks[taskName]; ok {
		t.lastScheduledAt = scheduledAt
	}
}

// createTask creates a new task instance in the database
func (s *Scheduler) createTask(ctx context.Context, task *scheduledTask, scheduledAt time.Time) error {
	newTask := &Task{
		ID:          uuid.New(),
		Queue:       task.queue,
		TaskType:    TaskTypePeriodic,
		TaskName:    task.name,
		Payload:     nil, // Periodic tasks have no payload
		Status:      TaskStatusPending,
		Priority:    task.priority,
		RetryCount:  0,
		MaxRetries:  task.maxRetries,
		ScheduledAt: scheduledAt,
		CreatedAt:   time.Now(),
	}

	return s.repo.CreateTask(ctx, newTask)
}

// RemoveTask removes a periodic task from the scheduler
func (s *Scheduler) RemoveTask(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tasks, name)

	s.logger.InfoContext(context.Background(), "removed periodic task",
		slog.String("task_name", name))
}

// ListTasks returns all registered periodic tasks
func (s *Scheduler) ListTasks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.tasks))
	for name := range s.tasks {
		names = append(names, name)
	}
	return names
}
