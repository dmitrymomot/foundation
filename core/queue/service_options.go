package queue

import (
	"context"
	"log/slog"
)

// ServiceOption configures a Service instance.
type ServiceOption func(*Service) error

// WithServiceLogger sets the logger for the service and its components.
func WithServiceLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) error {
		if logger == nil {
			return nil // Just use the default logger
		}
		s.logger = logger

		// Also set logger for components
		if s.worker != nil {
			s.worker.logger = logger.With(slog.String("component", "worker"))
		}
		if s.scheduler != nil {
			s.scheduler.logger = logger.With(slog.String("component", "scheduler"))
		}

		return nil
	}
}

// WithWorkerOptions applies options to the worker component.
// These options are applied when the worker is created.
func WithWorkerOptions(opts ...WorkerOption) ServiceOption {
	return func(s *Service) error {
		// Create new worker with options
		worker, err := NewWorker(s.storage, opts...)
		if err != nil {
			return err
		}
		s.worker = worker
		return nil
	}
}

// WithSchedulerOptions applies options to the scheduler component.
// These options are applied when the scheduler is created.
func WithSchedulerOptions(opts ...SchedulerOption) ServiceOption {
	return func(s *Service) error {
		// Create new scheduler with options
		scheduler, err := NewScheduler(s.storage, opts...)
		if err != nil {
			return err
		}
		s.scheduler = scheduler
		return nil
	}
}

// WithEnqueuerOptions applies options to the enqueuer component.
// These options are applied when the enqueuer is created.
func WithEnqueuerOptions(opts ...EnqueuerOption) ServiceOption {
	return func(s *Service) error {
		// Create new enqueuer with options
		enqueuer, err := NewEnqueuer(s.storage, opts...)
		if err != nil {
			return err
		}
		s.enqueuer = enqueuer
		return nil
	}
}

// WithSkipWorkerIfNoHandlers configures whether the worker should be skipped
// if no handlers are registered. Default is true.
func WithSkipWorkerIfNoHandlers(skip bool) ServiceOption {
	return func(s *Service) error {
		s.skipWorkerIfNoHandlers = skip
		return nil
	}
}

// WithSkipSchedulerIfNoTasks configures whether the scheduler should be skipped
// if no tasks are scheduled. Default is true.
func WithSkipSchedulerIfNoTasks(skip bool) ServiceOption {
	return func(s *Service) error {
		s.skipSchedulerIfNoTasks = skip
		return nil
	}
}

// WithBeforeStart sets a hook that runs before the service starts.
// This can be used for custom initialization logic.
func WithBeforeStart(hook func(context.Context) error) ServiceOption {
	return func(s *Service) error {
		s.beforeStart = hook
		return nil
	}
}

// WithAfterStop sets a hook that runs after the service stops.
// This can be used for cleanup logic.
func WithAfterStop(hook func() error) ServiceOption {
	return func(s *Service) error {
		s.afterStop = hook
		return nil
	}
}

// WithHandlers registers task handlers with the worker during service creation.
// This is a convenience option for registering handlers at initialization time.
func WithHandlers(handlers ...Handler) ServiceOption {
	return func(s *Service) error {
		if s.worker == nil {
			// Create worker if not already created
			worker, err := NewWorker(s.storage)
			if err != nil {
				return err
			}
			s.worker = worker
		}

		for _, handler := range handlers {
			if err := s.worker.RegisterHandler(handler); err != nil {
				return err
			}
		}

		return nil
	}
}

// WithScheduledTasks registers scheduled tasks with the scheduler during service creation.
// This is a convenience option for setting up periodic tasks at initialization time.
func WithScheduledTasks(tasks map[string]struct {
	Schedule Schedule
	Options  []SchedulerTaskOption
}) ServiceOption {
	return func(s *Service) error {
		if s.scheduler == nil {
			// Create scheduler if not already created
			scheduler, err := NewScheduler(s.storage)
			if err != nil {
				return err
			}
			s.scheduler = scheduler
		}

		for name, task := range tasks {
			if err := s.scheduler.AddTask(name, task.Schedule, task.Options...); err != nil {
				return err
			}
		}

		return nil
	}
}
