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
dispatcher.Register(command.NewHandlerFunc(createUser))
dispatcher.Register(command.NewHandlerFunc(createUser)) // PANIC: duplicate handler
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

dispatcher.Register(command.NewHandlerFunc(createUser))
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
err := dispatcher.Dispatch(ctx, cmd)
// err is handler error or ErrHandlerNotFound
```

**Channel Transport**:

```go
err := dispatcher.Dispatch(ctx, cmd)
// err is enqueue error (ErrBufferFull)
// Handler errors handled via:
// - Error handler callback
// - Middleware logging
```

**Decision**: Provide error handler callback option

```go
dispatcher := command.NewDispatcher(
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
    command.NewHandlerFunc(createUserHandler),
    3, // maxRetries
)
dispatcher.Register(handler)
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
dispatcher.Use(command.LoggingMiddleware(logger))
dispatcher.Use(metricsMiddleware)
```

### 9. API Naming

**Decision**: Follow established patterns

- `Dispatch(ctx, cmd)` - execute command (standard CQRS terminology)
- `Register(handler)` - register command handler
- `Use(middleware)` - add middleware
- `Stop()` - graceful shutdown (channel transport only)

**Rationale**: Simple and clear API focused on command dispatching

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
dispatcher := command.NewDispatcher(command.WithSyncTransport())
dispatcher.Register(handler)

// No lifecycle management needed
err := dispatcher.Dispatch(ctx, cmd) // Blocks until complete
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
dispatcher := command.NewDispatcher(
    command.WithChannelTransport(100, command.WithWorkers(5)),
    command.WithErrorHandler(errorHandler),
)
defer dispatcher.Stop() // Graceful shutdown

dispatcher.Register(handler)

err := dispatcher.Dispatch(ctx, cmd) // Returns immediately
if err != nil {
    // ErrBufferFull or ErrHandlerNotFound
}
```

**Configuration options**:

- Buffer size (required)
- Number of workers (default: 1)
- Error handler (optional)

## Package Structure

```
core/command/
  - dispatcher.go        // Main Dispatcher type
  - transport.go         // Transport interface
  - handler.go           // Handler interface, HandlerFunc
  - middleware.go        // Middleware types and built-ins
  - sync_transport.go    // Sync transport implementation
  - channel_transport.go // Async local transport
  - decorators.go        // Retry, backoff decorators
  - errors.go            // Package errors
  - doc.go               // Package documentation
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

dispatcher := command.NewDispatcher(command.WithSyncTransport())
dispatcher.Register(command.NewHandlerFunc(createUserHandler))

if err := dispatcher.Dispatch(ctx, CreateUser{Email: "test@example.com"}); err != nil {
    return err
}
```

### Async Local with Error Handling

```go
dispatcher := command.NewDispatcher(
    command.WithChannelTransport(100),
    command.WithErrorHandler(func(ctx context.Context, cmdName string, err error) {
        logger.Error("command failed", "command", cmdName, "error", err)
        metrics.Inc("command.errors", "command", cmdName)
    }),
)
defer dispatcher.Stop()

dispatcher.Use(command.LoggingMiddleware(logger))
dispatcher.Register(command.NewHandlerFunc(sendEmailHandler))

// Fire and forget
dispatcher.Dispatch(ctx, SendEmail{To: "user@example.com"})
```

### With Retry Decorator

```go
handler := command.WithRetry(
    command.NewHandlerFunc(generateThumbnailHandler),
    3, // maxRetries
)
dispatcher.Register(handler)
dispatcher.Dispatch(ctx, GenerateThumbnail{ImageID: "123"})
```

## Upgrade Path

Developers can start simple and progressively add complexity:

```go
// Phase 1: Simple app, sync
dispatcher := command.NewDispatcher(command.WithSyncTransport())

// Phase 2: Need decoupling, async local
dispatcher := command.NewDispatcher(command.WithChannelTransport(100))
```

## Testing Considerations

**Sync transport is ideal for testing**:

- Deterministic execution
- No timing issues
- Synchronous errors
- No goroutine leaks

```go
func TestCreateUser(t *testing.T) {
    dispatcher := command.NewDispatcher(command.WithSyncTransport())
    dispatcher.Register(command.NewHandlerFunc(createUserHandler))

    err := dispatcher.Dispatch(ctx, CreateUser{Email: "test@example.com"})
    require.NoError(t, err)

    // Assertions run immediately after
    assertUserExists(t, "test@example.com")
}
```

## Implementation Decisions

1. **Sync transport lifecycle**: No Start/Stop methods - stateless by design
2. **Default transport**: Sync transport (simplest, most efficient)
3. **Channel transport workers**: Configurable via `WithWorkers(n)` option (default: 1)
4. **Middleware scope**: Applies globally to all handlers regardless of transport
5. **Panic recovery**: Built into channel transport, logged and passed to error handler

## References

- Existing patterns: `core/queue`, `core/server`
- CQRS terminology: Dispatch, Command, Handler
- Similar projects: MediatR (.NET), Symfony Messenger (PHP)
