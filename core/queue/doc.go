// Package queue provides a comprehensive job queue system with workers, scheduling,
// and priority-based task processing. It supports both immediate task execution
// and scheduled task processing with configurable retry mechanisms and error handling.
//
// The package is designed around three core components:
//   - Enqueuer: Creates and submits tasks to queues
//   - Worker: Processes tasks from queues with concurrent execution
//   - Scheduler: Creates periodic tasks based on flexible schedules
//
// # Features
//
//   - Task enqueueing with priority support (0-100 scale)
//   - Background workers with configurable concurrent processing
//   - Scheduled task execution with flexible scheduling options
//   - Configurable retry policies with exponential backoff (max 10 retries)
//   - In-memory storage for testing and development
//   - Extensible repository interface for custom storage backends
//   - Graceful shutdown with proper cleanup using Run() and Stop() methods
//   - Type-safe task handlers using Go generics
//   - Dead letter queue for failed tasks that exhausted retries
//   - Multiple queue support for task categorization
//   - Task locking mechanism to prevent duplicate processing
//   - Comprehensive error handling and logging support
//
// # Basic Usage
//
// Create a queue system with enqueuer, worker, and scheduler:
//
//	import "github.com/dmitrymomot/foundation/core/queue"
//
//	// Create storage (in-memory for development)
//	storage := queue.NewMemoryStorage()
//	defer storage.Close()
//
//	// Create enqueuer for adding tasks
//	enqueuer, err := queue.NewEnqueuer(storage,
//		queue.WithDefaultQueue("email"),
//		queue.WithDefaultPriority(queue.PriorityHigh),
//	)
//
//	// Create worker for processing tasks
//	worker, err := queue.NewWorker(storage,
//		queue.WithQueues("email"),
//		queue.WithMaxConcurrentTasks(5),
//		queue.WithPullInterval(time.Second),
//	)
//
//	// Define payload type
//	type EmailPayload struct {
//		To      string `json:"to"`
//		Subject string `json:"subject"`
//		Body    string `json:"body"`
//	}
//
//	// Register type-safe handler
//	handler := queue.NewTaskHandler(func(ctx context.Context, email EmailPayload) error {
//		// Send email logic here
//		return sendEmail(email.To, email.Subject, email.Body)
//	})
//	worker.RegisterHandler(handler)
//
//	// Start worker
//	ctx := context.Background()
//	go worker.Start(ctx)
//
//	// Enqueue tasks
//	err = enqueuer.Enqueue(ctx, EmailPayload{
//		To:      "user@example.com",
//		Subject: "Welcome!",
//		Body:    "Welcome to our service!",
//	})
//
// # Task Types and Handlers
//
// The package provides type-safe handlers using Go generics. Handler names are automatically
// derived from the payload type:
//
//	type EmailPayload struct {
//		To      string `json:"to"`
//		Subject string `json:"subject"`
//		Body    string `json:"body"`
//	}
//
//	type ImageProcessPayload struct {
//		ImageURL string `json:"image_url"`
//		Width    int    `json:"width"`
//		Height   int    `json:"height"`
//	}
//
//	// Type-safe handler with automatic unmarshaling
//	emailHandler := queue.NewTaskHandler(func(ctx context.Context, email EmailPayload) error {
//		return emailService.Send(email.To, email.Subject, email.Body)
//	})
//
//	imageHandler := queue.NewTaskHandler(func(ctx context.Context, img ImageProcessPayload) error {
//		return imageProcessor.Resize(img.ImageURL, img.Width, img.Height)
//	})
//
//	// Register handlers
//	worker.RegisterHandlers(emailHandler, imageHandler)
//
// # Priority-Based Processing
//
// Use different priority levels for task processing:
//
//	// High priority tasks (processed first)
//	enqueuer.Enqueue(ctx, CriticalPayload{...},
//		queue.WithPriority(queue.PriorityMax),
//	)
//
//	// Normal priority tasks
//	enqueuer.Enqueue(ctx, StandardPayload{...},
//		queue.WithPriority(queue.PriorityMedium),
//	)
//
//	// Low priority tasks (processed last)
//	enqueuer.Enqueue(ctx, CleanupPayload{...},
//		queue.WithPriority(queue.PriorityLow),
//	)
//
// # Scheduled Tasks
//
// Create recurring tasks with flexible scheduling options:
//
//	// Create scheduler
//	scheduler, err := queue.NewScheduler(storage,
//		queue.WithSchedulerLogger(logger),
//	)
//
//	// Register periodic handler (no payload)
//	reportHandler := queue.NewPeriodicTaskHandler("daily_report", func(ctx context.Context) error {
//		return generateDailyReport()
//	})
//	worker.RegisterHandler(reportHandler)
//
//	// Schedule daily reports at 9 AM
//	scheduler.AddTask("daily_report", queue.DailyAt(9, 0),
//		queue.WithTaskPriority(queue.PriorityHigh),
//	)
//
//	// Schedule weekly cleanup on Mondays at 2 AM
//	scheduler.AddTask("weekly_cleanup", queue.WeeklyOn(time.Monday, 2, 0))
//
//	// Schedule with intervals
//	scheduler.AddTask("health_check", queue.EveryMinutes(5))
//
//	// Start scheduler (multiple options)
//
//	// Option 1: Simple start with context
//	go scheduler.Start(ctx)
//
//	// Option 2: Use Run() for errgroup pattern
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(scheduler.Run(ctx))
//
//	// Option 3: Manual lifecycle management
//	go func() {
//		if err := scheduler.Start(ctx); err != nil {
//			log.Printf("scheduler error: %v", err)
//		}
//	}()
//	// Later: scheduler.Stop() for graceful shutdown
//
// # Retry Mechanisms
//
// Configure retry policies for failed tasks:
//
//	// Set max retries when enqueueing
//	enqueuer.Enqueue(ctx, payload,
//		queue.WithMaxRetries(5), // Will retry up to 5 times
//	)
//
//	// Handle task with retry logic - retries are automatic
//	handler := queue.NewTaskHandler(func(ctx context.Context, data ProcessingPayload) error {
//		err := performOperation(data)
//		if err != nil {
//			// Return error to trigger retry with exponential backoff
//			return fmt.Errorf("operation failed: %w", err)
//		}
//		return nil
//	})
//
// # Multiple Queues
//
// Set up different queues for different task types:
//
//	// Email queue worker
//	emailWorker, _ := queue.NewWorker(storage,
//		queue.WithQueues("email"),
//		queue.WithMaxConcurrentTasks(10),
//	)
//	emailWorker.RegisterHandler(welcomeEmailHandler)
//	emailWorker.RegisterHandler(notificationHandler)
//
//	// Image processing queue worker (CPU intensive, fewer workers)
//	imageWorker, _ := queue.NewWorker(storage,
//		queue.WithQueues("images"),
//		queue.WithMaxConcurrentTasks(3),
//	)
//	imageWorker.RegisterHandler(resizeHandler)
//	imageWorker.RegisterHandler(thumbnailHandler)
//
//	// Analytics queue worker
//	analyticsWorker, _ := queue.NewWorker(storage,
//		queue.WithQueues("analytics"),
//		queue.WithMaxConcurrentTasks(2),
//	)
//	analyticsWorker.RegisterHandler(eventTrackingHandler)
//
//	// Start all workers
//	go emailWorker.Start(ctx)
//	go imageWorker.Start(ctx)
//	go analyticsWorker.Start(ctx)
//
// # Error Handling and Dead Letter Queue
//
// Failed tasks are automatically handled with retries and dead letter queue:
//
//	handler := queue.NewTaskHandler(func(ctx context.Context, data ProcessingPayload) error {
//		// Check context cancellation
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		default:
//		}
//
//		// Process task - errors trigger automatic retries
//		if err := processData(data); err != nil {
//			return fmt.Errorf("processing failed: %w", err)
//		}
//		return nil
//	})
//
//	// Tasks that exceed max retries are moved to dead letter queue
//	// Failed tasks can be inspected via the storage interface
//
// # Graceful Shutdown
//
// Implement proper shutdown procedures:
//
//	func runQueueSystem(ctx context.Context) error {
//		// Create components
//		storage := queue.NewMemoryStorage()
//		defer storage.Close()
//
//		worker, _ := queue.NewWorker(storage)
//		scheduler, _ := queue.NewScheduler(storage)
//
//		// Option 1: Using errgroup for coordinated lifecycle
//		g, ctx := errgroup.WithContext(context.Background())
//		g.Go(worker.Run(ctx))
//		g.Go(scheduler.Run(ctx))
//
//		// Wait for all components to finish
//		if err := g.Wait(); err != nil {
//			log.Fatal(err)
//		}
//
//		// Option 2: Manual lifecycle management
//		go worker.Start(ctx)
//		go scheduler.Start(ctx)
//
//		// Wait for shutdown signal
//		<-ctx.Done()
//
//		// Graceful shutdown
//		worker.Stop()    // Waits for active tasks to complete
//		scheduler.Stop() // Waits for active checks to complete
//
//		log.Info("Queue system shutdown complete")
//		return nil
//	}
//
// # Custom Storage Backend
//
// Implement custom storage for production use by satisfying the repository interfaces:
//
//	type PostgreSQLStorage struct {
//		db *sql.DB
//	}
//
//	// Implement EnqueuerRepository
//	func (s *PostgreSQLStorage) CreateTask(ctx context.Context, task *queue.Task) error {
//		query := `INSERT INTO tasks (id, queue, task_type, task_name, priority, payload, status, created_at)
//		         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
//		_, err := s.db.ExecContext(ctx, query,
//			task.ID, task.Queue, task.TaskType, task.TaskName,
//			task.Priority, task.Payload, task.Status, task.CreatedAt)
//		return err
//	}
//
//	// Implement WorkerRepository methods: ClaimTask, CompleteTask, FailTask, etc.
//	// Implement SchedulerRepository methods: GetPendingTaskByName
//
//	// Use custom storage
//	storage := &PostgreSQLStorage{db: database}
//	enqueuer, _ := queue.NewEnqueuer(storage)
//	worker, _ := queue.NewWorker(storage)
//
// # Delayed Tasks
//
// Schedule tasks for future execution:
//
//	// Enqueue with delay
//	enqueuer.Enqueue(ctx, payload,
//		queue.WithDelay(time.Hour), // Process in 1 hour
//	)
//
//	// Enqueue at specific time
//	enqueuer.Enqueue(ctx, payload,
//		queue.WithScheduledAt(time.Date(2024, 12, 25, 9, 0, 0, 0, time.UTC)),
//	)
//
// # Storage Interfaces
//
// The package defines three repository interfaces for different components:
//
// ## EnqueuerRepository
//
//	type EnqueuerRepository interface {
//		CreateTask(ctx context.Context, task *Task) error
//	}
//
// ## WorkerRepository
//
//	type WorkerRepository interface {
//		ClaimTask(ctx context.Context, workerID uuid.UUID, queues []string, lockDuration time.Duration) (*Task, error)
//		CompleteTask(ctx context.Context, taskID uuid.UUID) error
//		FailTask(ctx context.Context, taskID uuid.UUID, errorMsg string) error
//		MoveToDLQ(ctx context.Context, taskID uuid.UUID) error
//		ExtendLock(ctx context.Context, taskID uuid.UUID, duration time.Duration) error
//	}
//
// ## SchedulerRepository
//
//	type SchedulerRepository interface {
//		CreateTask(ctx context.Context, task *Task) error
//		GetPendingTaskByName(ctx context.Context, taskName string) (*Task, error)
//	}
//
// Use queue.NewMemoryStorage() for development or implement custom storage for production.
// The memory storage implements all three interfaces and provides thread-safe operations.
//
// # Graceful Shutdown Support
//
// Both Worker and Scheduler support graceful shutdown with two complementary approaches:
//
// ## Manual Lifecycle Management
//
// Use Start() and Stop() methods for direct control:
//
//	// Start components
//	go worker.Start(ctx)
//	go scheduler.Start(ctx)
//
//	// Later, graceful shutdown
//	worker.Stop()    // Waits for all active tasks to complete
//	scheduler.Stop() // Waits for all active checks to complete
//
// ## errgroup Pattern with Run()
//
// The Run() method provides errgroup compatibility for coordinated shutdown:
//
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(worker.Run(ctx))     // Returns func() error for errgroup
//	g.Go(scheduler.Run(ctx))  // Returns func() error for errgroup
//
//	// Blocks until context cancellation or error
//	if err := g.Wait(); err != nil {
//		log.Fatal(err)
//	}
//
// The Run() method automatically handles Start() and Stop() lifecycle:
// 1. Calls Start() internally when the returned function executes
// 2. Monitors context cancellation
// 3. Calls Stop() for graceful shutdown when context is cancelled
// 4. Returns nil for normal shutdown, error for unexpected failures
//
// This pattern is ideal for coordinated shutdown of multiple components
// where all must stop gracefully before the application exits.
// # Core Types and Constants
//
// ## Task Priorities
//
//	const (
//		PriorityMin     Priority = 0   // Lowest priority
//		PriorityLow     Priority = 25  // Low priority
//		PriorityMedium  Priority = 50  // Default priority
//		PriorityHigh    Priority = 75  // High priority
//		PriorityMax     Priority = 100 // Highest priority
//	)
//
// ## Task Types
//
//	const (
//		TaskTypeOneTime  TaskType = "one-time"  // Regular tasks
//		TaskTypePeriodic TaskType = "periodic" // Scheduled tasks
//	)
//
// ## Task Status
//
//	const (
//		TaskStatusPending    TaskStatus = "pending"    // Waiting to be processed
//		TaskStatusProcessing TaskStatus = "processing" // Currently being processed
//		TaskStatusCompleted  TaskStatus = "completed"  // Successfully completed
//		TaskStatusFailed     TaskStatus = "failed"     // Failed (may retry)
//	)
//
// # Schedule Types
//
// The package provides various schedule types for periodic tasks:
//
//	// Interval-based schedules
//	EveryInterval(time.Hour)    // Every N duration
//	EveryMinutes(30)            // Every N minutes
//	EveryHours(6)              // Every N hours
//	EveryMinute()              // Every minute
//	Hourly()                   // Every hour at :00
//
//	// Time-based schedules
//	DailyAt(9, 30)             // Daily at 9:30 AM
//	WeeklyOn(time.Monday, 9, 0) // Weekly on Monday at 9:00 AM
//	MonthlyOn(1, 0, 0)         // Monthly on 1st at midnight
//	HourlyAt(15)               // Every hour at :15
//
//	// Convenience schedules
//	Daily()                    // Daily at midnight
//	Weekly(time.Friday)        // Weekly on Friday at midnight
//	Monthly(15)                // Monthly on 15th at midnight
//
// # Configuration Options
//
// ## Enqueuer Options
//
//	WithDefaultQueue("email")           // Set default queue name
//	WithDefaultPriority(PriorityHigh)   // Set default priority
//
// ## Worker Options
//
//	WithQueues("email", "sms")          // Queues to process
//	WithMaxConcurrentTasks(10)          // Max concurrent tasks
//	WithPullInterval(5*time.Second)     // How often to check for tasks
//	WithLockTimeout(5*time.Minute)      // Task lock duration
//	WithWorkerLogger(logger)            // Structured logger
//
// ## Scheduler Options
//
//	WithCheckInterval(30*time.Second)   // How often to check schedules
//	WithSchedulerLogger(logger)         // Structured logger
//
// ## Task Options (for Enqueue)
//
//	WithQueue("priority")               // Override queue
//	WithPriority(PriorityMax)           // Override priority
//	WithMaxRetries(5)                   // Max retry attempts (0-10)
//	WithDelay(time.Hour)                // Delay before processing
//	WithScheduledAt(futureTime)         // Schedule for specific time
//	WithTaskName("custom-name")         // Custom task name
//
// ## Scheduled Task Options
//
//	WithTaskQueue("reports")            // Queue for scheduled task
//	WithTaskPriority(PriorityHigh)      // Priority for scheduled task
//	WithTaskMaxRetries(3)               // Max retries for scheduled task
//
// # Error Handling
//
// The package defines comprehensive errors for different failure scenarios:
//
//	ErrRepositoryNil            // Repository is nil
//	ErrPayloadNil               // Payload is nil
//	ErrInvalidPriority          // Priority out of range (0-100)
//	ErrHandlerNotFound          // No handler for task type
//	ErrNoHandlers               // Worker has no handlers
//	ErrTaskAlreadyRegistered    // Duplicate scheduled task
//	ErrSchedulerNotConfigured   // Scheduler has no tasks
//	ErrNoTaskToClaim            // No tasks available (normal)
//
// # Advanced Usage Patterns
//
// ## Long-Running Tasks
//
// For tasks that may exceed the lock timeout, extend the lock periodically:
//
//	handler := queue.NewTaskHandler(func(ctx context.Context, data LongProcessPayload) error {
//		// Start a goroutine to extend lock every 2 minutes
//		ticker := time.NewTicker(2 * time.Minute)
//		defer ticker.Stop()
//
//		go func() {
//			for {
//				select {
//				case <-ctx.Done():
//					return
//				case <-ticker.C:
//					// Extend lock by 5 minutes
//					worker.ExtendLockForTask(ctx, taskID, 5*time.Minute)
//				}
//			}
//		}()
//
//		// Perform long-running work
//		return processLargeDataset(data)
//	})
//
// ## Conditional Task Creation
//
// Create tasks only when certain conditions are met:
//
//	if shouldSendReminder(user) {
//		enqueuer.Enqueue(ctx, ReminderPayload{UserID: user.ID},
//			queue.WithDelay(24*time.Hour),
//			queue.WithPriority(queue.PriorityLow),
//		)
//	}
//
// ## Task Monitoring and Metrics
//
// Monitor task processing with structured logging:
//
//	handler := queue.NewTaskHandler(func(ctx context.Context, data ProcessPayload) error {
//		start := time.Now()
//		defer func() {
//			logger.InfoContext(ctx, "task processed",
//				slog.String("task_type", "ProcessPayload"),
//				slog.Duration("duration", time.Since(start)),
//			)
//		}()
//
//		return processData(data)
//	})
//
// ## Batch Task Processing
//
// Process multiple related items efficiently:
//
//	// Enqueue batch of related tasks
//	batchID := uuid.New().String()
//	for _, item := range items {
//		enqueuer.Enqueue(ctx, BatchProcessPayload{
//			BatchID: batchID,
//			Item:    item,
//		}, queue.WithQueue("batch"))
//	}
//
//	// Handler tracks batch completion
//	batchHandler := queue.NewTaskHandler(func(ctx context.Context, data BatchProcessPayload) error {
//		err := processItem(data.Item)
//		if err != nil {
//			return err
//		}
//
//		// Check if batch is complete
//		if isBatchComplete(data.BatchID) {
//			// Trigger batch completion task
//			enqueuer.Enqueue(ctx, BatchCompletePayload{BatchID: data.BatchID})
//		}
//		return nil
//	})
package queue
