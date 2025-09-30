// Package command provides a command bus implementation for executing commands
// with pluggable transport strategies.
//
// Commands represent intent/orders with one-to-one handler relationships.
// Each command has exactly one handler, and missing handlers are errors.
//
// # Core Concepts
//
// Commands are intent-based operations like CreateUser, GenerateThumbnail, SendEmail.
// Each command type maps to exactly one handler. The package provides two execution
// strategies via transport implementations:
//
//   - Sync: Direct synchronous execution (zero overhead)
//   - Channel: Asynchronous execution via buffered channels
//
// # Quick Start
//
// Basic synchronous command execution:
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
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(100, command.WithWorkers(5)),
//	    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
//	        logger.Error("command failed", "command", cmdName, "error", err)
//	    }),
//	)
//	defer dispatcher.Stop() // Graceful shutdown
//
//	dispatcher.Register(command.NewHandlerFunc(sendEmailHandler))
//
//	// Returns immediately
//	err := dispatcher.Dispatch(ctx, SendEmail{To: "user@example.com"})
//	if err != nil {
//	    // ErrBufferFull or ErrHandlerNotFound only
//	    return err
//	}
//
// # Middleware
//
// Middleware wraps handlers to add cross-cutting functionality like logging,
// metrics, tracing, validation, or authorization.
//
// Middleware must be configured at construction time using WithMiddleware().
// It cannot be added or modified after the dispatcher is created.
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
// # Graceful Shutdown
//
// Channel transport requires graceful shutdown to drain pending commands:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithChannelTransport(100),
//	    command.WithErrorHandler(errorHandler),
//	)
//	defer dispatcher.Stop() // Blocks until workers finish (30s timeout)
//
//	// Or with signal handling:
//	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
//	defer stop()
//
//	go func() {
//	    <-ctx.Done()
//	    dispatcher.Stop()
//	}()
//
// # Best Practices
//
// 1. Commands should be self-contained data structures
// 2. Include all necessary data in the command (don't rely on context values)
// 3. Use sync transport for testing
// 4. Use sync transport for transactional operations
// 5. Use channel transport for fire-and-forget operations
// 6. Always provide an error handler with async transports
// 7. Call dispatcher.Stop() for graceful shutdown with channel transport
// 8. Configure middleware at construction time using WithMiddleware()
// 9. Apply decorators at registration time, not in handlers
// 10. Use middleware for cross-cutting concerns
// 11. Keep handlers simple and focused
//
// # Upgrade Path
//
// Start simple and add complexity as needed:
//
//	// Phase 1: Simple app, sync
//	dispatcher := command.NewDispatcher(command.WithSyncTransport())
//
//	// Phase 2: Need decoupling, async local
//	dispatcher := command.NewDispatcher(command.WithChannelTransport(100))
package command
