# Logger Package

A context-aware logging utility built on Go's `log/slog` with generic attribute helpers for structured logging.

## Overview

The package provides:
1. A context-aware `slog` wrapper with automatic value extraction
2. Generic attribute helpers for common logging scenarios (HTTP, performance, debugging)
3. Zero-value safety - nil values return empty attributes instead of panicking

## Features

- **Context Extraction**: Automatically extract values from `context.Context`
- **Output Formats**: JSON or text with development/production presets
- **Generic Helpers**: Framework-agnostic attribute helpers for common patterns
- **Nil Safety**: All helpers handle nil/empty values gracefully
- **Type Safety**: Strongly typed helpers prevent logging errors

## Installation

```go
import "github.com/dmitrymomot/gokit/pkg/logger"
```

## Usage

### Basic Setup

```go
// Development logger with text output
log := logger.New(
    logger.WithDevelopment("my-service"),
)

// Production logger with JSON output
log := logger.New(
    logger.WithProduction("api-service"),
)

// Set as default slog logger
logger.SetAsDefault(log)
```

### Context Extraction

Extract values from context automatically:

```go
type ctxKey string
var requestIDKey ctxKey = "request-id"

log := logger.New(
    logger.WithProduction("api"),
    logger.WithContextValue("request_id", requestIDKey),
)

ctx := context.WithValue(context.Background(), requestIDKey, "req-123")
log.InfoContext(ctx, "processing request") // Automatically includes request_id
```

## Generic Attribute Helpers

The package provides generic helpers organized by category:

### Error Handling

```go
// Single error
log.Error("operation failed", logger.Error(err))

// Multiple errors
log.Error("startup failed", 
    logger.Errors(dbErr, cacheErr, configErr))
```

### Performance & Timing

```go
start := time.Now()
// ... do work ...

log.Info("request completed",
    logger.Duration(5*time.Second),      // Explicit duration
    logger.Latency(responseTime),        // Web-specific alias
    logger.Elapsed(start),                // Calculate from start time
)
```

### HTTP & Network

```go
log.Info("request",
    logger.Method("POST"),
    logger.Path("/api/users"),
    logger.StatusCode(201),
    logger.ClientIP("192.168.1.1"),
    logger.UserAgent("Mozilla/5.0"),
    logger.BytesIn(1024),
    logger.BytesOut(2048),
)
```

### Generic Identifiers

```go
log.Info("processing",
    logger.RequestID("req-123"),         // HTTP request ID
    logger.TraceID("trace-456"),         // Distributed tracing
    logger.CorrelationID("corr-789"),    // Event correlation
    logger.ID("user_id", userID),        // Custom identifier
)
```

### Metadata

```go
log.Info("event",
    logger.Component("auth"),             // System component
    logger.Event("user_login"),           // Event name
    logger.Type("notification"),          // Classification
    logger.Action("create"),              // Action performed
    logger.Result("success"),             // Operation result
    logger.Count("attempts", 3),          // Generic counter
    logger.Version("1.2.3"),              // Version info
    logger.RetryCount(2),                 // Retry attempts
)
```

### Debugging

```go
// Capture stack trace for errors
log.Error("panic recovered",
    logger.Error(err),
    logger.Stack(),                      // Full stack trace
    logger.Caller(),                     // Calling function info
)
```

### Generic Key-Value

```go
// For arbitrary key-value pairs
log.Info("custom event",
    logger.Key("custom_field", value),
    logger.ID("transaction_id", txID),
)
```

### Grouping

```go
// Group related attributes
log.Info("user action",
    logger.Group("request",
        slog.String("id", "123"),
        slog.String("method", "POST"),
    ),
)
```

## Creating Domain-Specific Helpers

While this package provides generic helpers, you can easily create domain-specific helpers in your application:

```go
package myapp

import (
    "log/slog"
    "github.com/dmitrymomot/gokit/pkg/logger"
)

// Domain-specific helpers
func UserID(id string) slog.Attr {
    return slog.String("user_id", id)
}

func TenantID(id string) slog.Attr {
    return slog.String("tenant_id", id)
}

func OrderID(id string) slog.Attr {
    return slog.String("order_id", id)
}

// Usage
log.Info("order processed",
    UserID(user.ID),                    // Your domain helper
    TenantID(tenant.ID),                 // Your domain helper
    OrderID(order.ID),                   // Your domain helper
    logger.Duration(elapsed),            // Generic helper
    logger.Result("success"),            // Generic helper
)
```

## Integration with HTTP Middleware

Example middleware using generic helpers:

```go
func LoggingMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Generate request ID
            requestID := uuid.New().String()
            
            // Log request
            log.Info("request started",
                logger.RequestID(requestID),
                logger.Method(r.Method),
                logger.Path(r.URL.Path),
                logger.ClientIP(getClientIP(r)),
                logger.UserAgent(r.UserAgent()),
            )
            
            // Wrap response writer to capture status
            wrapped := &responseWriter{ResponseWriter: w}
            
            // Process request
            next.ServeHTTP(wrapped, r)
            
            // Log response
            log.Info("request completed",
                logger.RequestID(requestID),
                logger.StatusCode(wrapped.status),
                logger.Elapsed(start),
                logger.BytesOut(int64(wrapped.written)),
            )
        })
    }
}
```

## Design Philosophy

This package follows these principles:

1. **Generic over Specific**: Provides universally useful helpers, not domain-specific ones
2. **Nil Safety**: All helpers handle nil/empty values gracefully
3. **Framework Agnostic**: Can be used with any Go application or framework
4. **Composable**: Helpers can be combined with custom domain helpers
5. **Performance**: Minimal overhead through efficient attribute creation

## Migration from Domain-Specific Helpers

If you were using domain-specific helpers like `UserID`, `WorkspaceID`, etc., migrate by:

1. Create your own domain helpers (as shown above)
2. Use generic `logger.ID()` for flexible identifiers
3. Leverage generic helpers for common patterns (HTTP, timing, errors)

This approach gives you the best of both worlds: generic utilities from the package and domain-specific helpers tailored to your application.