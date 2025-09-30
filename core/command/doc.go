// Package command provides a type-safe command bus implementation inspired by Watermill.
// It separates concerns into three components: Dispatcher (client), Transport (wire),
// and Processor (router/worker manager).
//
// Commands represent intent/orders with one-to-one handler relationships.
// Each command has exactly one handler, and missing handlers are errors.
//
// # Architecture (Watermill-Inspired)
//
// The package follows Watermill's architectural pattern:
//
//   - **Dispatcher** = Stateless client (like Watermill's Publisher)
//   - **Transport** = Passive wire (like Watermill's Subscriber - provides channel)
//   - **Processor** = Active router (like Watermill's Router - manages workers)
//
// This separation enables flexible deployment patterns:
//   - Local: Single process runs Dispatcher and Processor with shared transport
//   - Distributed: Dispatcher in web server, Processor in worker service
//   - Testing: Sync transport for deterministic, in-process execution
//
// # Core Concepts
//
// Commands are intent-based operations like CreateUser, GenerateThumbnail, SendEmail.
// Each command type maps to exactly one handler. The package provides:
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
// # Quick Start: Sync Transport (Simple)
//
// Synchronous execution in the same process:
//
//	import "github.com/dmitrymomot/foundation/core/command"
//
//	type CreateUser struct {
//	    Email string
//	    Name  string
//	}
//
//	func createUserHandler(ctx context.Context, cmd CreateUser) error {
//	    return db.Insert(ctx, cmd.Email, cmd.Name)
//	}
//
//	// Create transport (passive wire)
//	transport := command.NewSyncTransport()
//
//	// Create processor (active manager with handlers)
//	processor := command.NewProcessor(transport)
//	processor.Register(command.NewHandlerFunc(createUserHandler))
//
//	// Start processor (blocks until context cancelled)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go processor.Run(ctx)
//
//	// Create dispatcher (stateless client)
//	dispatcher := command.NewDispatcher(transport)
//
//	// Dispatch command - executes immediately
//	err := dispatcher.Dispatch(ctx, CreateUser{
//	    Email: "user@example.com",
//	    Name:  "John Doe",
//	})
//	if err != nil {
//	    // Handler error returned immediately
//	    log.Fatal(err)
//	}
//
// For sync transport, you can also use Processor.Dispatch() directly:
//
//	processor := command.NewProcessor(command.NewSyncTransport())
//	processor.Register(command.NewHandlerFunc(createUserHandler))
//	go processor.Run(ctx)
//
//	err := processor.Dispatch(ctx, CreateUser{Email: "user@example.com"})
//
// # Quick Start: Channel Transport (Async)
//
// Asynchronous execution with background workers managed by Processor:
//
//	import (
//	    "github.com/dmitrymomot/foundation/core/command"
//	    "golang.org/x/sync/errgroup"
//	)
//
//	// Create transport (passive wire - just a channel)
//	transport := command.NewChannelTransport(100)
//
//	// Create processor (active manager - controls workers)
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithWorkers(5),  // Processor controls worker count!
//	    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	        logger.Error("command failed", "command", cmdName, "error", err)
//	    }),
//	    command.WithLogger(logger),
//	)
//	processor.Register(command.NewHandlerFunc(sendEmailHandler))
//
//	// Start processor (manages worker lifecycle)
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(func() error {
//	    return processor.Run(ctx) // Blocks until context cancelled
//	})
//
//	// Create dispatcher (can be in different part of code)
//	dispatcher := command.NewDispatcher(transport)
//
//	// Dispatch returns immediately
//	err := dispatcher.Dispatch(ctx, SendEmail{To: "user@example.com"})
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
// Dispatcher and Processor in the same process, useful for simple apps:
//
//	transport := command.NewChannelTransport(100)
//
//	// Processor runs in background (manages workers)
//	processor := command.NewProcessor(transport, command.WithWorkers(5))
//	processor.Register(handler)
//	go processor.Run(ctx)
//
//	// Dispatcher used in HTTP handlers
//	dispatcher := command.NewDispatcher(transport)
//	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
//	    dispatcher.Dispatch(r.Context(), CreateUser{...})
//	    w.WriteHeader(http.StatusAccepted) // Returns immediately
//	})
//
// ## Pattern 2: Dispatcher-Only (Web Server)
//
// Web server dispatches to external queue/workers (future):
//
//	transport := redis.NewTransport("redis://localhost")
//	dispatcher := command.NewDispatcher(transport)
//
//	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
//	    dispatcher.Dispatch(r.Context(), CreateUser{...})
//	    w.WriteHeader(http.StatusAccepted)
//	})
//
// ## Pattern 3: Processor-Only (Worker Service)
//
// Dedicated worker service processes commands from queue (future):
//
//	transport := redis.NewTransport("redis://localhost")
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithWorkers(10),
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
//	func TestCreateUser(t *testing.T) {
//	    transport := command.NewSyncTransport()
//	    processor := command.NewProcessor(transport)
//	    processor.Register(command.NewHandlerFunc(createUserHandler))
//	    go processor.Run(context.Background())
//
//	    dispatcher := command.NewDispatcher(transport)
//	    err := dispatcher.Dispatch(ctx, CreateUser{Email: "test@example.com"})
//	    require.NoError(t, err)
//
//	    // Assertions run immediately after (synchronous execution)
//	    assertUserExists(t, "test@example.com")
//	}
//
// # Transports
//
// ## Sync Transport
//
// Executes commands immediately in the caller's goroutine.
// This is the simplest and most efficient transport with zero overhead.
//
// Characteristics:
//   - Direct function call (no goroutines, no channels)
//   - Synchronous error handling
//   - Runs in caller's context
//   - No worker management needed
//
// Use cases:
//   - HTTP request-response handlers
//   - Database transactions
//   - Testing (deterministic execution)
//   - Simple applications
//
// Example:
//
//	transport := command.NewSyncTransport()
//	processor := command.NewProcessor(transport)
//	processor.Register(command.NewHandlerFunc(handler))
//	go processor.Run(ctx)
//
//	dispatcher := command.NewDispatcher(transport)
//	err := dispatcher.Dispatch(ctx, CreateUser{...})
//	// Error is from handler execution
//
// ## Channel Transport
//
// Executes commands asynchronously using buffered channels.
// Transport is a passive wire - Processor manages workers.
//
// Characteristics:
//   - Non-blocking dispatch
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
//   - Non-critical async work
//
// Example:
//
//	transport := command.NewChannelTransport(100)
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithWorkers(5),  // Processor controls workers
//	    command.WithErrorHandler(errorHandler),
//	)
//	processor.Register(handler)
//	go processor.Run(ctx)
//
//	dispatcher := command.NewDispatcher(transport)
//	err := dispatcher.Dispatch(ctx, SendEmail{...})
//	// Error is only dispatch error (ErrBufferFull)
//	// Handler errors reported via errorHandler callback
//
// # Middleware
//
// Middleware wraps all handlers to add cross-cutting functionality like logging,
// metrics, tracing, validation, or authorization.
//
// IMPORTANT: Middleware is immutable and must be configured at Processor construction
// using WithMiddleware(). It cannot be added or modified after creation.
//
// Built-in middleware:
//   - LoggingMiddleware: Logs command execution with timing
//
// Example:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithMiddleware(
//	        command.LoggingMiddleware(logger),
//	        metricsMiddleware,
//	        tracingMiddleware,
//	    ),
//	)
//
// Custom middleware:
//
//	func metricsMiddleware(next command.Handler) command.Handler {
//	    return command.NewHandlerFunc(func(ctx context.Context, payload any) error {
//	        start := time.Now()
//	        err := next.Handle(ctx, payload)
//	        metrics.Observe(next.Name(), time.Since(start), err != nil)
//	        return err
//	    })
//	}
//
// # Decorators
//
// Decorators wrap individual handlers to add retry, backoff, or timeout logic.
// Unlike middleware (applied to all handlers), decorators are applied per-handler.
//
// ## Using WithXXX Functions
//
//	handler := command.WithTimeout(
//	    command.WithBackoff(
//	        command.WithRetry(
//	            command.NewHandlerFunc(apiCallHandler),
//	            3, // max retries
//	        ),
//	        5, 100*time.Millisecond, 10*time.Second,
//	    ),
//	    60*time.Second,
//	)
//	processor.Register(handler)
//
// ## Using Decorator Chaining (Cleaner)
//
// The Decorate() helper provides cleaner syntax:
//
//	handler := command.Decorate(
//	    command.NewHandlerFunc(apiCallHandler),
//	    command.Retry(3),
//	    command.Backoff(5, 100*time.Millisecond, 10*time.Second),
//	    command.Timeout(60*time.Second),
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
// In the example above: Timeout wraps Backoff wraps Retry wraps Handler.
//
// # Error Handling
//
// Error handling differs between sync and async transports:
//
// ## Sync Transport
//
// Handler errors are returned immediately to the caller:
//
//	err := dispatcher.Dispatch(ctx, cmd)
//	if err != nil {
//	    // Could be:
//	    // - ErrHandlerNotFound: No handler registered
//	    // - Handler error: Error from command execution
//	    // - Panic error: Handler panicked (caught and converted)
//	}
//
// ## Channel Transport
//
// Dispatch returns only dispatch errors (ErrBufferFull, etc).
// Handler errors are reported via WithErrorHandler callback:
//
//	processor := command.NewProcessor(
//	    transport,
//	    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	        logger.Error("command failed",
//	            "command", cmdName,
//	            "error", err,
//	            "trace_id", ctx.Value("trace_id"),
//	        )
//	        metrics.CommandFailed.Inc()
//	    }),
//	)
//
//	// Dispatch returns immediately
//	err := dispatcher.Dispatch(ctx, cmd)
//	if err == command.ErrBufferFull {
//	    // Channel buffer is full, apply backpressure
//	    return http.StatusServiceUnavailable
//	}
//
// # Panic Recovery
//
// All transports include unified panic recovery. If a handler panics, the panic
// is caught and converted to an error, preventing the entire process from crashing.
//
//	func riskyHandler(ctx context.Context, cmd ProcessData) error {
//	    panic("something went wrong") // Caught by transport
//	}
//
// Sync transport: Returns panic as error to caller
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
//	// 1. Channel transport: Closes channel, workers drain remaining commands
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
// 1. Commands should be self-contained data structures with all needed data
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
// 13. Dispatch context is propagated to handlers - use it for cancellation/values
// 14. Separate Dispatcher and Processor for distributed architectures
// 15. Share transport instance between Dispatcher and Processor in same process
// 16. Processor controls workers, not Transport (Watermill pattern)
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
//	    "github.com/dmitrymomot/foundation/core/command"
//	    "golang.org/x/sync/errgroup"
//	)
//
//	type CreateUser struct {
//	    Email string
//	    Name  string
//	}
//
//	func createUserHandler(ctx context.Context, cmd CreateUser) error {
//	    log.Printf("Creating user: %s (%s)", cmd.Name, cmd.Email)
//	    // Insert into database
//	    return nil
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
//	    transport := command.NewChannelTransport(100)
//
//	    // Create processor (active manager with workers)
//	    processor := command.NewProcessor(
//	        transport,
//	        command.WithWorkers(5),  // Processor controls workers
//	        command.WithMiddleware(command.LoggingMiddleware(logger)),
//	        command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	            log.Printf("Command %s failed: %v", cmdName, err)
//	        }),
//	    )
//
//	    // Register handlers with decorators
//	    processor.Register(command.Decorate(
//	        command.NewHandlerFunc(createUserHandler),
//	        command.Retry(3),
//	        command.Timeout(5*time.Second),
//	    ))
//
//	    // Start processor
//	    g, ctx := errgroup.WithContext(ctx)
//	    g.Go(func() error {
//	        return processor.Run(ctx)
//	    })
//
//	    // Create dispatcher for HTTP handlers
//	    dispatcher := command.NewDispatcher(transport)
//
//	    // Setup HTTP server
//	    http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
//	        cmd := CreateUser{
//	            Email: r.FormValue("email"),
//	            Name:  r.FormValue("name"),
//	        }
//
//	        if err := dispatcher.Dispatch(r.Context(), cmd); err != nil {
//	            http.Error(w, err.Error(), http.StatusServiceUnavailable)
//	            return
//	        }
//
//	        w.WriteHeader(http.StatusAccepted)
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
package command
