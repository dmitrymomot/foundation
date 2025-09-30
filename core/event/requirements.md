# Event Bus Requirements

## Overview

Event bus implementation for publishing events with pluggable transport strategies. Events represent **facts/notifications** with one-to-many handler relationships (one event = multiple handlers).

## Core Concepts

### Event Semantics
- **Fact-based**: Events are notifications that something happened (UserCreated, OrderPlaced)
- **One-to-many**: Each event can have zero or more handlers
- **Fan-out pattern**: All registered handlers receive the event
- **Fire-and-forget**: Publisher doesn't wait for handler completion
- **Idempotent handlers**: Same event may be delivered multiple times

### Event vs Command
Clear semantic distinction:

| Aspect | Event | Command |
|--------|-------|---------|
| Intent | Notification (happened) | Order (do this) |
| Tense | Past (UserCreated) | Imperative (CreateUser) |
| Handlers | Many (0+) | One (exactly 1) |
| Pattern | Fan-out (broadcast) | Competitive (one executes) |
| Missing handler | Warning, not error | Error |
| Use case | State sync, cache invalidation | Work distribution, actions |

### Transport Abstraction
Similar to commands, but always fan-out semantics:

| Transport | Execution | Blocking | Use Case |
|-----------|-----------|----------|----------|
| **Sync** | Direct call | Yes | Testing, simple apps |
| **Channel** | Goroutine + buffer | No | Local async, same instance |
| **Distributed** | Pub/Sub (Redis, NATS) | No | Multi-instance, broadcast |

**Important**: For competitive work (one instance processes), use commands instead.

## Architecture Decisions

### 1. Zero Handlers Behavior
**Decision**: Zero handlers is valid, log warning only

**Rationale**:
- Events are facts - whether anyone listens is separate concern
- Partial deployments: Instance A has handler, Instance B doesn't
- Future extensibility: Publish event now, add handlers later
- Prevents coupling publisher to consumer existence

```go
// Valid - logs warning but doesn't error
bus.Publish(ctx, UserCreated{UserID: "123"})
// Log: "no handlers registered for event: UserCreated"
```

**Optional strict mode**:
```go
bus := event.New(
    event.WithStrictHandlers(true), // error if zero handlers
)
```

### 2. Handler Registration Order
**Decision**: Handlers execute in FIFO registration order

**Rationale**:
- Deterministic behavior
- Predictable execution
- Easy to reason about
- Documented guarantee

```go
bus.Register(event.HandlerFunc(invalidateCache))  // Runs first
bus.Register(event.HandlerFunc(updateMetrics))    // Runs second
bus.Register(event.HandlerFunc(notifyWebhook))    // Runs third
```

**Note**: Order matters for dependencies (e.g., cache invalidation before metrics)

### 3. Handler Isolation
**Decision**: Handler errors don't stop other handlers

**Rationale**:
- One failing handler shouldn't prevent others from executing
- Better system resilience
- Failures are logged/tracked independently

```go
handler1() // succeeds
handler2() // FAILS
handler3() // still executes
```

**Error collection**: All handler errors are collected and returned/logged

### 4. Duplicate Handlers
**Decision**: Allow duplicate handler registration

**Rationale**:
- Unlike commands (one-to-one), events are one-to-many
- User might want same handler multiple times (e.g., different configs)
- User responsibility to manage

**Alternative**: Panic on duplicate if `WithStrictHandlers(true)`

### 5. Type Safety
**Decision**: Use generics for type-safe handlers (same as commands)

```go
func onUserCreated(ctx context.Context, evt UserCreated) error {
    // evt is strongly typed
    return cache.Invalidate(evt.UserID)
}

bus.Register(event.HandlerFunc(onUserCreated))
```

### 6. Serialization
**Decision**: Use standard library `encoding/json` (same as commands)

### 7. Context Usage
**Decision**: All handlers receive `context.Context` (same as commands)

**Note**: Async transports create fresh context for handlers

### 8. Middleware Support
**Decision**: Support middleware chain (same as commands)

**Built-in middleware**:
- Logging (default, requires `*slog.Logger`)

```go
bus.Use(event.LoggingMiddleware(logger))
bus.Use(event.MetricsMiddleware(metrics))
```

### 9. API Naming
**Decision**: Follow event-driven conventions

- `Publish(ctx, evt)` - publish event to all handlers
- `Start(ctx)` - begin processing (async transports)
- `Stop()` - graceful shutdown
- `Run(ctx) func() error` - errgroup compatibility

**Rationale**: `Publish` is standard event bus terminology

### 10. Competitive Work Pattern
**Decision**: Use commands for competitive work, not events

**Problem scenario**:
```go
// WRONG: Using event for work that should happen once
OnUserCreated -> SendWelcomeEmail() // Runs on ALL instances!
```

**Correct approach**:
```go
// Event handler publishes command for competitive work
func onUserCreated(ctx context.Context, evt UserCreated) error {
    // This handler runs on all instances, but command is competitive
    return commandBus.Dispatch(ctx, SendWelcomeEmail{
        UserID: evt.UserID,
    })
}

// Command handler executes once via queue transport
func sendEmailHandler(ctx context.Context, cmd SendWelcomeEmail) error {
    return mailer.Send(...)
}
```

