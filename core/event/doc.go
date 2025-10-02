// Package event provides a type-safe, extensible event processing system for building
// event-driven architectures. It supports in-memory and external event buses with
// concurrent handler execution, graceful shutdown, and comprehensive observability.
//
// # Core Components
//
// Event represents a domain event with metadata (ID, Name, Payload, CreatedAt).
// Events are automatically assigned UUIDs and timestamps upon creation.
//
// Handler processes events through a type-safe interface. Handlers can be created
// from functions with automatic type inference using NewHandlerFunc, or with explicit
// event names using NewHandler.
//
// Processor manages event handlers and coordinates concurrent event processing.
// It provides graceful shutdown, health checks, observability metrics, and configurable
// concurrency controls.
//
// Publisher publishes events to an event bus. It automatically marshals events to JSON
// and handles logging of publishing operations.
//
// ChannelBus provides a simple in-memory event bus using Go channels, suitable for
// single-instance monolithic applications. It implements both eventBus (for publishing)
// and eventSource (for consuming) interfaces.
//
// Decorator enables middleware-style wrapping of handler functions for cross-cutting
// concerns like logging, metrics, retries, and timeouts.
//
// # Basic Usage
//
// Create an event type, set up handlers, and process events:
//
//	import (
//		"context"
//		"log/slog"
//		"os"
//
//		"github.com/dmitrymomot/foundation/core/event"
//	)
//
//	type UserCreated struct {
//		UserID string
//		Email  string
//	}
//
//	func main() {
//		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//
//		// Create in-memory event bus
//		bus := event.NewChannelBus(
//			event.WithBufferSize(100),
//			event.WithChannelLogger(logger),
//		)
//		defer bus.Close()
//
//		// Create publisher
//		publisher := event.NewPublisher(bus, event.WithPublisherLogger(logger))
//
//		// Create handler with automatic type inference
//		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//			logger.Info("user created", "user_id", evt.UserID, "email", evt.Email)
//			return nil
//		})
//
//		// Create processor
//		processor := event.NewProcessor(
//			event.WithEventSource(bus),
//			event.WithHandler(handler),
//			event.WithProcessorLogger(logger),
//		)
//
//		// Start processor in background
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//
//		go func() {
//			if err := processor.Start(ctx); err != nil {
//				logger.Error("processor failed", "error", err)
//			}
//		}()
//
//		// Publish events
//		publisher.Publish(ctx, UserCreated{UserID: "123", Email: "user@example.com"})
//
//		// Graceful shutdown
//		cancel()
//		if err := processor.Stop(); err != nil {
//			logger.Error("shutdown failed", "error", err)
//		}
//	}
//
// # Multiple Handlers for Same Event
//
// Register multiple handlers for the same event type. All handlers execute concurrently:
//
//	emailHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//		return sendWelcomeEmail(ctx, evt.Email)
//	})
//
//	analyticsHandler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//		return trackUserSignup(ctx, evt.UserID)
//	})
//
//	processor := event.NewProcessor(
//		event.WithEventSource(bus),
//		event.WithHandler(emailHandler, analyticsHandler),
//	)
//
// # Handler Decorators
//
// Apply cross-cutting concerns using decorators. Decorators execute in order (first = outermost):
//
//	func LoggingDecorator[T any](fn event.HandlerFunc[T]) event.HandlerFunc[T] {
//		return func(ctx context.Context, payload T) error {
//			logger.Info("processing event", "type", fmt.Sprintf("%T", payload))
//			err := fn(ctx, payload)
//			if err != nil {
//				logger.Error("event processing failed", "error", err)
//			}
//			return err
//		}
//	}
//
//	func MetricsDecorator[T any](fn event.HandlerFunc[T]) event.HandlerFunc[T] {
//		return func(ctx context.Context, payload T) error {
//			start := time.Now()
//			err := fn(ctx, payload)
//			duration := time.Since(start)
//			metrics.RecordEventDuration(fmt.Sprintf("%T", payload), duration)
//			if err != nil {
//				metrics.IncrementEventErrors(fmt.Sprintf("%T", payload))
//			}
//			return err
//		}
//	}
//
//	handler := event.NewHandlerFunc(
//		event.ApplyDecorators(
//			func(ctx context.Context, evt UserCreated) error {
//				return processUser(ctx, evt)
//			},
//			LoggingDecorator[UserCreated],
//			MetricsDecorator[UserCreated],
//			event.WithTimeout[UserCreated](5*time.Second),
//		),
//	)
//
// # Context Metadata
//
// Event metadata is automatically attached to handler contexts for observability:
//
//	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
//		eventID := event.EventID(ctx)
//		eventName := event.EventName(ctx)
//		eventTime := event.EventTime(ctx)
//		processingStart := event.StartProcessingTime(ctx)
//
//		logger.Info("processing event",
//			"event_id", eventID,
//			"event_name", eventName,
//			"created_at", eventTime,
//			"processing_started_at", processingStart)
//
//		return processUser(ctx, evt)
//	})
//
// # Graceful Shutdown with errgroup
//
// Coordinate processor lifecycle with errgroup for clean shutdown:
//
//	import "golang.org/x/sync/errgroup"
//
//	g, ctx := errgroup.WithContext(context.Background())
//
//	// Start processor
//	g.Go(processor.Run(ctx))
//
//	// Start other services
//	g.Go(func() error {
//		return httpServer.ListenAndServe()
//	})
//
//	// Wait for all services or first error
//	if err := g.Wait(); err != nil {
//		logger.Error("service error", "error", err)
//	}
//
// # Fallback Handler
//
// Handle events with no registered handlers using a fallback:
//
//	processor := event.NewProcessor(
//		event.WithEventSource(bus),
//		event.WithHandler(userHandler),
//		event.WithFallbackHandler(func(ctx context.Context, evt event.Event) error {
//			logger.Warn("unhandled event",
//				"event_id", evt.ID,
//				"event_name", evt.Name,
//				"payload", evt.Payload)
//
//			// Forward to dead letter queue
//			return deadLetterQueue.Send(ctx, evt)
//		}),
//	)
//
// # Observability and Health Checks
//
// Monitor processor health and performance metrics:
//
//	// Get processor statistics
//	stats := processor.Stats()
//	logger.Info("processor stats",
//		"events_processed", stats.EventsProcessed,
//		"events_failed", stats.EventsFailed,
//		"active_events", stats.ActiveEvents,
//		"is_running", stats.IsRunning,
//		"last_activity", stats.LastActivityAt)
//
//	// Health check endpoint
//	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
//		if err := processor.Healthcheck(r.Context()); err != nil {
//			w.WriteHeader(http.StatusServiceUnavailable)
//			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
//			return
//		}
//		w.WriteHeader(http.StatusOK)
//		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
//	})
//
// # Concurrency Control
//
// Limit concurrent handler execution to prevent resource exhaustion:
//
//	processor := event.NewProcessor(
//		event.WithEventSource(bus),
//		event.WithHandler(handler),
//		event.WithMaxConcurrentHandlers(100), // Max 100 concurrent handlers
//		event.WithShutdownTimeout(30*time.Second),
//	)
//
// # Event Name Resolution
//
// Event names are automatically derived from payload types using reflection.
// For struct types, the type name is used directly (e.g., "UserCreated").
// For other types, Go's type string representation is used.
//
// To use custom event names, use NewHandler instead of NewHandlerFunc:
//
//	handler := event.NewHandler("user.created", func(ctx context.Context, payload any) error {
//		evt := payload.(UserCreated)
//		return processUser(ctx, evt)
//	})
//
// # Thread Safety
//
// All components are thread-safe and designed for concurrent use:
//
//   - Publisher can be called concurrently from multiple goroutines
//   - Processor safely manages concurrent handler execution
//   - ChannelBus handles concurrent publishers and respects context cancellation
//   - Stats and Healthcheck methods use atomic operations for safe concurrent access
//
// # Integration with External Event Systems
//
// To integrate with external event systems (e.g., Kafka, RabbitMQ, AWS SNS/SQS),
// implement the eventBus interface for publishing:
//
//	type eventBus interface {
//		Publish(ctx context.Context, data []byte) error
//	}
//
// And the eventSource interface for consuming:
//
//	type eventSource interface {
//		Events() <-chan []byte
//	}
//
// The processor expects events as JSON-marshaled Event structs with ID, Name,
// Payload, and CreatedAt fields.
package event
