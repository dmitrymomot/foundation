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

	"golang.org/x/sync/errgroup"
)

// ServiceState represents the lifecycle state of the service.
type ServiceState int32

const (
	// StateConfiguring indicates the service is being configured.
	// Handlers and scheduled tasks can only be registered in this state.
	StateConfiguring ServiceState = iota

	// StateRunning indicates the service is running.
	// No configuration changes are allowed in this state.
	StateRunning

	// StateStopped indicates the service has stopped.
	StateStopped
)

// String returns a string representation of the service state.
func (s ServiceState) String() string {
	switch s {
	case StateConfiguring:
		return "configuring"
	case StateRunning:
		return "running"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ServiceConfig holds runtime configuration for the service.
type ServiceConfig struct {
	// SkipWorkerIfNoHandlers skips starting the worker if no handlers are registered.
	SkipWorkerIfNoHandlers bool

	// SkipSchedulerIfNoTasks skips starting the scheduler if no tasks are scheduled.
	SkipSchedulerIfNoTasks bool

	// RequireHandlers causes Run() to fail if no handlers are registered.
	RequireHandlers bool

	// RequireScheduledTasks causes Run() to fail if no tasks are scheduled.
	RequireScheduledTasks bool
}

// Service provides a unified management interface for queue system components.
// It orchestrates Worker, Scheduler, and Enqueuer instances, handling their
// lifecycle and providing convenient access methods for other modules.
//
// The Service follows a configure-then-run pattern where all handlers and
// scheduled tasks must be registered before calling Run(). Once Run() is called,
// the service transitions to a running state and no further configuration is allowed.
//
// This design ensures thread-safety and eliminates race conditions between
// configuration and execution.
type Service struct {
	// Components
	worker    *Worker
	scheduler *Scheduler
	enqueuer  *Enqueuer
	storage   Storage
	logger    *slog.Logger

	// State management
	state   atomic.Int32 // Current service state
	stateMu sync.RWMutex // Protects state transitions

	// Readiness signaling
	ready    chan struct{} // Closed when service is fully started
	stopOnce sync.Once     // Ensures Stop() cleanup runs once

	// Configuration
	config ServiceConfig

	// Lifecycle hooks
	beforeStart func(context.Context) error
	afterStop   func() error
}

// NewService creates a new queue service with all components using the provided storage.
// The storage must implement the unified Storage interface that combines all repository interfaces.
// Options can be used to customize service behavior and component configuration.
//
// Example usage:
//
//	// Create storage (use your database implementation in production)
//	storage := queue.NewMemoryStorage()
//	defer storage.Close()
//
//	// Create service with options
//	service, err := queue.NewService(storage,
//	    queue.WithWorkerOptions(
//	        queue.WithPullInterval(100*time.Millisecond),
//	        queue.WithMaxConcurrentTasks(10),
//	    ),
//	    queue.WithServiceLogger(slog.Default()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Define task payload type
//	type EmailTask struct {
//	    To      string `json:"to"`
//	    Subject string `json:"subject"`
//	    Body    string `json:"body"`
//	}
//
//	// Register task handler
//	emailHandler := queue.NewTaskHandler(func(ctx context.Context, task EmailTask) error {
//	    fmt.Printf("Sending email to %s: %s\n", task.To, task.Subject)
//	    // Implement email sending logic
//	    return nil
//	})
//	service.RegisterHandler(emailHandler)
//
//	// Start service
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	go func() {
//	    if err := service.Run(ctx); err != nil {
//	        log.Printf("Service error: %v", err)
//	    }
//	}()
//
//	// Enqueue tasks
//	service.Enqueue(context.Background(), EmailTask{
//	    To:      "user@example.com",
//	    Subject: "Welcome!",
//	    Body:    "Welcome to our service",
//	})
//
//	// Enqueue with delay
//	service.EnqueueWithDelay(context.Background(), EmailTask{
//	    To:      "admin@example.com",
//	    Subject: "Reminder",
//	    Body:    "Delayed reminder",
//	}, 2*time.Second)
func NewService(storage Storage, opts ...ServiceOption) (*Service, error) {
	if storage == nil {
		return nil, ErrRepositoryNil
	}

	// Default service configuration with no-op logger
	s := &Service{
		storage: storage,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		ready:   make(chan struct{}),
		config: ServiceConfig{
			SkipWorkerIfNoHandlers: true,
			SkipSchedulerIfNoTasks: true,
		},
	}

	// Initialize state to configuring
	s.state.Store(int32(StateConfiguring))

	// Create enqueuer (always needed for enqueueing tasks)
	enqueuer, err := NewEnqueuer(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create enqueuer: %w", err)
	}
	s.enqueuer = enqueuer

	// Create worker
	worker, err := NewWorker(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}
	s.worker = worker

	// Create scheduler
	scheduler, err := NewScheduler(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}
	s.scheduler = scheduler

	// Apply service options (may override components)
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("failed to apply service option: %w", err)
		}
	}

	return s, nil
}

// NewServiceFromConfig creates a new queue service using configuration and storage.
// This factory method simplifies service initialization by applying configuration
// values to all components automatically. Additional options can override config values.
func NewServiceFromConfig(cfg Config, storage Storage, opts ...ServiceOption) (*Service, error) {
	// Create service with configured components
	// Option functions handle zero/empty values appropriately
	serviceOpts := append([]ServiceOption{
		WithWorkerOptions(
			WithPullInterval(cfg.PollInterval),
			WithLockTimeout(cfg.LockTimeout),
			WithMaxConcurrentTasks(cfg.MaxConcurrentTasks),
			WithQueues(cfg.Queues...),
		),
		WithSchedulerOptions(
			WithCheckInterval(cfg.CheckInterval),
		),
		WithEnqueuerOptions(
			WithDefaultQueue(cfg.DefaultQueue),
			WithDefaultPriority(cfg.DefaultPriority),
		),
	}, opts...)

	return NewService(storage, serviceOpts...)
}

// Run starts the queue service components in an error group.
// After calling Run(), no more handlers or scheduled tasks can be registered.
// Components are started conditionally based on service configuration:
// - Worker starts only if handlers are registered (unless forced)
// - Scheduler starts only if tasks are scheduled (unless forced)
//
// The method blocks until the context is cancelled or an error occurs.
func (s *Service) Run(ctx context.Context) error {
	// Transition to running state
	if !s.transitionToRunning() {
		return ErrServiceAlreadyRunning
	}

	// Ensure we transition to stopped state on exit
	defer func() {
		s.state.Store(int32(StateStopped))
	}()

	// Validate configuration
	if err := s.validate(); err != nil {
		return fmt.Errorf("service validation failed: %w", err)
	}

	// Run before start hook if provided
	if s.beforeStart != nil {
		if err := s.beforeStart(ctx); err != nil {
			return fmt.Errorf("before start hook failed: %w", err)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	// Start worker conditionally
	if s.shouldStartWorker() {
		eg.Go(func() error {
			// Get worker info through public methods
			handlerCount := s.worker.HandlerCount()
			queues := s.worker.Queues()

			s.logger.InfoContext(ctx, "starting queue worker",
				slog.Int("handlers", handlerCount),
				slog.Any("queues", queues),
			)
			return s.worker.Start(ctx)
		})
	} else {
		s.logger.InfoContext(ctx, "worker skipped (no handlers registered)")
	}

	// Start scheduler conditionally
	if s.shouldStartScheduler() {
		eg.Go(func() error {
			tasks := s.scheduler.ListTasks()
			s.logger.InfoContext(ctx, "starting queue scheduler",
				slog.Int("task_count", len(tasks)),
				slog.Any("tasks", tasks),
			)
			return s.scheduler.Start(ctx)
		})
	} else {
		s.logger.InfoContext(ctx, "scheduler skipped (no tasks scheduled)")
	}

	// Signal that service is ready
	close(s.ready)

	// Wait for all components
	err := eg.Wait()

	// Run after stop hook if provided
	s.stopOnce.Do(func() {
		if s.afterStop != nil {
			if stopErr := s.afterStop(); stopErr != nil {
				if err == nil {
					err = fmt.Errorf("after stop hook failed: %w", stopErr)
				} else {
					// Use context.Background() since original context may be cancelled
					s.logger.ErrorContext(context.Background(), "after stop hook failed", slog.String("error", stopErr.Error()))
				}
			}
		}
	})

	return err
}

// Stop gracefully stops the queue service components.
// This method should be called to ensure clean shutdown of workers.
func (s *Service) Stop() error {
	state := ServiceState(s.state.Load())
	if state != StateRunning {
		return fmt.Errorf("cannot stop service in state %s", state)
	}

	// Use context.Background() for stop operations
	ctx := context.Background()
	s.logger.InfoContext(ctx, "stopping queue service")

	// Stop worker if it was started
	if s.shouldStartWorker() {
		if err := s.worker.Stop(); err != nil {
			s.logger.ErrorContext(ctx, "failed to stop worker", slog.String("error", err.Error()))
			return fmt.Errorf("failed to stop worker: %w", err)
		}
	}

	// Run after stop hook
	s.stopOnce.Do(func() {
		if s.afterStop != nil {
			if err := s.afterStop(); err != nil {
				s.logger.ErrorContext(ctx, "after stop hook failed", slog.String("error", err.Error()))
			}
		}
	})

	s.state.Store(int32(StateStopped))
	return nil
}

// Worker returns the worker instance for handler registration.
func (s *Service) Worker() *Worker {
	return s.worker
}

// Scheduler returns the scheduler instance for task scheduling.
func (s *Service) Scheduler() *Scheduler {
	return s.scheduler
}

// Enqueuer returns the enqueuer instance for task enqueueing.
func (s *Service) Enqueuer() *Enqueuer {
	return s.enqueuer
}

// Storage returns the underlying storage implementation.
func (s *Service) Storage() Storage {
	return s.storage
}

// RegisterHandler registers a task handler with the worker.
// This method can only be called before Run().
func (s *Service) RegisterHandler(handler Handler) error {
	if !s.isConfiguring() {
		return ErrServiceNotConfiguring
	}
	return s.worker.RegisterHandler(handler)
}

// RegisterHandlers registers multiple task handlers with the worker.
// This method can only be called before Run().
func (s *Service) RegisterHandlers(handlers ...Handler) error {
	if !s.isConfiguring() {
		return ErrServiceNotConfiguring
	}
	return s.worker.RegisterHandlers(handlers...)
}

// AddScheduledTask registers a periodic task with the scheduler.
// This method can only be called before Run().
func (s *Service) AddScheduledTask(name string, schedule Schedule, opts ...SchedulerTaskOption) error {
	if !s.isConfiguring() {
		return ErrServiceNotConfiguring
	}
	return s.scheduler.AddTask(name, schedule, opts...)
}

// Enqueue adds a task to the queue.
// This is a convenience method equivalent to service.Enqueuer().Enqueue(ctx, payload, opts...).
func (s *Service) Enqueue(ctx context.Context, payload any, opts ...EnqueueOption) error {
	return s.enqueuer.Enqueue(ctx, payload, opts...)
}

// EnqueueWithDelay adds a task to the queue with a delay.
// This is a convenience method that adds the delay option to the enqueue call.
func (s *Service) EnqueueWithDelay(ctx context.Context, payload any, delay time.Duration, opts ...EnqueueOption) error {
	// Add delay option to the existing options
	allOpts := append([]EnqueueOption{WithDelay(delay)}, opts...)
	return s.enqueuer.Enqueue(ctx, payload, allOpts...)
}

// EnqueueAt adds a task to the queue to be executed at a specific time.
// This is a convenience method that adds the scheduled time option to the enqueue call.
func (s *Service) EnqueueAt(ctx context.Context, payload any, at time.Time, opts ...EnqueueOption) error {
	// Add scheduled time option to the existing options
	allOpts := append([]EnqueueOption{WithScheduledAt(at)}, opts...)
	return s.enqueuer.Enqueue(ctx, payload, allOpts...)
}

// Ready returns a channel that is closed when the service is fully started.
// This is useful for testing and coordination.
func (s *Service) Ready() <-chan struct{} {
	return s.ready
}

// State returns the current service state.
func (s *Service) State() ServiceState {
	return ServiceState(s.state.Load())
}

// --- Private helper methods ---

func (s *Service) isConfiguring() bool {
	return ServiceState(s.state.Load()) == StateConfiguring
}

func (s *Service) transitionToRunning() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	current := ServiceState(s.state.Load())
	if current != StateConfiguring {
		return false
	}

	s.state.Store(int32(StateRunning))
	return true
}

func (s *Service) validate() error {
	handlerCount := s.worker.HandlerCount()
	taskCount := len(s.scheduler.ListTasks())

	if s.config.RequireHandlers && handlerCount == 0 {
		return errors.New("no handlers registered (RequireHandlers is true)")
	}

	if s.config.RequireScheduledTasks && taskCount == 0 {
		return errors.New("no scheduled tasks registered (RequireScheduledTasks is true)")
	}

	return nil
}

func (s *Service) shouldStartWorker() bool {
	if !s.config.SkipWorkerIfNoHandlers {
		return true
	}
	return s.worker.HandlerCount() > 0
}

func (s *Service) shouldStartScheduler() bool {
	if !s.config.SkipSchedulerIfNoTasks {
		return true
	}
	return len(s.scheduler.ListTasks()) > 0
}