**Rationale**:
- Events = fan-out (all instances)
- Commands = competitive (one instance)
- Clear separation of concerns
- Aligns with existing `core/queue` (competitive)

## Transport Specifications

### Sync Transport

**Characteristics**:
- Direct function call
- All handlers execute sequentially
- Blocks until all handlers complete
- Returns aggregated errors

**Use cases**:
- Testing (deterministic)
- Simple applications
- Transaction boundaries

**API**:
```go
bus := event.New(event.WithSyncTransport())
bus.Register(handler1)
bus.Register(handler2)

// Blocks until all handlers complete
err := bus.Publish(ctx, UserCreated{UserID: "123"})
if err != nil {
    // Aggregated handler errors (if any)
}
```

### Channel Transport (Async Local)

**Characteristics**:
- Non-blocking publish
- Buffered channel
- Handlers execute on same instance
- No persistence
- Error handling via callback/middleware

**Use cases**:
- Local async notifications
- Cache invalidation across goroutines
- Decoupling within instance

**API**:
```go
bus := event.New(
    event.WithChannelTransport(bufferSize: 100),
    event.WithErrorHandler(errorHandler),
)
bus.Register(handler)

go bus.Start(ctx)

err := bus.Publish(ctx, UserCreated{UserID: "123"}) // Returns immediately
if err != nil {
    // ErrBufferFull
}
```

### Distributed Transport (Pub/Sub)

**Characteristics**:
- Non-blocking publish
- All instances receive event
- Persistent (depends on infrastructure)
- Fan-out pattern

**Use cases**:
- Cache invalidation across instances
- State synchronization
- Metrics collection
- Multi-instance notifications

**Implementation options**:
- Redis Pub/Sub
- NATS
- Kafka (with unique consumer group per instance)

**API**:
```go
// All instances
bus := event.New(event.WithRedisPubSubTransport(client))
bus.Register(event.HandlerFunc(invalidateCache))
bus.Register(event.HandlerFunc(updateMetrics))
bus.Start(ctx)

// Any instance can publish
bus.Publish(ctx, UserCreated{UserID: "123"})
// All instances receive and process
```

## Package Structure

```
core/event/
  - bus.go              // Main Bus type
  - transport.go        // Transport interface
  - handler.go          // Handler interface, HandlerFunc
  - middleware.go       // Middleware types and built-ins
  - sync_transport.go   // Sync transport implementation
  - channel_transport.go // Async local transport
  - errors.go           // Package errors
  - doc.go              // Package documentation
```

**Note**: Flat structure, no sub-folders
**Note**: Distributed transports (Redis, NATS) can be added later or kept in separate packages

## Usage Examples

### Simple Sync Example
```go
type UserCreated struct {
    UserID string
    Email  string
}

func invalidateCache(ctx context.Context, evt UserCreated) error {
    return cache.Delete(ctx, "user:"+evt.UserID)
}

func updateMetrics(ctx context.Context, evt UserCreated) error {
    metrics.Inc("users.created")
    return nil
}

bus := event.New(event.WithSyncTransport())
bus.Register(event.HandlerFunc(invalidateCache))
bus.Register(event.HandlerFunc(updateMetrics))

if err := bus.Publish(ctx, UserCreated{UserID: "123"}); err != nil {
    // Handle aggregated errors
}
```

### Async Local with Multiple Handlers
```go
bus := event.New(
    event.WithChannelTransport(100),
    event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
        logger.Error("event handler failed",
            "event", evtName,
            "error", err)
    }),
)

bus.Use(event.LoggingMiddleware(logger))

bus.Register(event.HandlerFunc(invalidateCache))
bus.Register(event.HandlerFunc(updateMetrics))
bus.Register(event.HandlerFunc(notifyWebhook))

go bus.Start(ctx)

// Fire and forget - all handlers will be called
bus.Publish(ctx, UserCreated{UserID: "123"})
```

### Event → Command Pattern
```go
// Event handler publishes command for competitive work
type EmailDispatcher struct {
    commandBus *command.Bus
}

func (h *EmailDispatcher) OnUserCreated(ctx context.Context, evt UserCreated) error {
    // This runs on all instances, but command is competitive
    return h.commandBus.Dispatch(ctx, SendWelcomeEmail{
        UserID: evt.UserID,
        Email:  evt.Email,
    })
}

// Register event handler
eventBus.Register(event.HandlerFunc(emailDispatcher.OnUserCreated))

// Command handler (on worker instance)
commandBus.Register(command.HandlerFunc(sendEmailHandler))
```

### Testing with Sync Transport
```go
func TestUserCreatedEvent(t *testing.T) {
    var invalidateCalled, metricsCalled bool

    bus := event.New(event.WithSyncTransport())

    bus.Register(event.HandlerFunc(func(ctx context.Context, evt UserCreated) error {
        invalidateCalled = true
        return nil
    }))

    bus.Register(event.HandlerFunc(func(ctx context.Context, evt UserCreated) error {
        metricsCalled = true
        return nil
    }))

    err := bus.Publish(ctx, UserCreated{UserID: "123"})
    require.NoError(t, err)

    // Synchronous - can assert immediately
    assert.True(t, invalidateCalled)
    assert.True(t, metricsCalled)
}
```

