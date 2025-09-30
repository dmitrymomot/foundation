// Package event provides a type-safe event bus implementation inspired by Watermill.
// It separates concerns into three components: Publisher (client), Transport (wire),
// and Processor (router/worker manager).
//
// Events represent facts/notifications with one-to-many handler relationships.
// Each event can have zero or more handlers (fan-out pattern).
//
// # Architecture (Watermill-Inspired)
//
// The package follows Watermill's architectural pattern:
//
//   - **Publisher** = Stateless client (like Watermill's Publisher)
//   - **Transport** = Passive wire (like Watermill's Subscriber - provides channel)
//   - **Processor** = Active router (like Watermill's Router - manages workers)
//
// This separation enables flexible deployment patterns:
//   - Local: Single process runs Publisher and Processor with shared transport
//   - Distributed: Publisher in web server, Processor in worker service
//   - Testing: Sync transport for deterministic, in-process execution
//
// # Core Concepts
//
// Events are fact-based notifications like UserCreated, OrderPlaced, PaymentProcessed.
// Each event type can have multiple handlers (fan-out). The package provides:
//
//   - Two execution strategies (Sync, Channel)
//   - Type-safe handlers via generics
//   - Immutable middleware configured at Processor construction
//   - Context-based lifecycle management
//   - Unified panic recovery across all transports
//   - Decorator chaining for retry, timeout, backoff
//   - Stats tracking for observability (async transports)
//   - Worker management in Processor (not Transport)
//
// # Events vs Commands
//
// Key semantic differences:
//
//   - **Events**: Notifications (happened) - UserCreated
//   - **Commands**: Orders (do this) - CreateUser
//   - **Events**: One-to-many (0+ handlers, fan-out)
//   - **Commands**: One-to-one (exactly 1 handler, competitive)
//   - **Events**: Missing handler is warning
//   - **Commands**: Missing handler is error
//
// # Quick Start: Sync Transport (Simple)
//
// Synchronous execution in the same process:
//
//	import "github.com/dmitrymomot/foundation/core/event"
//
//	type UserCreated struct {
//	    UserID string
//	    Email  string
//	}
//
//	func invalidateCacheHandler(ctx context.Context, evt UserCreated) error {
//	    return cache.Invalidate(ctx, evt.UserID)
//	}
//
//	func updateMetricsHandler(ctx context.Context, evt UserCreated) error {
//	    metrics.Inc("users.created")
//	    return nil
//	}
//
//	// Create transport (passive wire)
//	transport := event.NewSyncTransport()
//
//	// Create processor (active manager with handlers)
//	processor := event.NewProcessor(transport)
//	processor.Register(event.NewHandlerFunc(invalidateCacheHandler))
//	processor.Register(event.NewHandlerFunc(updateMetricsHandler))
//
//	// Start processor (blocks until context cancelled)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go processor.Run(ctx)
//
//	// For sync transport, processor can publish directly
//	err := processor.Publish(ctx, UserCreated{
//	    UserID: "123",
//	    Email:  "user@example.com",
//	})
//	if err != nil {
//	    // Aggregated handler errors via errors.Join()
//	    log.Fatal(err)
//	}
//
// # Quick Start: Channel Transport (Async)
//
// Asynchronous execution with background workers managed by Processor:
//
//	import (
//	    "github.com/dmitrymomot/foundation/core/event"
//	    "golang.org/x/sync/errgroup"
//	)
//
//	// Create transport (passive wire - just a channel)
//	transport := event.NewChannelTransport(100)
//
//	// Create processor (active manager - controls workers)
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithWorkers(5),  // Processor controls worker count!
//	    event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
//	        logger.Error("event handler failed", "event", evtName, "error", err)
//	    }),
//	    event.WithLogger(logger),
//	)
//	processor.Register(event.NewHandlerFunc(invalidateCacheHandler))
//	processor.Register(event.NewHandlerFunc(updateMetricsHandler))
//
//	// Start processor (manages worker lifecycle)
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(func() error {
//	    return processor.Run(ctx) // Blocks until context cancelled
//	})
//
//	// Create publisher (can be in different part of code)
//	publisher := event.NewPublisher(transport)
//
//	// Publish returns immediately
//	err := publisher.Publish(ctx, UserCreated{UserID: "123"})
//	if err != nil {
//	    // Only dispatch errors (ErrBufferFull, etc)
//	    // Handler errors reported via WithErrorHandler callback
//	    log.Fatal(err)
//	}
//
// # Deployment Patterns
//
// ## Pattern 1: Local (Single Process)
//
// Publisher and Processor in the same process:
//
//	transport := event.NewChannelTransport(100)
//
//	// Processor runs in background (manages workers)
//	processor := event.NewProcessor(transport, event.WithWorkers(5))
//	processor.Register(invalidateCacheHandler)
//	processor.Register(updateMetricsHandler)
//	go processor.Run(ctx)
//
//	// Publisher used in HTTP handlers
//	publisher := event.NewPublisher(transport)
//	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
//	    publisher.Publish(r.Context(), UserCreated{...})
//	    w.WriteHeader(http.StatusOK)
//	})
//
// ## Pattern 2: Publisher-Only (Web Server)
//
// Web server publishes to external queue/workers (future):
//
//	transport := redis.NewTransport("redis://localhost")
//	publisher := event.NewPublisher(transport)
//
//	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
//	    publisher.Publish(r.Context(), UserCreated{...})
//	    w.WriteHeader(http.StatusOK)
//	})
//
// ## Pattern 3: Processor-Only (Worker Service)
//
// Dedicated worker service processes events from queue (future):
//
//	transport := redis.NewTransport("redis://localhost")
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithWorkers(10),
//	)
//	processor.Register(handler1)
//	processor.Register(handler2)
//
//	// Blocks until shutdown signal
//	if err := processor.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// ## Pattern 4: Testing (Sync)
//
// Deterministic execution for tests:
//
//	func TestUserCreatedEvent(t *testing.T) {
//	    transport := event.NewSyncTransport()
//	    processor := event.NewProcessor(transport)
//	    processor.Register(event.NewHandlerFunc(invalidateCacheHandler))
//	    go processor.Run(context.Background())
//
//	    err := processor.Publish(ctx, UserCreated{UserID: "123"})
//	    require.NoError(t, err)
//
//	    // Assertions run immediately after (synchronous execution)
//	    assertCacheInvalidated(t, "123")
//	}
//
// # Transports
//
// ## Sync Transport
//
// Executes events immediately in the caller's goroutine.
// All handlers execute sequentially in FIFO registration order.
//
// Characteristics:
//   - Direct function call (no goroutines, no channels)
//   - Synchronous error handling (errors.Join)
//   - Runs in caller's context
//   - No worker management needed
//
// Use cases:
//   - Testing (deterministic execution)
//   - Simple applications
//   - Transaction boundaries
//
// Example:
//
//	transport := event.NewSyncTransport()
//	processor := event.NewProcessor(transport)
//	processor.Register(event.NewHandlerFunc(handler1))
//	processor.Register(event.NewHandlerFunc(handler2))
//	go processor.Run(ctx)
//
//	err := processor.Publish(ctx, UserCreated{...})
//	// Error contains all handler errors via errors.Join()
//
// ## Channel Transport
//
// Executes events asynchronously using buffered channels.
// Transport is a passive wire - Processor manages workers.
//
// Characteristics:
//   - Non-blocking publish
//   - Buffered channel (configurable size)
//   - Passive wire (no internal workers)
//   - Workers managed by Processor
//   - Graceful shutdown (drains channel)
//   - Error handling via callback
//
// Use cases:
//   - Fire-and-forget operations
//   - Decoupling (don't block HTTP response)
//   - Local background tasks
//   - Cache invalidation
//
// Example:
//
//	transport := event.NewChannelTransport(100)
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithWorkers(5),  // Processor controls workers
//	    event.WithErrorHandler(errorHandler),
//	)
//	processor.Register(handler1)
//	processor.Register(handler2)
//	go processor.Run(ctx)
//
//	publisher := event.NewPublisher(transport)
//	err := publisher.Publish(ctx, UserCreated{...})
//	// Error is only dispatch error (ErrBufferFull)
//	// Handler errors reported via errorHandler callback
//
// # Middleware
//
// Middleware wraps all handlers to add cross-cutting functionality.
//
// IMPORTANT: Middleware is immutable and must be configured at Processor construction
// using WithMiddleware(). It cannot be added or modified after creation.
//
// Built-in middleware:
//   - LoggingMiddleware: Logs event execution with timing
//
// Example:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithMiddleware(
//	        event.LoggingMiddleware(logger),
//	        metricsMiddleware,
//	        tracingMiddleware,
//	    ),
//	)
//
// Custom middleware:
//
//	func metricsMiddleware(next event.Handler) event.Handler {
//	    return &customHandler{
//	        name: next.Name(),
//	        fn: func(ctx context.Context, payload any) error {
//	            start := time.Now()
//	            err := next.Handle(ctx, payload)
//	            metrics.Observe(next.Name(), time.Since(start), err != nil)
//	            return err
//	        },
//	    }
//	}
//
// # Decorators
//
// Decorators wrap individual handlers to add retry, backoff, or timeout logic.
// Unlike middleware (applied to all handlers), decorators are applied per-handler.
//
// ## Using Decorator Chaining (Recommended)
//
// The Decorate() helper provides cleaner syntax:
//
//	handler := event.Decorate(
//	    event.NewHandlerFunc(notifyWebhookHandler),
//	    event.Retry(3),
//	    event.Backoff(5, 100*time.Millisecond, 10*time.Second),
//	    event.Timeout(60*time.Second),
//	)
//	processor.Register(handler)
//
// Available decorators:
//
//   - Retry(maxRetries): Retries on error up to maxRetries times
//   - Backoff(maxRetries, initialDelay, maxDelay): Exponential backoff retry
//   - Timeout(duration): Enforces maximum execution time
//
// Decorators are applied left-to-right (first decorator wraps innermost).
//
// # Error Handling
//
// Error handling differs between sync and async transports:
//
// ## Sync Transport
//
// Handler errors are collected and returned via errors.Join():
//
//	err := processor.Publish(ctx, evt)
//	if err != nil {
//	    // Could contain:
//	    // - Multiple handler errors (one per failed handler)
//	    // - Panic errors (caught and converted)
//	}
//
// **Handler isolation**: One handler's error doesn't stop other handlers.
// All handlers execute, all errors are collected.
//
// ## Channel Transport
//
// Publish returns only dispatch errors (ErrBufferFull, etc).
// Handler errors are reported via WithErrorHandler callback:
//
//	processor := event.NewProcessor(
//	    transport,
//	    event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
//	        logger.Error("event handler failed",
//	            "event", evtName,
//	            "error", err,
//	            "trace_id", ctx.Value("trace_id"),
//	        )
//	        metrics.EventFailed.Inc()
//	    }),
//	)
//
//	// Publish returns immediately
//	err := publisher.Publish(ctx, evt)
//	if err == event.ErrBufferFull {
//	    // Channel buffer is full
//	    return http.StatusServiceUnavailable
//	}
//
// # Panic Recovery
//
// All transports include unified panic recovery. If a handler panics, the panic
// is caught and converted to an error, preventing the entire process from crashing.
//
//	func riskyHandler(ctx context.Context, evt ProcessData) error {
//	    panic("something went wrong") // Caught by transport
//	}
//
// Sync transport: Returns panic as error to caller (via errors.Join)
// Channel transport: Reports panic via WithErrorHandler callback
//
// The panic message and stack trace are included in the error.
//
// # Graceful Shutdown
//
// Processor uses context-based lifecycle management for clean shutdown:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel() // Triggers shutdown
//
//	g, ctx := errgroup.WithContext(ctx)
//	g.Go(func() error {
//	    return processor.Run(ctx)
//	})
//
//	// When context is cancelled:
//	// 1. Channel transport: Closes channel, workers drain remaining events
//	// 2. Sync transport: Returns immediately (no workers)
//	// 3. All transports: Close() called for cleanup
//
// Signal-based shutdown:
//
//	import "os/signal"
//	import "syscall"
//
//	ctx, stop := signal.NotifyContext(
//	    context.Background(),
//	    syscall.SIGTERM,
//	    syscall.SIGINT,
//	)
//	defer stop()
//
//	g, ctx := errgroup.WithContext(ctx)
//	g.Go(func() error {
//	    return processor.Run(ctx)
//	})
//
//	if err := g.Wait(); err != nil {
//	    log.Printf("shutdown error: %v", err)
//	}
//
// # Stats and Observability
//
// Processor tracks statistics for observability:
//
//	stats := processor.Stats()
//	log.Printf("Received: %d, Processed: %d, Failed: %d",
//	    stats.Received, stats.Processed, stats.Failed)
//
// Stats are tracked via atomic counters and are thread-safe.
// Useful for monitoring worker health and throughput.
//
// # Best Practices
//
// 1. Event types should be self-contained with all needed data
// 2. Use sync transport for testing (deterministic, no timing issues)
// 3. Use sync transport for transactional operations (immediate errors)
// 4. Use channel transport for fire-and-forget operations
// 5. Always provide WithErrorHandler with async transports
// 6. Use errgroup for managing Processor lifecycle
// 7. Configure middleware at Processor construction time (immutable after creation)
// 8. Apply decorators at registration time, not inside handlers
// 9. Use middleware for cross-cutting concerns (logging, metrics, tracing)
// 10. Use decorators for per-handler concerns (retry, timeout, backoff)
// 11. Keep handlers simple and focused on business logic
// 12. Let panic recovery handle unexpected failures gracefully
// 13. Context is propagated to handlers - use it for cancellation/values
// 14. Make handlers idempotent - events may be delivered multiple times
// 15. Separate Publisher and Processor for distributed architectures
// 16. Share transport instance between Publisher and Processor in same process
// 17. Processor controls workers, not Transport (Watermill pattern)
// 18. Use commands for competitive work, events for broadcasting
//
// # Event → Command Pattern
//
// For operations that should happen only once (competitive work), publish a command:
//
//	// Event handler (runs on all instances)
//	func onUserCreated(ctx context.Context, evt UserCreated) error {
//	    // This runs on all processor instances (fan-out)
//	    // But command executes competitively (one instance only)
//	    return cmdDispatcher.Dispatch(ctx, SendWelcomeEmail{
//	        UserID: evt.UserID,
//	        Email:  evt.Email,
//	    })
//	}
//
//	// Event processor (all instances)
//	eventProcessor.Register(event.NewHandlerFunc(onUserCreated))
//
//	// Command processor (competitive)
//	commandProcessor.Register(command.NewHandlerFunc(sendEmailHandler))
//
// # Complete Example
//
// Full example with HTTP server and background workers:
//
//	package main
//
//	import (
//	    "context"
//	    "log"
//	    "net/http"
//	    "os/signal"
//	    "syscall"
//	    "time"
//
//	    "github.com/dmitrymomot/foundation/core/event"
//	    "golang.org/x/sync/errgroup"
//	)
//
//	type UserCreated struct {
//	    UserID string
//	    Email  string
//	}
//
//	func invalidateCacheHandler(ctx context.Context, evt UserCreated) error {
//	    log.Printf("Invalidating cache for user: %s", evt.UserID)
//	    return cache.Invalidate(ctx, evt.UserID)
//	}
//
//	func updateMetricsHandler(ctx context.Context, evt UserCreated) error {
//	    log.Printf("Updating metrics for user: %s", evt.UserID)
//	    return metrics.Inc("users.created")
//	}
//
//	func main() {
//	    // Setup graceful shutdown
//	    ctx, stop := signal.NotifyContext(
//	        context.Background(),
//	        syscall.SIGTERM,
//	        syscall.SIGINT,
//	    )
//	    defer stop()
//
//	    // Create transport (passive wire)
//	    transport := event.NewChannelTransport(100)
//
//	    // Create processor (active manager with workers)
//	    processor := event.NewProcessor(
//	        transport,
//	        event.WithWorkers(5),  // Processor controls workers
//	        event.WithMiddleware(event.LoggingMiddleware(logger)),
//	        event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
//	            log.Printf("Event %s failed: %v", evtName, err)
//	        }),
//	    )
//
//	    // Register handlers with decorators
//	    processor.Register(event.Decorate(
//	        event.NewHandlerFunc(invalidateCacheHandler),
//	        event.Retry(3),
//	        event.Timeout(5*time.Second),
//	    ))
//	    processor.Register(event.NewHandlerFunc(updateMetricsHandler))
//
//	    // Start processor
//	    g, ctx := errgroup.WithContext(ctx)
//	    g.Go(func() error {
//	        return processor.Run(ctx)
//	    })
//
//	    // Create publisher for HTTP handlers
//	    publisher := event.NewPublisher(transport)
//
//	    // Setup HTTP server
//	    http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
//	        evt := UserCreated{
//	            UserID: r.FormValue("user_id"),
//	            Email:  r.FormValue("email"),
//	        }
//
//	        if err := publisher.Publish(r.Context(), evt); err != nil {
//	            http.Error(w, err.Error(), http.StatusServiceUnavailable)
//	            return
//	        }
//
//	        w.WriteHeader(http.StatusOK)
//	    })
//
//	    server := &http.Server{Addr: ":8080"}
//
//	    g.Go(func() error {
//	        log.Println("Server starting on :8080")
//	        if err := server.ListenAndServe(); err != http.ErrServerClosed {
//	            return err
//	        }
//	        return nil
//	    })
//
//	    g.Go(func() error {
//	        <-ctx.Done()
//	        return server.Shutdown(context.Background())
//	    })
//
//	    if err := g.Wait(); err != nil {
//	        log.Printf("Error: %v", err)
//	    }
//	}
package event
