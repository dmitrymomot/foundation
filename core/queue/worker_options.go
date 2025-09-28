package queue

import (
	"log/slog"
	"time"
)

// WorkerOption is a functional option for configuring a worker
type WorkerOption func(*workerOptions)

type workerOptions struct {
	queues             []string
	pullInterval       time.Duration
	lockTimeout        time.Duration
	maxConcurrentTasks int
	logger             *slog.Logger
}

func WithQueues(queues ...string) WorkerOption {
	return func(o *workerOptions) {
		o.queues = queues
	}
}

func WithPullInterval(d time.Duration) WorkerOption {
	return func(o *workerOptions) {
		if d > 0 {
			o.pullInterval = d
		}
	}
}

func WithLockTimeout(d time.Duration) WorkerOption {
	return func(o *workerOptions) {
		if d > 0 {
			o.lockTimeout = d
		}
	}
}

func WithMaxConcurrentTasks(n int) WorkerOption {
	return func(o *workerOptions) {
		if n > 0 {
			o.maxConcurrentTasks = n
		}
	}
}

func WithWorkerLogger(logger *slog.Logger) WorkerOption {
	return func(o *workerOptions) {
		if logger != nil {
			o.logger = logger
		}
	}
}
