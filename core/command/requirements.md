# Command Bus Requirements

## Overview

Command bus implementation for executing commands with pluggable transport strategies. Commands represent **intent/orders** with one-to-one handler relationships (one command = one handler).

## Core Concepts

### Command Semantics

- **Intent-based**: Commands are orders to do something (CreateUser, GenerateThumbnail)
- **One-to-one**: Each command has exactly one handler
- **Must be handled**: Missing handler is an error (behavior depends on transport)
- **Can be sync or async**: Determined by transport implementation

### Transport Abstraction

Two built-in transports with different characteristics:

| Transport   | Execution          | Blocking | Validation | Error Handling      | Use Case                       |
| ----------- | ------------------ | -------- | ---------- | ------------------- | ------------------------------ |
| **Sync**    | Direct call        | Yes      | Immediate  | Returns to caller   | Transactions, request-response |
| **Channel** | Goroutine + buffer | No       | Immediate  | Callback/middleware | Local async, decoupling        |

## Architecture Decisions

### 1. Handler Registration

**Decision**: Panic on duplicate handler registration

**Rationale**:

- Duplicate handlers = programming error (configuration mistake)
- Should fail fast during initialization
- Consistent with command one-to-one semantics

```go
bus.Register(command.HandlerFunc(createUser))
bus.Register(command.HandlerFunc(createUser)) // PANIC: duplicate handler
```

### 2. Missing Handler Behavior

**Decision**: Behavior depends on transport type

**Sync Transport**:

- Validates handler exists on `Dispatch()`
- Returns `ErrHandlerNotFound` immediately
- Rationale: Caller needs synchronous error

**Channel Transport**:

- Validates handler exists on `Dispatch()`
- Returns `ErrHandlerNotFound` immediately
- Rationale: Fail fast, no point buffering unhandleable command

**Queue Transport**:

- Accepts command without validation (enqueues)
- Worker validates handler when processing
- Missing handler → moves task to DLQ
- Rationale: Command may be handled by different instance

### 3. Type Safety

**Decision**: Use generics for type-safe handlers

**Rationale**:

- Compile-time type checking
- No manual unmarshaling in handlers
- Consistent with existing `core` package patterns
- Better developer experience

```go
func createUser(ctx context.Context, cmd CreateUser) error {
    // cmd is strongly typed
    return db.Insert(cmd.Email, cmd.Name)
}

bus.Register(command.HandlerFunc(createUser))
```

### 4. Serialization

**Decision**: Use standard library `encoding/json`

**Rationale**:

- Zero dependencies
- Sufficient for 95% of use cases
- Users can implement custom transport for protobuf/msgpack if needed
- Simple and predictable

### 5. Context Usage

**Decision**: All handlers receive `context.Context` as first parameter

**Rationale**:

- Tenant ID propagation (multi-tenant apps)
- Request tracing spans
- Database transaction passing
- Cancellation for long-running work
- Deadline enforcement

**Note**: Async transports lose parent context - handlers get fresh context

### 6. Error Handling

**Sync Transport**:

```go
err := bus.Dispatch(ctx, cmd)
// err is handler error or ErrHandlerNotFound
```

**Async Transports (Channel/Queue)**:

```go
err := bus.Dispatch(ctx, cmd)
// err is enqueue error (ErrBufferFull, network error)
// Handler errors handled via:
// - Error handler callback
// - Middleware logging
// - DLQ (queue transport)
```

**Decision**: Provide error handler callback option

```go
bus := command.New(
    command.WithChannelTransport(100),
    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
        logger.Error("command failed", "command", cmdName, "error", err)
    }),
)
```

### 7. Retry/Backoff Strategy

**Decision**: Implement via decorators, not built-in

**Rationale**:

- Keeps core simple
- Composable: `WithRetry(WithBackoff(handler))`
- Users only pay for complexity they need
- Similar to `http.Handler` middleware pattern

```go
handler := command.WithRetry(
    command.WithBackoff(createUserHandler),
    maxRetries: 3,
)
bus.Register(handler)
```

### 8. Middleware Support

**Decision**: Support middleware chain for cross-cutting concerns

**Built-in middleware**:

- Logging (default, requires `*slog.Logger` in constructor)

**User middleware**:

- Metrics collection
- Tracing
- Validation
- Authorization

```go
bus.Use(command.LoggingMiddleware(logger))
bus.Use(command.MetricsMiddleware(metrics))
```

### 9. API Naming

**Decision**: Follow established patterns

- `Dispatch(ctx, cmd)` - execute command (standard CQRS terminology)
- `Start(ctx)` - begin processing (async transports)
- `Stop()` - graceful shutdown
- `Run(ctx) func() error` - errgroup compatibility

**Rationale**: Consistent with `core/queue` and `core/server` packages

## Transport Specifications

### Sync Transport

**Characteristics**:

- Zero overhead (direct function call)
- Runs in caller's goroutine
- Synchronous error handling
- No lifecycle management needed

**Use cases**:

- Database transaction boundaries
- HTTP request-response
- Testing (deterministic)
- Simple applications

**API**:

```go
bus := command.New(command.WithSyncTransport())
bus.Register(handler)

// No Start() needed
err := bus.Dispatch(ctx, cmd) // Blocks until complete
if err != nil {
    // Handler error or ErrHandlerNotFound
}
```

