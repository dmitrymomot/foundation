// Package queue provides a job queue system for background task processing with
// workers, scheduling, and priority-based execution. It supports both immediate
// and scheduled task processing with configurable retry mechanisms.
//
// The package is built around three components:
//   - Enqueuer: Submits tasks to queues
//   - Worker: Processes tasks with concurrent execution
//   - Scheduler: Creates periodic tasks on flexible schedules
//
// # Features
//
//   - Priority-based task processing (0-100 scale)
//   - Concurrent worker execution
//   - Scheduled/periodic task support
//   - Automatic retries with exponential backoff (max 10 retries)
//   - In-memory storage (development) and extensible storage interface
//   - Graceful shutdown with Run() and Stop() methods
//   - Type-safe handlers using Go generics
//   - Dead letter queue for failed tasks
//   - Multiple queue support
//   - Task locking to prevent duplicate processing
//
// # Quick Start
//
// Complete working example with all components:
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"os/signal"
//		"syscall"
//		"time"
//
//		"github.com/dmitrymomot/foundation/core/queue"
//		"golang.org/x/sync/errgroup"
//	)
//
//	type EmailPayload struct {
//		To      string `json:"to"`
//		Subject string `json:"subject"`
//		Body    string `json:"body"`
//	}
//
//	func main() {
//		// Setup context with signal handling
//		ctx, stop := signal.NotifyContext(context.Background(),
//			syscall.SIGINT, syscall.SIGTERM)
//		defer stop()
//
//		// Create storage (in-memory for development)
//		storage := queue.NewMemoryStorage()
//
//		// Create components
//		enqueuer, _ := queue.NewEnqueuer(storage,
//			queue.WithDefaultQueue("email"))
//		worker, _ := queue.NewWorker(storage,
//			queue.WithQueues("email"),
//			queue.WithMaxConcurrentTasks(5))
//		scheduler, _ := queue.NewScheduler(storage)
//
//		// Register handlers
//		emailHandler := queue.NewTaskHandler(func(ctx context.Context, email EmailPayload) error {
//			log.Printf("Sending email to %s: %s", email.To, email.Subject)
//			return nil
//		})
//		worker.RegisterHandler(emailHandler)
//
//		reportHandler := queue.NewPeriodicTaskHandler("daily_report", func(ctx context.Context) error {
//			log.Println("Generating daily report")
//			return nil
//		})
//		worker.RegisterHandler(reportHandler)
//
//		// Schedule tasks
//		scheduler.AddTask("daily_report", queue.DailyAt(9, 0))
//
//		// Start all components with errgroup
//		g, ctx := errgroup.WithContext(ctx)
//		g.Go(storage.Run(ctx))
//		g.Go(worker.Run(ctx))
//		g.Go(scheduler.Run(ctx))
//
//		// Enqueue a task
//		enqueuer.Enqueue(context.Background(), EmailPayload{
//			To:      "user@example.com",
//			Subject: "Welcome!",
//			Body:    "Welcome to our service!",
//		})
//
//		// Wait for shutdown
//		if err := g.Wait(); err != nil {
//			log.Fatal(err)
//		}
//	}
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
//	// Start worker in goroutine
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
// Create recurring tasks with flexible scheduling:
//
//	scheduler, _ := queue.NewScheduler(storage)
//
//	// Register periodic handler
//	reportHandler := queue.NewPeriodicTaskHandler("daily_report", func(ctx context.Context) error {
//		return generateDailyReport()
//	})
//	worker.RegisterHandler(reportHandler)
//
//	// Schedule tasks
//	scheduler.AddTask("daily_report", queue.DailyAt(9, 0))
//	scheduler.AddTask("weekly_cleanup", queue.WeeklyOn(time.Monday, 2, 0))
//	scheduler.AddTask("health_check", queue.EveryMinutes(5))
//
//	// Start scheduler
//	go scheduler.Start(ctx)
//
// # Retry Mechanisms
//
// Failed tasks automatically retry with exponential backoff:
//
//	// Set max retries (default is 3, max is 10)
//	enqueuer.Enqueue(ctx, payload, queue.WithMaxRetries(5))
//
//	// Returning an error triggers automatic retry
//	handler := queue.NewTaskHandler(func(ctx context.Context, data ProcessingPayload) error {
//		if err := performOperation(data); err != nil {
//			return err // Will retry with exponential backoff
//		}
//		return nil
//	})
//
//	// Tasks exceeding max retries move to dead letter queue
//
// # Multiple Queues
//
// Use separate queues for different workload types:
//
//	// Email worker (high throughput)
//	emailWorker, _ := queue.NewWorker(storage,
//		queue.WithQueues("email"),
//		queue.WithMaxConcurrentTasks(10),
//	)
//
//	// Image worker (CPU intensive)
//	imageWorker, _ := queue.NewWorker(storage,
//		queue.WithQueues("images"),
//		queue.WithMaxConcurrentTasks(3),
//	)
//
//	// Register handlers and start
//	emailWorker.RegisterHandler(emailHandler)
//	imageWorker.RegisterHandler(imageHandler)
//	go emailWorker.Start(ctx)
//	go imageWorker.Start(ctx)
//
// # Error Handling
//
// Handle errors and context cancellation properly:
//
//	handler := queue.NewTaskHandler(func(ctx context.Context, data ProcessingPayload) error {
//		// Respect context cancellation
//		if ctx.Err() != nil {
//			return ctx.Err()
//		}
//
//		// Process task
//		if err := processData(data); err != nil {
//			return fmt.Errorf("processing failed: %w", err)
//		}
//		return nil
//	})
//
// # Graceful Shutdown
//
// Two patterns for lifecycle management:
//
//	// Pattern 1: Using errgroup (recommended)
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(storage.Run(ctx))
//	g.Go(worker.Run(ctx))
//	g.Go(scheduler.Run(ctx))
//	if err := g.Wait(); err != nil {
//		log.Fatal(err)
//	}
//
//	// Pattern 2: Manual Start/Stop
//	go worker.Start(ctx)
//	go scheduler.Start(ctx)
//
//	// On shutdown signal
//	cancel() // Cancel context
//	worker.Stop() // Blocks until shutdown complete (default 30s timeout)
//	scheduler.Stop()
//
//	// Configure shutdown timeout
//	worker, _ := queue.NewWorker(storage,
//		queue.WithShutdownTimeout(60*time.Second),
//	)
//
// # Storage Interfaces
//
// Implement custom storage by satisfying the repository interfaces:
//
//	// Required for Enqueuer
//	type EnqueuerRepository interface {
//		CreateTask(ctx context.Context, task *Task) error
//	}
//
//	// Required for Worker
//	type WorkerRepository interface {
//		ClaimTask(ctx context.Context, workerID uuid.UUID, queues []string, lockDuration time.Duration) (*Task, error)
//		CompleteTask(ctx context.Context, taskID uuid.UUID) error
//		FailTask(ctx context.Context, taskID uuid.UUID, errorMsg string) error
//		MoveToDLQ(ctx context.Context, taskID uuid.UUID) error
//		ExtendLock(ctx context.Context, taskID uuid.UUID, duration time.Duration) error
//	}
//
//	// Required for Scheduler
//	type SchedulerRepository interface {
//		CreateTask(ctx context.Context, task *Task) error
//		GetPendingTaskByName(ctx context.Context, taskName string) (*Task, error)
//	}
//
//	// Use queue.NewMemoryStorage() for development
//	// Implement your own for production (e.g., PostgreSQL, Redis)
//
// # Delayed Tasks
//
// Schedule tasks for future execution:
//
//	enqueuer.Enqueue(ctx, payload, queue.WithDelay(time.Hour))
//	enqueuer.Enqueue(ctx, payload, queue.WithScheduledAt(futureTime))
//
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
// Common configuration options:
//
//	// Enqueuer options
//	enqueuer, _ := queue.NewEnqueuer(storage,
//		queue.WithDefaultQueue("email"),
//		queue.WithDefaultPriority(queue.PriorityHigh),
//	)
//
//	// Worker options
//	worker, _ := queue.NewWorker(storage,
//		queue.WithQueues("email", "sms"),
//		queue.WithMaxConcurrentTasks(10),
//		queue.WithPullInterval(5*time.Second),
//		queue.WithLockTimeout(5*time.Minute),
//		queue.WithShutdownTimeout(60*time.Second),
//	)
//
//	// Scheduler options
//	scheduler, _ := queue.NewScheduler(storage,
//		queue.WithCheckInterval(30*time.Second),
//		queue.WithSchedulerShutdownTimeout(60*time.Second),
//	)
//
//	// Task options
//	enqueuer.Enqueue(ctx, payload,
//		queue.WithQueue("priority"),
//		queue.WithPriority(queue.PriorityMax),
//		queue.WithMaxRetries(5),
//		queue.WithDelay(time.Hour),
//	)
//
// # Observability
//
// Components provide Stats() for monitoring and Healthcheck() for readiness:
//
//	// Get statistics
//	stats := worker.Stats()
//	fmt.Printf("Processed: %d, Failed: %d, Active: %d\n",
//		stats.TasksProcessed, stats.TasksFailed, stats.ActiveTasks)
//
//	// Health check
//	if err := worker.Healthcheck(ctx); err != nil {
//		log.Printf("unhealthy: %v", err)
//	}
package queue