## Handler Error Handling

### Sync Transport
All handler errors collected and returned:

```go
err := bus.Publish(ctx, evt)
// err contains all handler errors via errors.Join()
```

### Async Transports
Handler errors cannot return to publisher:

**Options**:
1. Error handler callback (configured on bus)
2. Logging middleware (logs all errors)
3. Metrics middleware (tracks error counts)

```go
bus := event.New(
    event.WithChannelTransport(100),
    event.WithErrorHandler(func(ctx context.Context, evtName string, handlerIdx int, err error) {
        logger.Error("event handler failed",
            "event", evtName,
            "handler_index", handlerIdx,
            "error", err)

        // Optional: custom logic (alert, retry, etc.)
    }),
)
```

## Lifecycle Management

### Sync Transport
No lifecycle needed:

```go
bus := event.New(event.WithSyncTransport())
bus.Register(handlers...)
bus.Publish(ctx, evt) // Just works
```

### Async Transports
Require Start/Stop:

```go
bus := event.New(event.WithChannelTransport(100))
bus.Register(handlers...)

ctx, cancel := context.WithCancel(context.Background())

go func() {
    if err := bus.Start(ctx); err != nil {
        log.Fatal(err)
    }
}()

// Publish events...
bus.Publish(ctx, evt)

// Graceful shutdown
cancel()
if err := bus.Stop(); err != nil {
    log.Error("shutdown error", err)
}
```

### errgroup Pattern
```go
g, ctx := errgroup.WithContext(context.Background())
g.Go(bus.Run(ctx))
// ...
if err := g.Wait(); err != nil {
    log.Fatal(err)
}
```

## Observability

### Built-in Metrics (via middleware)
- Events published count
- Handler execution count
- Handler error count
- Handler execution duration

### Logging (default middleware)
```
level=INFO msg="event published" event=UserCreated handlers=3
level=INFO msg="event handler completed" event=UserCreated handler=InvalidateCache duration=5ms
level=ERROR msg="event handler failed" event=UserCreated handler=NotifyWebhook error="connection timeout"
```

## Testing Strategies

### Unit Testing (Sync Transport)
```go
bus := event.New(event.WithSyncTransport())
// Register test handlers
// Publish event
// Assert synchronously
```

### Integration Testing (Channel Transport)
```go
bus := event.New(event.WithChannelTransport(10))
go bus.Start(ctx)

bus.Publish(ctx, evt)

// Wait for processing with stats
stats := bus.Stats()
require.Eventually(t, func() bool {
    return stats.EventsProcessed > 0
}, time.Second, 10*time.Millisecond)
```

### Mocking
```go
type MockEventBus struct {
    PublishedEvents []any
}

func (m *MockEventBus) Publish(ctx context.Context, evt any) error {
    m.PublishedEvents = append(m.PublishedEvents, evt)
    return nil
}
```

## Configuration Options

```go
event.New(
    // Transport (required, one of)
    event.WithSyncTransport(),
    event.WithChannelTransport(bufferSize),
    event.WithRedisPubSubTransport(client),

    // Error handling
    event.WithErrorHandler(handler),

    // Validation
    event.WithStrictHandlers(true), // error on zero handlers

    // Logging
    event.WithLogger(logger),
)
```

## Upgrade Path

```go
// Phase 1: Simple app, sync
bus := event.New(event.WithSyncTransport())

// Phase 2: Async within instance
bus := event.New(event.WithChannelTransport(100))

// Phase 3: Multi-instance broadcasting
bus := event.New(event.WithRedisPubSubTransport(client))
```

## Anti-Patterns

### ❌ Using Events for Competitive Work
```go
// WRONG: Email sent by ALL instances
OnUserCreated -> SendEmail()
```

**Fix**: Use command bus for competitive work

### ❌ Depending on Handler Order (Without Documentation)
```go
// WRONG: Implicit dependency on execution order
bus.Register(handler1) // Must run first (undocumented)
bus.Register(handler2) // Depends on handler1
```

**Fix**: Document order dependency or make handlers independent

### ❌ Blocking Operations in Handlers
```go
// WRONG: Long-running work blocks other handlers
func onUserCreated(ctx context.Context, evt UserCreated) error {
    processImageForHours() // Blocks!
    return nil
}
```

**Fix**: Publish command for long-running work

### ❌ Expecting Synchronous Results
```go
// WRONG: Async bus doesn't return handler results
bus.Publish(ctx, evt)
result := ??? // No way to get handler result
```

**Fix**: Use sync transport for testing, or use command bus for request-response

## Open Questions

1. Should distributed transport implementations be in this package or separate?
2. Handler timeout configuration - per-handler or global?
3. Handler panic recovery strategy?
4. Should middleware be able to filter/modify events?
5. Event versioning strategy for schema evolution?

## References

- Event-driven architecture patterns
- Pub/Sub semantics
- Fan-out vs competitive patterns
- Existing patterns: `core/queue`, `core/server`