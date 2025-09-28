package queue

import "time"

// Config holds the configuration for the task queue
type Config struct {
	// Worker configuration
	PollInterval       time.Duration `env:"QUEUE_POLL_INTERVAL" envDefault:"5s"`
	LockTimeout        time.Duration `env:"QUEUE_LOCK_TIMEOUT" envDefault:"5m"`
	ShutdownTimeout    time.Duration `env:"QUEUE_SHUTDOWN_TIMEOUT" envDefault:"30s"`
	MaxConcurrentTasks int           `env:"QUEUE_MAX_CONCURRENT_TASKS" envDefault:"10"`
	Queues             []string      `env:"QUEUE_WORKER_QUEUES" envDefault:"default" envSeparator:","`

	// Scheduler configuration
	CheckInterval time.Duration `env:"QUEUE_CHECK_INTERVAL" envDefault:"10s"`

	// Enqueuer configuration
	DefaultQueue    string   `env:"QUEUE_DEFAULT_QUEUE" envDefault:"default"`
	DefaultPriority Priority `env:"QUEUE_DEFAULT_PRIORITY" envDefault:"50"` // PriorityMedium
}

func DefaultConfig() Config {
	return Config{
		// Worker defaults
		PollInterval:       5 * time.Second,
		LockTimeout:        5 * time.Minute,
		ShutdownTimeout:    30 * time.Second,
		MaxConcurrentTasks: 10,
		Queues:             []string{"default"},

		// Scheduler defaults
		CheckInterval: 10 * time.Second,

		// Enqueuer defaults
		DefaultQueue:    "default",
		DefaultPriority: PriorityMedium,
	}
}
