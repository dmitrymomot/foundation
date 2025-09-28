package queue

import (
	"log/slog"
	"time"
)

// SchedulerOption is a functional option for configuring a scheduler
type SchedulerOption func(*schedulerOptions)

type schedulerOptions struct {
	checkInterval time.Duration
	logger        *slog.Logger
}

func WithCheckInterval(d time.Duration) SchedulerOption {
	return func(o *schedulerOptions) {
		if d > 0 {
			o.checkInterval = d
		}
	}
}

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

func WithTaskQueue(queue string) SchedulerTaskOption {
	return func(o *schedulerTaskOptions) {
		if queue != "" {
			o.queue = queue
		}
	}
}

func WithTaskPriority(priority Priority) SchedulerTaskOption {
	return func(o *schedulerTaskOptions) {
		if priority.Valid() {
			o.priority = priority
		}
	}
}

// WithTaskMaxRetries sets the max retries for the scheduled task (0-10)
// Capped at 10 to prevent infinite retry loops on persistent failures
func WithTaskMaxRetries(maxRetries int8) SchedulerTaskOption {
	return func(o *schedulerTaskOptions) {
		if maxRetries >= 0 && maxRetries <= 10 {
			o.maxRetries = maxRetries
		}
	}
}
