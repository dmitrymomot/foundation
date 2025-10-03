// Package command provides a type-safe CQRS command pattern implementation for Go applications.
//
// This package implements a flexible command bus architecture that decouples command
// producers from consumers using channels or external message brokers. It supports
// type-safe handlers using generics, handler decorators for cross-cutting concerns,
// and graceful lifecycle management with health checks and observability metrics.
//
// # Core Components
//
// The package provides these main types:
//
//   - Command: Envelope containing command metadata (ID, Name, Payload, CreatedAt)
//   - Handler: Interface for processing commands of specific types
//   - Dispatcher: Coordinates command routing and handler execution
//   - Sender: Publishes commands to the command bus
//   - ChannelBus: In-memory command bus using Go channels (for monolithic apps)
//   - Decorator: Function wrapper for adding cross-cutting functionality
//
// # Basic Usage
//
// Define a command type and handler:
//
//	import "github.com/dmitrymomot/foundation/core/command"
//
//	type CreateUser struct {
//		UserID string
//		Email  string
//		Name   string
//	}
//
//	func handleCreateUser(ctx context.Context, cmd CreateUser) error {
//		// Process command
//		log.Printf("Creating user: %s (%s)", cmd.Name, cmd.Email)
//		// ... database operations, business logic, etc.
//		return nil
//	}
//
//	// Create handler (command name derived from type automatically)
//	handler := command.NewHandlerFunc(handleCreateUser)
//
// Set up dispatcher and sender with in-memory channel bus:
//
//	// Create in-memory command bus
//	bus := command.NewChannelBus(
//		command.WithBufferSize(100),
//		command.WithChannelLogger(logger),
//	)
//	defer bus.Close()
//
//	// Create dispatcher
//	dispatcher := command.NewDispatcher(
//		command.WithCommandSource(bus),
//		command.WithHandler(handler),
//		command.WithDispatcherLogger(logger),
//	)
//
//	// Start dispatcher in background
//	go func() {
//		if err := dispatcher.Start(ctx); err != nil {
//			log.Printf("Dispatcher error: %v", err)
//		}
//	}()
//	defer dispatcher.Stop()
//
//	// Create sender
//	sender := command.NewSender(bus, command.WithSenderLogger(logger))
//
//	// Send command
//	err := sender.Send(ctx, CreateUser{
//		UserID: "123",
//		Email:  "user@example.com",
//		Name:   "John Doe",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Handler Registration
//
// There are two ways to create handlers:
//
//	// Option 1: Automatic command name (derived from type)
//	handler1 := command.NewHandlerFunc(func(ctx context.Context, cmd CreateUser) error {
//		return processCreateUser(ctx, cmd)
//	})
//	// handler1.CommandName() returns "CreateUser"
//
//	// Option 2: Explicit command name
//	handler2 := command.NewHandler("user.create", func(ctx context.Context, payload any) error {
//		cmd := payload.(CreateUser)
//		return processCreateUser(ctx, cmd)
//	})
//	// handler2.CommandName() returns "user.create"
//
// Register multiple handlers with a dispatcher:
//
//	dispatcher := command.NewDispatcher(
//		command.WithCommandSource(bus),
//		command.WithHandler(handler1, handler2, handler3),
//	)
//
// # Decorators
//
// Use decorators to add cross-cutting functionality like logging, metrics, or retries:
//
//	// Built-in timeout decorator
//	handler := command.NewHandlerFunc(
//		command.ApplyDecorators(
//			handleCreateUser,
//			command.WithTimeout[CreateUser](5*time.Second),
//		),
//	)
//
//	// Custom logging decorator
//	func LoggingDecorator[T any](fn command.HandlerFunc[T]) command.HandlerFunc[T] {
//		return func(ctx context.Context, payload T) error {
//			log.Printf("Processing command: %T", payload)
//			err := fn(ctx, payload)
//			if err != nil {
//				log.Printf("Command failed: %v", err)
//			}
//			return err
//		}
//	}
//
//	// Apply multiple decorators (executed in order: Logging -> Metrics -> Retry -> handler)
//	handler := command.NewHandlerFunc(
//		command.ApplyDecorators(
//			handleCreateUser,
//			LoggingDecorator[CreateUser],
//			MetricsDecorator[CreateUser],
//			RetryDecorator[CreateUser],
//		),
//	)
//
// # Dispatcher Lifecycle
//
// The dispatcher supports three lifecycle patterns:
//
//	// Pattern 1: Blocking Start/Stop
//	go func() {
//		if err := dispatcher.Start(ctx); err != nil {
//			log.Fatal(err)
//		}
//	}()
//	// ... application logic ...
//	if err := dispatcher.Stop(); err != nil {
//		log.Printf("Shutdown error: %v", err)
//	}
//
//	// Pattern 2: Context cancellation
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	go func() {
//		if err := dispatcher.Start(ctx); err != nil {
//			log.Printf("Dispatcher stopped: %v", err)
//		}
//	}()
//	// cancel() triggers graceful shutdown
//
//	// Pattern 3: errgroup coordination
//	g, ctx := errgroup.WithContext(context.Background())
//	g.Go(dispatcher.Run(ctx))
//	// ... start other services ...
//	if err := g.Wait(); err != nil {
//		log.Fatal(err)
//	}
//
// # Configuration Options
//
// Configure dispatcher behavior:
//
//	dispatcher := command.NewDispatcher(
//		command.WithCommandSource(bus),
//		command.WithHandler(handler1, handler2),
//		command.WithMaxConcurrentHandlers(100),         // Limit concurrent handlers
//		command.WithShutdownTimeout(30*time.Second),    // Graceful shutdown timeout
//		command.WithStaleThreshold(5*time.Minute),      // Health check threshold
//		command.WithStuckThreshold(1000),               // Max active commands threshold
//		command.WithDispatcherLogger(logger),           // Structured logging
//		command.WithFallbackHandler(fallbackHandler),   // Handle unregistered commands
//	)
//
// # Fallback Handler
//
// Use a fallback handler for commands without registered handlers:
//
//	dispatcher := command.NewDispatcher(
//		command.WithCommandSource(bus),
//		command.WithHandler(handler1),
//		command.WithFallbackHandler(func(ctx context.Context, cmd command.Command) error {
//			log.Warn("Unhandled command",
//				"id", cmd.ID,
//				"name", cmd.Name,
//				"created_at", cmd.CreatedAt)
//			// Forward to dead letter queue, log for analysis, etc.
//			return nil
//		}),
//	)
//
// # Observability
//
// Monitor dispatcher health and metrics:
//
//	// Get real-time statistics
//	stats := dispatcher.Stats()
//	log.Printf("Processed: %d, Failed: %d, Active: %d",
//		stats.CommandsProcessed,
//		stats.CommandsFailed,
//		stats.ActiveCommands)
//
//	// Health check endpoint
//	http.HandleFunc("/health/commands", func(w http.ResponseWriter, r *http.Request) {
//		if err := dispatcher.Healthcheck(r.Context()); err != nil {
//			http.Error(w, err.Error(), http.StatusServiceUnavailable)
//			return
//		}
//		json.NewEncoder(w).Encode(dispatcher.Stats())
//	})
//
// # Context Helpers
//
// Extract command metadata from handler context:
//
//	func handleCreateUser(ctx context.Context, cmd CreateUser) error {
//		// Extract command metadata
//		commandID := command.CommandID(ctx)           // Command ID
//		commandName := command.CommandName(ctx)       // Command name
//		createdAt := command.CommandTime(ctx)         // Command creation time
//		startedAt := command.StartProcessingTime(ctx) // Handler start time
//
//		log.Printf("Processing %s (%s) created at %v",
//			commandName, commandID, createdAt)
//
//		// Calculate processing latency
//		latency := time.Since(createdAt)
//		log.Printf("Command latency: %v", latency)
//
//		return nil
//	}
//
// # Channel Bus
//
// The ChannelBus provides an in-memory command bus for monolithic applications:
//
//	bus := command.NewChannelBus(
//		command.WithBufferSize(100),          // Buffer up to 100 commands
//		command.WithChannelLogger(logger),    // Enable logging
//	)
//	defer bus.Close()
//
//	// Use with sender and dispatcher
//	sender := command.NewSender(bus)
//	dispatcher := command.NewDispatcher(
//		command.WithCommandSource(bus),
//		command.WithHandler(handler),
//	)
//
// For distributed systems, implement the commandBus and commandSource interfaces
// using external message brokers (RabbitMQ, Kafka, Redis Streams, etc.).
//
// # Custom Command Bus
//
// Implement custom command bus for external brokers:
//
//	// commandBus interface for sending
//	type commandBus interface {
//		Publish(ctx context.Context, data []byte) error
//	}
//
//	// commandSource interface for receiving
//	type commandSource interface {
//		Commands() <-chan []byte
//	}
//
//	// Example: Redis Streams implementation
//	type RedisCommandBus struct {
//		client *redis.Client
//		stream string
//	}
//
//	func (r *RedisCommandBus) Publish(ctx context.Context, data []byte) error {
//		return r.client.XAdd(ctx, &redis.XAddArgs{
//			Stream: r.stream,
//			Values: map[string]interface{}{"data": data},
//		}).Err()
//	}
//
//	func (r *RedisCommandBus) Commands() <-chan []byte {
//		ch := make(chan []byte)
//		go r.consumeStream(ch)
//		return ch
//	}
//
// # Error Handling
//
// The package defines these error types:
//
//   - ErrNoHandler: No handler registered for command
//   - ErrHandlerAlreadyRegistered: Duplicate handler registration
//   - ErrDispatcherAlreadyStarted: Dispatcher already running
//   - ErrDispatcherNotStarted: Dispatcher not started
//   - ErrCommandSourceNil: Command source is nil
//   - ErrHealthcheckFailed: Health check failed
//   - ErrDispatcherNotRunning: Dispatcher not running
//   - ErrDispatcherStale: No recent activity
//   - ErrDispatcherStuck: Too many active commands
//   - ErrChannelBusClosed: Channel bus already closed
//
// Handlers should return domain-specific errors for business logic failures.
// Infrastructure errors (storage failures, network errors) should be wrapped
// to provide context.
//
// # Command Naming
//
// By default, command names are derived from type names without package paths.
// For example, both users.CreateUser and billing.CreateUser resolve to "CreateUser"
// and trigger the same handler. Ensure unique command type names across your
// codebase to avoid handler collisions, or use explicit command names with
// NewHandler for namespacing.
//
// # Thread Safety
//
// All components are designed for concurrent use:
//   - Dispatcher processes commands concurrently (unless limited by WithMaxConcurrentHandlers)
//   - ChannelBus is safe for concurrent publishers
//   - Sender can be used from multiple goroutines
//   - Handlers may execute in parallel for different commands
//
// # Performance Considerations
//
// For high-throughput applications, consider:
//   - Set WithMaxConcurrentHandlers to prevent unbounded goroutine spawning
//   - Use appropriate buffer sizes for ChannelBus
//   - Implement handler-level timeouts with WithTimeout decorator
//   - Monitor dispatcher metrics for bottlenecks
//   - For distributed systems, use external message brokers instead of ChannelBus
//
// # Design Patterns
//
// This implementation follows CQRS (Command Query Responsibility Segregation)
// principles:
//   - Commands represent write operations or state changes
//   - Commands are fire-and-forget (no return values except errors)
//   - Commands are processed asynchronously
//   - Each command type has exactly one handler
//   - Handlers are idempotent when possible
//
// For read operations (queries), use a separate query handler pattern or
// direct database access.
package command