### Channel Transport (Async Local)

**Characteristics**:

- Non-blocking dispatch
- Buffered channel (user-specified size)
- Local execution (same instance)
- No persistence (lost on restart)
- Error handling via callback/middleware

**Use cases**:

- Fire-and-forget operations
- Decoupling (don't block HTTP response)
- Local background tasks
- Non-critical async work

**API**:

```go
bus := command.New(
    command.WithChannelTransport(bufferSize: 100),
    command.WithErrorHandler(errorHandler),
)
bus.Register(handler)

ctx, cancel := context.WithCancel(context.Background())
go func() {
    if err := bus.Start(ctx); err != nil {
        log.Fatal(err)
    }
}()

err := bus.Dispatch(ctx, cmd) // Returns immediately
if err != nil {
    // ErrBufferFull or ErrHandlerNotFound
}

cancel()
bus.Stop() // Graceful shutdown
```

**Configuration options**:

- Buffer size (required)
- Number of workers (default: 1)
- Error handler (optional)

### Queue Transport (Async Distributed)

**Characteristics**:

- Non-blocking dispatch
- Persistent (survives restart)
- Cross-instance execution
- Retry/DLQ support
- Handler validation deferred to worker

**Use cases**:

- Background jobs
- Cross-instance work distribution
- Reliable task processing
- Image processing, notifications, etc.

**API**:

```go
// Dispatcher instance (e.g., web server)
bus := command.New(command.WithQueueTransport(repo))
err := bus.Dispatch(ctx, cmd) // Enqueues, returns immediately

// Worker instance (e.g., background worker)
bus := command.New(command.WithQueueTransport(repo))
bus.Register(handler)
bus.Start(ctx) // Blocks, processing commands from queue
```

**Implementation note**: Wraps existing `core/queue` package internally

## Package Structure

```
core/command/
  - bus.go              // Main Bus type
  - transport.go        // Transport interface
  - handler.go          // Handler interface, HandlerFunc
  - middleware.go       // Middleware types and built-ins
  - sync_transport.go   // Sync transport implementation
  - channel_transport.go // Async local transport
  - queue_transport.go  // Async distributed transport (wraps core/queue)
  - decorators.go       // Retry, backoff decorators
  - errors.go           // Package errors
  - doc.go              // Package documentation
```

**Note**: Flat structure, no sub-folders

## Usage Examples

### Simple Sync Example

```go
type CreateUser struct {
    Email string
    Name  string
}

func createUserHandler(ctx context.Context, cmd CreateUser) error {
    return db.Insert(ctx, cmd.Email, cmd.Name)
}

bus := command.New(command.WithSyncTransport())
bus.Register(command.HandlerFunc(createUserHandler))

if err := bus.Dispatch(ctx, CreateUser{Email: "test@example.com"}); err != nil {
    return err
}
```

### Async Local with Error Handling

```go
bus := command.New(
    command.WithChannelTransport(100),
    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
        logger.Error("command failed", "command", cmdName, "error", err)
        metrics.Inc("command.errors", "command", cmdName)
    }),
)
bus.Use(command.LoggingMiddleware(logger))
bus.Register(command.HandlerFunc(sendEmailHandler))

go bus.Start(ctx)

// Fire and forget
bus.Dispatch(ctx, SendEmail{To: "user@example.com"})
```

### Distributed Queue with Retry

```go
// Web server - dispatch only
dispatchBus := command.New(command.WithQueueTransport(repo))
dispatchBus.Dispatch(ctx, GenerateThumbnail{ImageID: "123"})

// Worker server - handles commands
workerBus := command.New(command.WithQueueTransport(repo))

handler := command.WithRetry(
    command.HandlerFunc(generateThumbnailHandler),
    maxRetries: 3,
)
workerBus.Register(handler)
workerBus.Start(ctx)
```

## Upgrade Path

Developers can start simple and progressively add complexity:

```go
// Phase 1: Simple app, sync
bus := command.New(command.WithSyncTransport())

// Phase 2: Need decoupling, async local
bus := command.New(command.WithChannelTransport(100))

// Phase 3: Scale to multiple instances
bus := command.New(command.WithQueueTransport(repo))
```

## Testing Considerations

**Sync transport is ideal for testing**:

- Deterministic execution
- No timing issues
- Synchronous errors
- No goroutine leaks

```go
func TestCreateUser(t *testing.T) {
    bus := command.New(command.WithSyncTransport())
    bus.Register(command.HandlerFunc(createUserHandler))

    err := bus.Dispatch(ctx, CreateUser{Email: "test@example.com"})
    require.NoError(t, err)

    // Assertions run immediately after
    assertUserExists(t, "test@example.com")
}
```

## Open Questions

1. Should sync transport support `Start()` as no-op for API consistency?
2. Default transport if none specified - sync or channel?
3. Channel transport worker count configuration?
4. Should middleware apply to all transports or be transport-specific?
5. Panic recovery strategy for handlers?

## References

- Existing patterns: `core/queue`, `core/server`
- CQRS terminology: Dispatch, Command, Handler
- Similar projects: MediatR (.NET), Symfony Messenger (PHP)
