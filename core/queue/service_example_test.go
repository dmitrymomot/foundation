package queue_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/dmitrymomot/foundation/core/queue"
)

// Example_service demonstrates how to use the unified Service for queue management
func Example_service() {
	// Create storage (in-memory for example, use your database implementation in production)
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	// Create service from configuration
	cfg := queue.DefaultConfig()
	service, err := queue.NewServiceFromConfig(cfg, storage,
		queue.WithServiceLogger(slog.Default()),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Define task payload types
	type EmailTask struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	type ReportTask struct {
		Type   string    `json:"type"`
		UserID string    `json:"user_id"`
		Date   time.Time `json:"date"`
	}

	// Register task handlers
	emailHandler := queue.NewTaskHandler(func(ctx context.Context, task EmailTask) error {
		fmt.Printf("Sending email to %s: %s\n", task.To, task.Subject)
		// Implement email sending logic here
		return nil
	})

	reportHandler := queue.NewTaskHandler(func(ctx context.Context, task ReportTask) error {
		fmt.Printf("Generating %s report for user %s\n", task.Type, task.UserID)
		// Implement report generation logic here
		return nil
	})

	// Register handlers with the service
	if err := service.RegisterHandlers(emailHandler, reportHandler); err != nil {
		log.Fatal(err)
	}

	// Register a periodic task for daily reports
	dailyReportHandler := queue.NewPeriodicTaskHandler("daily_report", func(ctx context.Context) error {
		fmt.Println("Running daily report generation")
		// Enqueue individual report tasks
		return service.Enqueue(ctx, ReportTask{
			Type:   "daily",
			UserID: "all",
			Date:   time.Now(),
		})
	})

	if err := service.RegisterHandler(dailyReportHandler); err != nil {
		log.Fatal(err)
	}

	// Schedule the daily report (in production, use DailyAt)
	if err := service.AddScheduledTask("daily_report",
		queue.EveryHours(24),
		queue.WithTaskQueue("reports"),
		queue.WithTaskPriority(queue.PriorityHigh),
	); err != nil {
		log.Fatal(err)
	}

	// Start the service in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		if err := service.Run(ctx); err != nil && err != context.DeadlineExceeded {
			log.Printf("Service error: %v", err)
		}
	}()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Enqueue some tasks
	service.Enqueue(context.Background(), EmailTask{
		To:      "user@example.com",
		Subject: "Welcome!",
		Body:    "Welcome to our service",
	})

	// Enqueue with delay
	service.EnqueueWithDelay(context.Background(), EmailTask{
		To:      "admin@example.com",
		Subject: "Reminder",
		Body:    "This is a delayed reminder",
	}, 2*time.Second)

	// Enqueue at specific time
	service.EnqueueAt(context.Background(), ReportTask{
		Type:   "weekly",
		UserID: "user123",
		Date:   time.Now(),
	}, time.Now().Add(1*time.Second))

	// Wait for demonstration
	time.Sleep(3 * time.Second)

	// Output:
	// Sending email to user@example.com: Welcome!
	// Generating weekly report for user user123
	// Sending email to admin@example.com: Reminder
}

// Example_serviceWithCustomStorage demonstrates using the Service with a custom storage implementation
func Example_serviceWithCustomStorage() {
	// In production, you would implement the Storage interface for your database
	// For example, a PostgreSQL implementation:
	//
	// type PostgresStorage struct {
	//     db *sql.DB
	// }
	//
	// func (s *PostgresStorage) CreateTask(ctx context.Context, task *queue.Task) error {
	//     // Implementation
	// }
	//
	// func (s *PostgresStorage) ClaimTask(...) (*queue.Task, error) {
	//     // Implementation
	// }
	// ... other interface methods

	// For this example, we'll use the in-memory storage
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	// Create service with custom options
	service, err := queue.NewService(storage,
		// Configure worker
		queue.WithWorkerOptions(
			queue.WithMaxConcurrentTasks(20),
			queue.WithPullInterval(100*time.Millisecond),
			queue.WithQueues("critical", "default", "low-priority"),
		),
		// Configure scheduler
		queue.WithSchedulerOptions(
			queue.WithCheckInterval(30*time.Second),
		),
		// Configure enqueuer
		queue.WithEnqueuerOptions(
			queue.WithDefaultQueue("default"),
			queue.WithDefaultPriority(queue.PriorityMedium),
		),
		// Service-level configuration
		queue.WithServiceLogger(slog.Default()),
		queue.WithBeforeStart(func(ctx context.Context) error {
			fmt.Println("Service starting...")
			return nil
		}),
		queue.WithAfterStop(func() error {
			fmt.Println("Service stopped")
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use the service...
	_ = service

	// Output:
}

// Example_microserviceIntegration shows how to integrate the Service into a microservice
func Example_microserviceIntegration() {
	// This example shows a typical microservice setup

	// 1. Initialize storage (shared across your application)
	storage := initializeStorage()
	defer storage.Close()

	// 2. Create queue service
	queueService := initializeQueueService(storage)

	// 3. Register all task handlers
	registerTaskHandlers(queueService)

	// 4. Set up scheduled tasks
	setupScheduledTasks(queueService)

	// 5. Start the service as part of your application lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run in error group with other services
	go func() {
		if err := queueService.Run(ctx); err != nil {
			log.Printf("Queue service error: %v", err)
		}
	}()

	// Your application continues running...
	// The queue service processes tasks in the background

	// 6. Use the enqueuer throughout your application
	enqueuer := queueService.Enqueuer()
	_ = enqueuer // Use this to enqueue tasks from anywhere in your app

	// Output:
}

// Helper functions for the microservice example
func initializeStorage() *queue.MemoryStorage {
	// In production, initialize your database-backed storage
	return queue.NewMemoryStorage()
}

func initializeQueueService(storage queue.Storage) *queue.Service {
	cfg := queue.DefaultConfig()
	// Override with environment variables or config file
	cfg.MaxConcurrentTasks = 50
	cfg.Queues = []string{"critical", "default", "batch", "reports"}

	service, err := queue.NewServiceFromConfig(cfg, storage)
	if err != nil {
		log.Fatal(err)
	}
	return service
}

func registerTaskHandlers(service *queue.Service) {
	// Register all your application's task handlers
	// These would typically be in separate packages/modules

	// Example handlers (implement your actual business logic)
	type ProcessPayment struct {
		OrderID string `json:"order_id"`
		Amount  int64  `json:"amount"`
	}

	paymentHandler := queue.NewTaskHandler(func(ctx context.Context, task ProcessPayment) error {
		// Process payment logic
		return nil
	})

	service.RegisterHandler(paymentHandler)
	// Register more handlers...
}

func setupScheduledTasks(service *queue.Service) {
	// Set up all periodic/scheduled tasks

	// Cleanup old data every hour
	cleanupHandler := queue.NewPeriodicTaskHandler("cleanup_old_data", func(ctx context.Context) error {
		// Cleanup logic
		return nil
	})

	service.RegisterHandler(cleanupHandler)
	service.AddScheduledTask("cleanup_old_data",
		queue.EveryHours(1),
		queue.WithTaskQueue("batch"),
	)

	// Generate daily reports at 9 AM
	reportHandler := queue.NewPeriodicTaskHandler("daily_reports", func(ctx context.Context) error {
		// Report generation logic
		return nil
	})

	service.RegisterHandler(reportHandler)
	service.AddScheduledTask("daily_reports",
		queue.DailyAt(9, 0), // 9:00 AM
		queue.WithTaskQueue("reports"),
		queue.WithTaskPriority(queue.PriorityHigh),
	)
}
