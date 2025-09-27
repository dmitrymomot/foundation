package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EnqueuerRepository defines the interface for task creation.
type EnqueuerRepository interface {
	CreateTask(ctx context.Context, task *Task) error
}

// Enqueuer handles task enqueueing with configurable defaults.
type Enqueuer struct {
	repo            EnqueuerRepository
	defaultQueue    string
	defaultPriority Priority
}

// NewEnqueuer creates a new Enqueuer with the given repository and options.
func NewEnqueuer(repo EnqueuerRepository, opts ...EnqueuerOption) (*Enqueuer, error) {
	if repo == nil {
		return nil, ErrRepositoryNil
	}

	options := &enqueuerOptions{
		defaultQueue:    DefaultQueueName,
		defaultPriority: PriorityDefault,
	}

	for _, opt := range opts {
		opt(options)
	}

	return &Enqueuer{
		repo:            repo,
		defaultQueue:    options.defaultQueue,
		defaultPriority: options.defaultPriority,
	}, nil
}

// NewEnqueuerFromConfig creates an Enqueuer from configuration.
// Repository must be provided. Additional options can override config values.
func NewEnqueuerFromConfig(cfg Config, repo EnqueuerRepository, opts ...EnqueuerOption) (*Enqueuer, error) {
	if repo == nil {
		return nil, ErrRepositoryNil
	}

	// Build options from config
	configOpts := make([]EnqueuerOption, 0)

	// Apply enqueuer configuration
	if cfg.DefaultQueue != "" {
		configOpts = append(configOpts, WithDefaultQueue(cfg.DefaultQueue))
	}
	if cfg.DefaultPriority.Valid() {
		configOpts = append(configOpts, WithDefaultPriority(cfg.DefaultPriority))
	}

	// Combine config options with user-provided options (user options override)
	allOpts := append(configOpts, opts...)

	return NewEnqueuer(repo, allOpts...)
}

// Enqueue adds a new task to the queue with the given payload and options.
func (e *Enqueuer) Enqueue(ctx context.Context, payload any, opts ...EnqueueOption) error {
	if payload == nil {
		return ErrPayloadNil
	}

	// Apply default options from enqueuer configuration
	options := &enqueueOptions{
		queue:      e.defaultQueue,
		priority:   e.defaultPriority,
		maxRetries: 3,
	}

	// Apply user-provided options to override defaults
	for _, opt := range opts {
		opt(options)
	}

	// Validate priority is within allowed range
	if !options.priority.Valid() {
		return ErrInvalidPriority
	}

	// Build task with payload and options
	task, err := e.buildTask(payload, options)
	if err != nil {
		return err
	}

	// Store task in repository
	if err := e.repo.CreateTask(ctx, task); err != nil {
		return fmt.Errorf("failed to create task %q in queue %q: %w", task.TaskName, task.Queue, err)
	}

	return nil
}

// buildTask constructs a Task from payload and options.
// Marshals payload to JSON and generates UUID and timestamps.
func (e *Enqueuer) buildTask(payload any, options *enqueueOptions) (*Task, error) {
	// Marshal payload to JSON for storage
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload of type %T: %w", payload, err)
	}

	// Determine task name from options or infer from payload type
	taskName := options.taskName
	if taskName == "" {
		taskName = qualifiedStructName(payload)
	}

	// Calculate scheduled time based on delay or explicit scheduling
	scheduledAt := time.Now()
	if options.scheduledAt != nil {
		scheduledAt = *options.scheduledAt
	} else if options.delay > 0 {
		scheduledAt = scheduledAt.Add(options.delay)
	}

	return &Task{
		ID:          uuid.New(),
		Queue:       options.queue,
		TaskType:    TaskTypeOneTime,
		TaskName:    taskName,
		Payload:     payloadBytes,
		Status:      TaskStatusPending,
		Priority:    options.priority,
		RetryCount:  0,
		MaxRetries:  options.maxRetries,
		ScheduledAt: scheduledAt,
		CreatedAt:   time.Now(),
	}, nil
}
