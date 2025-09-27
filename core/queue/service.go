package queue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

// Service provides a unified management interface for queue system components.
// It orchestrates Worker, Scheduler, and Enqueuer instances, handling their
// lifecycle and providing convenient access methods for other modules.
//
// The Service is designed to be easily integrated into applications requiring
// background task processing, scheduled jobs, and asynchronous work queues.
type Service struct {
	worker    *Worker
	scheduler *Scheduler
	enqueuer  *Enqueuer
	storage   Storage
	logger    *slog.Logger

	// Configuration for conditional startup
	skipWorkerIfNoHandlers bool
	skipSchedulerIfNoTasks bool

	// Hooks for custom initialization
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
		storage:                storage,
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		skipWorkerIfNoHandlers: true,
		skipSchedulerIfNoTasks: true,
	}

	// Create default components first
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
// Components are started conditionally based on service configuration:
// - Worker starts only if handlers are registered (unless forced)
// - Scheduler starts only if tasks are scheduled (unless forced)
//
// The method blocks until the context is cancelled or an error occurs.
func (s *Service) Run(ctx context.Context) error {
	// Run before start hook if provided
	if s.beforeStart != nil {
		if err := s.beforeStart(ctx); err != nil {
			return fmt.Errorf("before start hook failed: %w", err)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	// Start worker conditionally
	eg.Go(func() error {
		// Check if worker should be skipped
		if s.skipWorkerIfNoHandlers && len(s.worker.handlers) == 0 {
			s.logger.InfoContext(ctx, "no task handlers registered, worker will not start")
			return nil
		}

		queues := s.worker.queues
		if len(queues) == 0 {
			queues = []string{DefaultQueueName}
		}

		s.logger.InfoContext(ctx, "starting queue worker",
			slog.Any("queues", queues),
		)

		err := s.worker.Start(ctx)
		if errors.Is(err, ErrNoHandlers) && s.skipWorkerIfNoHandlers {
			s.logger.InfoContext(ctx, "no task handlers registered, worker stopped")
			return nil
		}
		return err
	})

	// Start scheduler conditionally
	eg.Go(func() error {
		tasks := s.scheduler.ListTasks()

		// Check if scheduler should be skipped
		if s.skipSchedulerIfNoTasks && len(tasks) == 0 {
			s.logger.InfoContext(ctx, "no scheduled tasks registered, scheduler will not start")
			return nil
		}

		s.logger.InfoContext(ctx, "starting queue scheduler",
			slog.Int("task_count", len(tasks)),
		)

		return s.scheduler.Start(ctx)
	})

	// Wait for all components
	err := eg.Wait()

	// Run after stop hook if provided
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

	return err
}

// Stop gracefully stops the queue service components.
// This method should be called to ensure clean shutdown of workers.
func (s *Service) Stop() error {
	// Use context.Background() for stop operations
	ctx := context.Background()
	s.logger.InfoContext(ctx, "stopping queue service")

	// Stop worker (it has graceful shutdown logic)
	if err := s.worker.Stop(); err != nil {
		s.logger.ErrorContext(ctx, "failed to stop worker", slog.String("error", err.Error()))
		return err
	}

	// Run after stop hook if provided
	if s.afterStop != nil {
		if err := s.afterStop(); err != nil {
			s.logger.ErrorContext(ctx, "after stop hook failed", slog.String("error", err.Error()))
			return err
		}
	}

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
// This is a convenience method equivalent to service.Worker().RegisterHandler(handler).
func (s *Service) RegisterHandler(handler Handler) error {
	return s.worker.RegisterHandler(handler)
}

// RegisterHandlers registers multiple task handlers with the worker.
// This is a convenience method equivalent to service.Worker().RegisterHandlers(handlers).
func (s *Service) RegisterHandlers(handlers ...Handler) error {
	return s.worker.RegisterHandlers(handlers...)
}

// AddScheduledTask registers a periodic task with the scheduler.
// This is a convenience method equivalent to service.Scheduler().AddTask(name, schedule, opts...).
func (s *Service) AddScheduledTask(name string, schedule Schedule, opts ...SchedulerTaskOption) error {
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
