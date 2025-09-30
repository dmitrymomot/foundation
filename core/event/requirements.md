# Event Publisher Requirements

## Overview

Event publisher implementation for publishing events with pluggable transport strategies. Events represent **facts/notifications** with one-to-many handler relationships (one event = multiple handlers).

**Follows Watermill-inspired architecture** like `core/command` package for consistency.

## Architecture (Watermill-Inspired)

Following the same pattern as `core/command`:

- **Publisher** = Stateless client (like Watermill's Publisher)
- **Transport** = Passive wire (like Watermill's Subscriber - provides channel)
- **Processor** = Active router (like Watermill's Router - manages workers)

```
┌───────────┐                                    ┌────────────┐
│ Publisher │──Publish()──▶ Transport ──────▶   │ Processor  │
│(stateless)│               (wire)               │(has Run()) │
└───────────┘                                    └────────────┘
                                                       │
                                                Manages workers
                                                Executes handlers
```

## Core Concepts

### Event Semantics

- **Fact-based**: Events are notifications that something happened (UserCreated, OrderPlaced)
- **One-to-many**: Each event can have zero or more handlers
- **Fan-out pattern**: All registered handlers receive the event
- **Fire-and-forget**: Publisher doesn't wait for handler completion
- **Idempotent handlers**: Same event may be delivered multiple times

### Event vs Command

| Aspect            | Event                          | Command                    |
| ----------------- | ------------------------------ | -------------------------- |
| Intent            | Notification (happened)        | Order (do this)            |
| Tense             | Past (UserCreated)             | Imperative (CreateUser)    |
| Handlers          | Many (0+)                      | One (exactly 1)            |
| Pattern           | Fan-out (broadcast)            | Competitive (one executes) |
| Missing handler   | Warning, not error             | Error (ErrHandlerNotFound) |
| Duplicate handler | Allowed (many-to-many)         | Panic (enforces 1:1)       |

## Key Architecture Decisions

### 1. Zero Handlers Behavior

**Decision**: Zero handlers is valid, log warning only

```go
// Valid - logs warning but doesn't error
publisher.Publish(ctx, UserCreated{UserID: "123"})
```

**Optional strict mode**:
```go
processor := event.NewProcessor(
    transport,
    event.WithStrictHandlers(true), // error if zero handlers
)
```

### 2. Handler Isolation

**Decision**: Handler errors don't stop other handlers

```go
handler1() // succeeds
handler2() // FAILS
handler3() // still executes
```

All handler errors are collected and reported.

### 3. Type Safety

Use generics for type-safe handlers (same as commands):

```go
type UserCreated struct {
    UserID string
    Email  string
}

func onUserCreated(ctx context.Context, evt UserCreated) error {
    return cache.Invalidate(evt.UserID)
}

processor.Register(event.NewHandlerFunc(onUserCreated))
```

### 4. Transport Pattern (Watermill-Inspired)

**Transport is passive wire** - just provides channel:

```go
// PublisherTransport sends events
type PublisherTransport interface {
    Dispatch(ctx context.Context, eventName string, payload any) error
}

// ProcessorTransport provides event channel
type ProcessorTransport interface {
    // Subscribe returns channel of events to process
    // Returns nil for sync transports (special case)
    Subscribe(ctx context.Context) (<-chan envelope, error)
    Close() error
}
```

**Processor manages workers**:

```go
processor := event.NewProcessor(
    transport,
    event.WithWorkers(5),  // Processor controls workers!
    event.WithErrorHandler(errorHandler),
)
```

### 5. Middleware & Decorators

**Middleware** (cross-cutting, all handlers):
```go
processor := event.NewProcessor(
    transport,
    event.WithMiddleware(
        event.LoggingMiddleware(logger),
        metricsMiddleware,
    ),
)
```

**Decorators** (per-handler):
```go
handler := event.Decorate(
    event.NewHandlerFunc(notifyWebhook),
    event.Retry(3),
    event.Backoff(5, 100*time.Millisecond, 10*time.Second),
    event.Timeout(30*time.Second),
)
processor.Register(handler)
```

## Transport Specifications

### Sync Transport

**Passive, no split needed**:

```go
transport := event.NewSyncTransport()

processor := event.NewProcessor(transport)
processor.Register(handler1)
processor.Register(handler2)

go processor.Run(ctx) // Just blocks, no workers needed

// Can use processor.Publish() directly for sync transport
err := processor.Publish(ctx, UserCreated{UserID: "123"})
// Returns aggregated errors via errors.Join()
```

### Channel Transport

**Passive wire, processor manages workers**:

```go
// Passive transport - just a channel
transport := event.NewChannelTransport(100)

// Publisher (stateless client)
publisher := event.NewPublisher(transport)
publisher.Publish(ctx, UserCreated{}) // Returns immediately

// Processor (active manager with workers)
processor := event.NewProcessor(
    transport,
    event.WithWorkers(5),  // Processor controls workers
    event.WithErrorHandler(errorHandler),
)
processor.Register(handler1)
processor.Register(handler2)

go processor.Run(ctx) // Manages workers, blocks until shutdown
```

### Distributed Transport (Future)

**Same pattern**:

```go
// Web server (publisher only)
transport := redis.NewTransport("redis://localhost")
publisher := event.NewPublisher(transport)
publisher.Publish(ctx, UserCreated{})

// Worker service (processor only)
transport := redis.NewTransport("redis://localhost")
processor := event.NewProcessor(
    transport,
    event.WithWorkers(10),
)
processor.Register(handlers...)
processor.Run(ctx) // Transport Subscribe() polls Redis
```

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

// Sync transport
transport := event.NewSyncTransport()
processor := event.NewProcessor(transport)
processor.Register(event.NewHandlerFunc(invalidateCache))
processor.Register(event.NewHandlerFunc(updateMetrics))

go processor.Run(ctx)

// Direct publish (sync execution)
if err := processor.Publish(ctx, UserCreated{UserID: "123"}); err != nil {
    log.Fatal(err)
}
```

### Async Local with Multiple Handlers

```go
// Shared transport (passive wire)
transport := event.NewChannelTransport(100)

// Publisher (HTTP handlers)
publisher := event.NewPublisher(transport)
publisher.Publish(ctx, UserCreated{UserID: "123"}) // Fire and forget

// Processor (worker)
processor := event.NewProcessor(
    transport,
    event.WithWorkers(5),  // Processor manages workers
    event.WithErrorHandler(errorHandler),
    event.WithMiddleware(event.LoggingMiddleware(logger)),
)
processor.Register(event.NewHandlerFunc(invalidateCache))
processor.Register(event.NewHandlerFunc(updateMetrics))
processor.Register(event.NewHandlerFunc(notifyWebhook))

g, ctx := errgroup.WithContext(ctx)
g.Go(func() error {
    return processor.Run(ctx) // Starts workers, blocks until shutdown
})
```

### Event → Command Pattern

```go
// Event handler publishes command for competitive work
func onUserCreated(ctx context.Context, evt UserCreated) error {
    // This runs on all processor instances (broadcast)
    // But command executes competitively (one instance only)
    return cmdDispatcher.Dispatch(ctx, SendWelcomeEmail{
        UserID: evt.UserID,
        Email:  evt.Email,
    })
}

// Event processor (all instances receive event)
eventProcessor.Register(event.NewHandlerFunc(onUserCreated))

// Command processor (one instance executes command)
commandProcessor.Register(command.NewHandlerFunc(sendEmailHandler))
```

## Error Handling

### Sync Transport
All handler errors collected and returned via `errors.Join()`:
```go
err := processor.Publish(ctx, evt)
// err contains all handler errors
```

### Async Transports
Handler errors reported via callback on processor:
```go
processor := event.NewProcessor(
    transport,
    event.WithErrorHandler(func(ctx context.Context, evtName string, err error) {
        logger.Error("event handler failed", "event", evtName, "error", err)
    }),
)
```

## Lifecycle Management

### Sync Transport
No lifecycle needed:
```go
processor := event.NewProcessor(transport)
processor.Publish(ctx, evt) // Just works
```

### Async Transports
Standard lifecycle with Run():
```go
processor := event.NewProcessor(
    transport,
    event.WithWorkers(5),
    event.WithErrorHandler(errorHandler),
)
processor.Register(handlers...)

// Run() starts workers and blocks
if err := processor.Run(ctx); err != nil {
    log.Fatal(err)
}
```

### errgroup Pattern
```go
g, ctx := errgroup.WithContext(context.Background())

g.Go(func() error {
    return processor.Run(ctx)
})

if err := g.Wait(); err != nil {
    log.Fatal(err)
}
```

## Package Structure

```
core/event/
  - publisher.go        // Publisher type (stateless client)
  - processor.go        // Processor type (active manager)
  - transport.go        // Transport interfaces
  - handler.go          // Handler interface, NewHandlerFunc
  - middleware.go       // Middleware (processor only)
  - decorators.go       // Handler decorators
  - sync_transport.go   // Sync transport
  - channel_transport.go // Async local transport
  - utils.go            // Helpers
  - errors.go           // Package errors
  - doc.go              // Documentation
```

## Configuration Options

**Transport creation**:
```go
transport := event.NewSyncTransport()
transport := event.NewChannelTransport(bufferSize)
```

**Publisher** (no options):
```go
publisher := event.NewPublisher(transport)
```

**Processor options**:
```go
processor := event.NewProcessor(
    transport,
    event.WithWorkers(5),          // Worker count (processor manages)
    event.WithMiddleware(...),     // Immutable middleware
    event.WithErrorHandler(...),   // Error callback (async)
    event.WithStrictHandlers(true),// Validate zero handlers
    event.WithLogger(logger),      // Logger
)
```

## Key Differences from Commands

| Aspect                | Event                           | Command                    |
| --------------------- | ------------------------------- | -------------------------- |
| **Semantic**          | Notification (happened)         | Order (do this)            |
| **Handlers**          | Many (0+)                       | One (exactly 1)            |
| **Pattern**           | Fan-out (broadcast)             | Competitive (one executes) |
| **Missing handler**   | Warning only                    | Error                      |
| **Duplicate handler** | Allowed                         | Panic                      |
| **Worker control**    | Processor via WithWorkers()     | Processor via WithWorkers()|
| **Transport role**    | Passive wire (Subscribe)        | Passive wire (Subscribe)   |

**Shared patterns** (consistent between both):
- Watermill-inspired architecture (Publisher/Dispatcher, Transport, Processor)
- Transport is passive wire (Subscribe pattern)
- Processor manages workers and lifecycle
- Generic HandlerFunc[T] with reflection
- Immutable middleware at construction
- Decorators with Decorate() helper
- Unified panic recovery
- Same transport types (Sync, Channel)

## Best Practices

1. **Event types should be self-contained** with all needed data
2. **Use sync transport for testing** (deterministic)
3. **Split Publisher and Processor** for async transports
4. **Publisher is stateless** - just publishes
5. **Processor manages lifecycle** - use Run(ctx)
6. **Processor controls workers** via WithWorkers()
7. **Always provide WithErrorHandler** for async transports
8. **Configure middleware at construction** (immutable)
9. **Make handlers idempotent** - events may be delivered multiple times
10. **Use commands for competitive work**, events for broadcasting
11. **Transport is passive wire** - Subscribe() provides channel
12. **Let processor manage workers** - not transport

## Implementation Checklist

**Core types**:
- [ ] `Publisher` type (stateless client)
- [ ] `Processor` type (active manager with Run())
- [ ] Generic `HandlerFunc[T]`
- [ ] `Handler` interface

**Transport layer** (Watermill pattern):
- [ ] `PublisherTransport.Dispatch(ctx, name, payload) error`
- [ ] `ProcessorTransport.Subscribe(ctx) (<-chan envelope, error)` - returns channel
- [ ] `ProcessorTransport.Close() error`
- [ ] Sync transport (Subscribe returns nil)
- [ ] Channel transport (Subscribe returns channel)

**Processor options**:
- [ ] `WithWorkers(n)` - processor controls workers
- [ ] `WithMiddleware()` - immutable
- [ ] `WithErrorHandler()` - async errors
- [ ] `WithStrictHandlers()` - validation
- [ ] `WithLogger()`

**Decorators**:
- [ ] `Decorator` type
- [ ] Factories: `Retry()`, `Backoff()`, `Timeout()`
- [ ] Helpers: `WithRetry()`, `WithBackoff()`, `WithTimeout()`
- [ ] `Decorate(handler, decorators...)`

**Lifecycle**:
- [ ] `Processor.Run(ctx) error` - manages workers, blocks
- [ ] Graceful shutdown on context cancel
- [ ] Workers drain events before exit

**Behavior**:
- [ ] Unified panic recovery
- [ ] Allow zero handlers (warn), allow duplicates
- [ ] FIFO handler execution
- [ ] Handler isolation (continue on error)
- [ ] `Stats()` method on Processor

**Documentation**:
- [ ] Comprehensive doc.go (Watermill-inspired)
- [ ] Black-box tests
- [ ] Examples for all patterns
