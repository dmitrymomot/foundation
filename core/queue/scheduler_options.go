package queue

import (
	"log/slog"
	"time"
)

// SchedulerOption is a functional option for configuring a scheduler
type SchedulerOption func(*schedulerOptions)

type schedulerOptions struct {
	checkInterval   time.Duration
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

// WithCheckInterval configures how frequently the scheduler checks for due tasks.
// Shorter intervals provide more precise scheduling but increase CPU usage.
func WithCheckInterval(d time.Duration) SchedulerOption {
	return func(o *schedulerOptions) {
		if d > 0 {
			o.checkInterval = d
		}
	}
}

// WithSchedulerShutdownTimeout configures maximum wait time for active checks during shutdown.
// Scheduler will wait this long for in-flight operations to complete before forcing shutdown.
func WithSchedulerShutdownTimeout(d time.Duration) SchedulerOption {
	return func(o *schedulerOptions) {
		if d > 0 {
			o.shutdownTimeout = d
		}
	}
}

// WithSchedulerLogger configures structured logging for scheduler operations.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithSchedulerLogger(logger *slog.Logger) SchedulerOption {
	return func(o *schedulerOptions) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// SchedulerTaskOption is a functional option for configuring a scheduled task
type SchedulerTaskOption func(*schedulerTaskOptions)

type schedulerTaskOptions struct {
	queue      string
	priority   Priority
	maxRetries int8
}

// WithTaskQueue specifies which queue the scheduled task instances should be enqueued to.
// Allows routing scheduled tasks to specific workers.
func WithTaskQueue(queue string) SchedulerTaskOption {
	return func(o *schedulerTaskOptions) {
		if queue != "" {
			o.queue = queue
		}
	}
}

// WithTaskPriority sets the priority for scheduled task instances.
// Higher priority tasks are processed before lower priority ones.
func WithTaskPriority(priority Priority) SchedulerTaskOption {
	return func(o *schedulerTaskOptions) {
		if priority.Valid() {
			o.priority = priority
		}
	}
}

// WithTaskMaxRetries configures retry behavior for scheduled task instances.
// Capped at 10 to prevent infinite retry loops on persistent failures.
func WithTaskMaxRetries(maxRetries int8) SchedulerTaskOption {
	return func(o *schedulerTaskOptions) {
		if maxRetries >= 0 && maxRetries <= 10 {
			o.maxRetries = maxRetries
		}
	}
}
