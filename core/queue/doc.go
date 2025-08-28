// Package queue provides a comprehensive job queue system with workers, scheduling,
// and priority-based task processing. It supports both immediate task execution
// and scheduled task processing with configurable retry mechanisms and error handling.
//
// # Features
//
//   - Task enqueueing with priority support
//   - Background workers with concurrent processing
//   - Scheduled task execution with cron-like scheduling
//   - Configurable retry policies with exponential backoff
//   - In-memory storage for testing and development
//   - Extensible repository interface for custom storage backends
//   - Graceful shutdown with proper cleanup
//   - Comprehensive error handling and logging
//
// # Basic Usage
//
// Create a queue system with enqueuer, worker, and scheduler:
//
//	import "github.com/dmitrymomot/foundation/core/queue"
//
//	// Create storage (in-memory for development)
//	storage := queue.NewMemoryStorage()
//
//	// Create enqueuer for adding tasks
//	enqueuer, err := queue.NewEnqueuer(storage,
//		queue.WithDefaultQueue("email_queue"),
//		queue.WithDefaultPriority(queue.PriorityHigh),
//	)
//
//	// Create worker for processing tasks
//	worker, err := queue.NewWorker(storage,
//		queue.WithWorkerQueue("email_queue"),
//		queue.WithConcurrency(5),
//		queue.WithRetryPolicy(3, time.Second*5),
//	)
//
//	// Register task handler
//	worker.RegisterHandler("send_email", handleEmailTask)
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
//	}, queue.WithTaskType("send_email"))
//
// # Task Types and Handlers
//
// Define custom task types and their handlers:
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
//	// Task handler function
//	func handleEmailTask(ctx context.Context, task *queue.Task) error {
//		var payload EmailPayload
//		if err := json.Unmarshal(task.Payload, &payload); err != nil {
//			return fmt.Errorf("invalid email payload: %w", err)
//		}
//
//		// Send email
//		return emailService.Send(payload.To, payload.Subject, payload.Body)
//	}
//
//	func handleImageProcess(ctx context.Context, task *queue.Task) error {
//		var payload ImageProcessPayload
//		if err := json.Unmarshal(task.Payload, &payload); err != nil {
//			return err
//		}
//
//		// Process image
//		return imageProcessor.Resize(payload.ImageURL, payload.Width, payload.Height)
//	}
//
// # Priority-Based Processing
//
// Use different priority levels for task processing:
//
//	// High priority tasks (processed first)
//	enqueuer.Enqueue(ctx, CriticalPayload{...},
//		queue.WithTaskType("critical_operation"),
//		queue.WithPriority(queue.PriorityCritical),
//	)
//
//	// Normal priority tasks
//	enqueuer.Enqueue(ctx, StandardPayload{...},
//		queue.WithTaskType("standard_operation"),
//		queue.WithPriority(queue.PriorityDefault),
//	)
//
//	// Low priority tasks (processed last)
//	enqueuer.Enqueue(ctx, CleanupPayload{...},
//		queue.WithTaskType("cleanup"),
//		queue.WithPriority(queue.PriorityLow),
//	)
//
// # Scheduled Tasks
//
// Create scheduled tasks with cron-like syntax:
//
//	// Create scheduler
//	scheduler, err := queue.NewScheduler(storage,
//		queue.WithSchedulerLogger(logger),
//	)
//
//	// Schedule daily email reports
//	scheduler.Schedule("daily_report",
//		queue.NewCronSchedule("0 9 * * *"), // 9 AM daily
//		ReportPayload{Type: "daily"},
//		queue.WithSchedulerTaskType("generate_report"),
//		queue.WithSchedulerPriority(queue.PriorityHigh),
//	)
//
//	// Schedule weekly cleanup
//	scheduler.Schedule("weekly_cleanup",
//		queue.NewCronSchedule("0 2 * * 1"), // 2 AM on Mondays
//		CleanupPayload{Type: "weekly"},
//		queue.WithSchedulerTaskType("cleanup"),
//	)
//
//	// Start scheduler
//	go scheduler.Start(ctx)
//
// # Retry Mechanisms
//
// Configure retry policies for failed tasks:
//
//	worker, err := queue.NewWorker(storage,
//		queue.WithWorkerQueue("processing_queue"),
//		queue.WithRetryPolicy(5, time.Minute*2), // 5 retries, 2 min interval
//	)
//
//	// Handle task with retry logic
//	func handleRetryableTask(ctx context.Context, task *queue.Task) error {
//		// Attempt operation
//		err := performOperation()
//		if err != nil {
//			// Log retry attempt
//			log.Warn("Task failed, will retry",
//				"task_id", task.ID,
//				"attempt", task.AttemptCount+1,
//				"error", err,
//			)
//			return err // Will be retried automatically
//		}
//		return nil
//	}
//
// # Multiple Queues
//
// Set up different queues for different task types:
//
//	// Email queue worker
//	emailWorker, _ := queue.NewWorker(storage,
//		queue.WithWorkerQueue("email"),
//		queue.WithConcurrency(10),
//	)
//	emailWorker.RegisterHandler("send_welcome", handleWelcomeEmail)
//	emailWorker.RegisterHandler("send_notification", handleNotification)
//
//	// Image processing queue worker
//	imageWorker, _ := queue.NewWorker(storage,
//		queue.WithWorkerQueue("images"),
//		queue.WithConcurrency(3), // CPU intensive, fewer workers
//	)
//	imageWorker.RegisterHandler("resize", handleImageResize)
//	imageWorker.RegisterHandler("thumbnail", handleThumbnail)
//
//	// Analytics queue worker
//	analyticsWorker, _ := queue.NewWorker(storage,
//		queue.WithWorkerQueue("analytics"),
//		queue.WithConcurrency(2),
//	)
//	analyticsWorker.RegisterHandler("track_event", handleEventTracking)
//
//	// Start all workers
//	go emailWorker.Start(ctx)
//	go imageWorker.Start(ctx)
//	go analyticsWorker.Start(ctx)
//
// # Error Handling
//
// Implement comprehensive error handling:
//
//	func handleTaskWithErrorHandling(ctx context.Context, task *queue.Task) error {
//		defer func() {
//			if r := recover(); r != nil {
//				log.Error("Task panicked",
//					"task_id", task.ID,
//					"task_type", task.Type,
//					"panic", r,
//				)
//			}
//		}()
//
//		// Validate task
//		if task.Payload == nil {
//			return fmt.Errorf("task payload is nil")
//		}
//
//		// Check context cancellation
//		select {
//		case <-ctx.Done():
//			return fmt.Errorf("task cancelled: %w", ctx.Err())
//		default:
//		}
//
//		// Process task
//		if err := processTask(task); err != nil {
//			// Log error with context
//			log.Error("Task processing failed",
//				"task_id", task.ID,
//				"task_type", task.Type,
//				"error", err,
//			)
//			return err
//		}
//
//		log.Info("Task completed successfully",
//			"task_id", task.ID,
//			"task_type", task.Type,
//		)
//		return nil
//	}
//
// # Graceful Shutdown
//
// Implement proper shutdown procedures:
//
//	func runQueueSystem(ctx context.Context) error {
//		// Create components
//		storage := queue.NewMemoryStorage()
//		worker, _ := queue.NewWorker(storage)
//		scheduler, _ := queue.NewScheduler(storage)
//
//		// Start components
//		workerCtx, workerCancel := context.WithCancel(ctx)
//		schedulerCtx, schedulerCancel := context.WithCancel(ctx)
//
//		go worker.Start(workerCtx)
//		go scheduler.Start(schedulerCtx)
//
//		// Wait for shutdown signal
//		<-ctx.Done()
//
//		// Graceful shutdown
//		log.Info("Shutting down queue system...")
//
//		// Stop accepting new tasks
//		workerCancel()
//		schedulerCancel()
//
//		// Wait for current tasks to complete (with timeout)
//		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		worker.Shutdown(shutdownCtx)
//		scheduler.Stop()
//
//		log.Info("Queue system shutdown complete")
//		return nil
//	}
//
// # Custom Storage Backend
//
// Implement custom storage for production use:
//
//	type PostgreSQLStorage struct {
//		db *sql.DB
//	}
//
//	func (s *PostgreSQLStorage) CreateTask(ctx context.Context, task *queue.Task) error {
//		query := `INSERT INTO tasks (id, queue, type, priority, payload, created_at)
//		         VALUES ($1, $2, $3, $4, $5, $6)`
//		_, err := s.db.ExecContext(ctx, query,
//			task.ID, task.Queue, task.Type, task.Priority, task.Payload, task.CreatedAt)
//		return err
//	}
//
//	func (s *PostgreSQLStorage) GetNextTask(ctx context.Context, queue string) (*queue.Task, error) {
//		query := `SELECT id, queue, type, priority, payload, created_at, attempt_count
//		         FROM tasks WHERE queue = $1 AND status = 'pending'
//		         ORDER BY priority DESC, created_at ASC LIMIT 1`
//		// Implementation...
//		return task, nil
//	}
//
//	// Use custom storage
//	storage := &PostgreSQLStorage{db: database}
//	worker, _ := queue.NewWorker(storage)
//
// # Monitoring and Metrics
//
// Add monitoring to your queue system:
//
//	type MetricsHandler struct {
//		processed   int64
//		failed      int64
//		retries     int64
//		mu          sync.Mutex
//	}
//
//	func (m *MetricsHandler) WrapHandler(taskType string, handler queue.TaskHandler) queue.TaskHandler {
//		return func(ctx context.Context, task *queue.Task) error {
//			start := time.Now()
//			err := handler(ctx, task)
//			duration := time.Since(start)
//
//			m.mu.Lock()
//			if err != nil {
//				m.failed++
//				if task.AttemptCount > 0 {
//					m.retries++
//				}
//			} else {
//				m.processed++
//			}
//			m.mu.Unlock()
//
//			// Record metrics
//			recordTaskMetrics(taskType, duration, err == nil)
//			return err
//		}
//	}
//
// # Best Practices
//
//   - Use appropriate queue names for different task types
//   - Set concurrency based on task characteristics (CPU vs I/O bound)
//   - Implement proper error handling and logging in task handlers
//   - Use priority levels to ensure critical tasks are processed first
//   - Configure retry policies based on task failure patterns
//   - Implement graceful shutdown to avoid losing in-progress tasks
//   - Monitor queue depth and processing metrics
//   - Use scheduled tasks for recurring operations
//   - Keep task payloads small and serialize efficiently
//   - Handle context cancellation properly in long-running tasks
package queue
