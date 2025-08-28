// Package queue provides a comprehensive job queue system with workers, scheduling,
// and priority-based task processing. It supports both immediate task execution
// and scheduled task processing with configurable retry mechanisms and error handling.
//
// # Features
//
//   - Task enqueueing with priority support
//   - Background workers with concurrent processing
//   - Scheduled task execution with flexible scheduling options
//   - Configurable retry policies with exponential backoff
//   - In-memory storage for testing and development
//   - Extensible repository interface for custom storage backends
//   - Graceful shutdown with proper cleanup
//   - Type-safe task handlers using Go generics
//   - Dead letter queue for failed tasks
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
//	// Start scheduler
//	go scheduler.Start(ctx)
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
//		// Start components in goroutines
//		go worker.Start(ctx)
//		go scheduler.Start(ctx)
//
//		// Wait for shutdown signal
//		<-ctx.Done()
//
//		// Workers stop automatically when context is cancelled
//		// Call Stop() for immediate shutdown
//		worker.Stop()
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
//   - EnqueuerRepository: CreateTask for task creation
//   - WorkerRepository: ClaimTask, CompleteTask, FailTask, MoveToDLQ, ExtendLock
//   - SchedulerRepository: CreateTask, GetPendingTaskByName
//
// Use queue.NewMemoryStorage() for development or implement custom storage for production.
package queue
