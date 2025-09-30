// Package command provides a type-safe command bus implementation with pluggable
// transport strategies, middleware support, and unified panic recovery.
//
// Commands represent intent/orders with one-to-one handler relationships.
// Each command has exactly one handler, and missing handlers are errors.
//
// # Core Concepts
//
// Commands are intent-based operations like CreateUser, GenerateThumbnail, SendEmail.
// Each command type maps to exactly one handler. The package provides:
//
//   - Two execution strategies (Sync, Channel)
//   - Type-safe handlers via generics
//   - Immutable middleware configured at construction
//   - Context-based lifecycle management
//   - Unified panic recovery across all transports
//   - Decorator pattern for retry, timeout, backoff
//
// # Quick Start
//
// Basic synchronous command execution:
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
//	dispatcher := command.NewDispatcher(command.WithSyncTransport())
//	dispatcher.Register(command.NewHandlerFunc(createUserHandler))
//
//	err := dispatcher.Dispatch(ctx, CreateUser{
//	    Email: "user@example.com",
//	    Name:  "John Doe",
//	})
//
// # Sync Transport
//
// Synchronous transport executes commands immediately in the caller's goroutine.
// This is the simplest and most efficient transport with zero overhead.
//
// Characteristics:
//   - Direct function call (no goroutines, no channels)
//   - Synchronous error handling
//   - Runs in caller's context
//   - No lifecycle management needed
//
// Use cases:
//   - HTTP request-response handlers
//   - Database transactions
//   - Testing (deterministic execution)
//   - Simple applications
//
// Example:
//
//	dispatcher := command.NewDispatcher(command.WithSyncTransport())
//	dispatcher.Register(command.NewHandlerFunc(createUserHandler))
//
//	err := dispatcher.Dispatch(ctx, CreateUser{Email: "user@example.com"})
//	if err != nil {
//	    return err // Handler error or ErrHandlerNotFound
//	}
//
// # Channel Transport
//
// Channel transport executes commands asynchronously using buffered channels
// and worker goroutines.
//
// Characteristics:
//   - Non-blocking dispatch
//   - Buffered channel (configurable size)
//   - Local execution (same process)
//   - No persistence (commands lost on shutdown)
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
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel() // Triggers graceful shutdown
//
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(ctx, 100, command.WithWorkers(5)),
//	    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	        logger.Error("command failed", "command", cmdName, "error", err)
//	    }),
//	)
//
//	dispatcher.Register(command.NewHandlerFunc(sendEmailHandler))
//
//	// Returns immediately, preserving dispatch context for handler
//	err := dispatcher.Dispatch(ctx, SendEmail{To: "user@example.com"})
//	if err != nil {
//	    // ErrBufferFull or ErrHandlerNotFound only
//	    // Handler errors are reported via WithErrorHandler callback
//	    return err
//	}
//
// # Middleware
//
// Middleware wraps handlers to add cross-cutting functionality like logging,
// metrics, tracing, validation, or authorization.
//
// IMPORTANT: Middleware is immutable and must be configured at construction time
// using WithMiddleware(). It cannot be added or modified after the dispatcher
// is created. This design ensures thread-safety and predictable behavior.
//
// Built-in middleware:
//   - LoggingMiddleware: Logs command execution with timing
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithMiddleware(
//	        command.LoggingMiddleware(logger),
//	        metricsMiddleware,
//	    ),
//	)
//	dispatcher.Register(command.NewHandlerFunc(handler))
//
// Custom middleware:
//
//	func metricsMiddleware(next command.Handler) command.Handler {
//	    return &middlewareHandler{
//	        name: next.Name(),
//	        fn: func(ctx context.Context, payload any) error {
//	            start := time.Now()
//	            err := next.Handle(ctx, payload)
//	            metrics.Observe(next.Name(), time.Since(start))
//	            return err
//	        },
//	    }
//	}
//
// # Decorators
//
// Decorators wrap individual handlers to add retry, backoff, or timeout logic.
// They are composable and applied per-handler.
//
// Retry example:
//
//	handler := command.WithRetry(
//	    command.NewHandlerFunc(apiCallHandler),
//	    3, // max retries
//	)
//	dispatcher.Register(handler)
//
// Backoff example:
//
//	handler := command.WithBackoff(
//	    command.NewHandlerFunc(sendEmailHandler),
//	    5,                    // max retries
//	    100*time.Millisecond, // initial delay
//	    10*time.Second,       // max delay
//	)
//	dispatcher.Register(handler)
//
// Timeout example:
//
//	handler := command.WithTimeout(
//	    command.NewHandlerFunc(processImageHandler),
//	    30*time.Second,
//	)
//	dispatcher.Register(handler)
//
// Composition:
//
//	handler := command.WithTimeout(
//	    command.WithRetryAndBackoff(
//	        command.NewHandlerFunc(apiCallHandler),
//	        5, 100*time.Millisecond, 10*time.Second,
//	    ),
//	    60*time.Second,
//	)
//
// # Testing
//
// Sync transport is ideal for testing - it's deterministic with no timing issues.
//
// Example:
//
//	func TestCreateUser(t *testing.T) {
//	    dispatcher := command.NewDispatcher(command.WithSyncTransport())
//	    dispatcher.Register(command.NewHandlerFunc(createUserHandler))
//
//	    err := dispatcher.Dispatch(ctx, CreateUser{Email: "test@example.com"})
//	    require.NoError(t, err)
//
//	    // Assertions run immediately after
//	    assertUserExists(t, "test@example.com")
//	}
//
// # Error Handling
//
// Sync transport returns handler errors immediately:
//
//	err := dispatcher.Dispatch(ctx, cmd)
//	// err is handler error or ErrHandlerNotFound
//
// Channel transport returns dispatch errors only:
//
//	err := dispatcher.Dispatch(ctx, cmd)
//	// err is ErrBufferFull or ErrHandlerNotFound
//	// Handler errors handled via WithErrorHandler callback
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
// The original dispatch context is always propagated to handlers, even in async
// transports, ensuring context values and cancellation work correctly.
//
// # Graceful Shutdown
//
// Channel transport uses context-based lifecycle management for clean shutdown.
// When the context is cancelled, workers drain all pending commands before exiting.
//
// Basic shutdown:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel() // Triggers graceful shutdown
//
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(ctx, 100),
//	    command.WithErrorHandler(errorHandler),
//	)
//
//	// When done:
//	cancel() // Workers drain channel and exit gracefully
//
// Signal-based shutdown:
//
//	import "os/signal"
//	import "syscall"
//
//	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
//	defer stop()
//
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(ctx, 100),
//	    command.WithErrorHandler(errorHandler),
//	)
//
//	// Shutdown happens automatically on SIGTERM/SIGINT
//
// Note: Sync transport has no lifecycle management - it executes immediately and
// requires no cleanup.
//
// # Best Practices
//
// 1. Commands should be self-contained data structures with all needed data
// 2. Use sync transport for testing (deterministic, no timing issues)
// 3. Use sync transport for transactional operations (immediate errors)
// 4. Use channel transport for fire-and-forget operations
// 5. Always provide WithErrorHandler with async transports
// 6. Pass a cancellable context to WithChannelTransport for lifecycle management
// 7. Configure middleware at construction time (immutable after creation)
// 8. Apply decorators at registration time, not inside handlers
// 9. Use middleware for cross-cutting concerns (logging, metrics, tracing)
// 10. Keep handlers simple and focused on business logic
// 11. Let panic recovery handle unexpected failures gracefully
// 12. Dispatch context is propagated to handlers - use it for cancellation/values
//
// # Upgrade Path
//
// Start simple and add complexity as needed:
//
//	// Phase 1: Simple app, sync
//	dispatcher := command.NewDispatcher(command.WithSyncTransport())
//
//	// Phase 2: Need decoupling, async local
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	dispatcher := command.NewDispatcher(command.WithChannelTransport(ctx, 100))
package command
