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
	shutdownTimeout    time.Duration
	maxConcurrentTasks int
	logger             *slog.Logger
}

// WithQueues specifies which queues this worker should process tasks from.
// Worker will claim tasks from any of the specified queues based on priority.
func WithQueues(queues ...string) WorkerOption {
	return func(o *workerOptions) {
		o.queues = queues
	}
}

// WithPullInterval configures how frequently the worker checks for new tasks.
// Shorter intervals reduce task latency but increase database load.
func WithPullInterval(d time.Duration) WorkerOption {
	return func(o *workerOptions) {
		if d > 0 {
			o.pullInterval = d
		}
	}
}

// WithLockTimeout sets how long a worker holds exclusive lock on a claimed task.
// Tasks exceeding this duration may be reclaimed by other workers.
// Use ExtendLockForTask for legitimately long-running operations.
func WithLockTimeout(d time.Duration) WorkerOption {
	return func(o *workerOptions) {
		if d > 0 {
			o.lockTimeout = d
		}
	}
}

// WithMaxConcurrentTasks limits how many tasks can be processed simultaneously.
// Tune based on workload characteristics and available resources.
func WithMaxConcurrentTasks(n int) WorkerOption {
	return func(o *workerOptions) {
		if n > 0 {
			o.maxConcurrentTasks = n
		}
	}
}

// WithShutdownTimeout configures maximum wait time for active tasks during shutdown.
// Worker will wait this long for running tasks to complete before forcing shutdown.
func WithShutdownTimeout(d time.Duration) WorkerOption {
	return func(o *workerOptions) {
		if d > 0 {
			o.shutdownTimeout = d
		}
	}
}

// WithWorkerLogger configures structured logging for worker operations.
// Use slog.New(slog.NewTextHandler(io.Discard, nil)) to disable logging.
func WithWorkerLogger(logger *slog.Logger) WorkerOption {
	return func(o *workerOptions) {
		if logger != nil {
			o.logger = logger
		}
	}
}
