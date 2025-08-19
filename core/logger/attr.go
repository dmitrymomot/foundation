package logger

import (
	"log/slog"
	"runtime"
	"strconv"
	"time"
)

// Attribute helpers use the empty Attr pattern for nil safety.
// This allows calls like log.Info("msg", logger.Error(err)) without explicit nil checks,
// following the principle of making zero values useful.

// Group creates a group of attributes under a single key.
func Group(name string, attrs ...slog.Attr) slog.Attr {
	return slog.Attr{Key: name, Value: slog.GroupValue(attrs...)}
}

// ============================================================================
// Error Handling
// ============================================================================

// Errors groups multiple non-nil errors under the key "errors".
// Uses index-based keys to preserve error order. Returns empty Attr for all nil errors.
func Errors(errs ...error) slog.Attr {
	// Count non-nil errors first to allocate exact size
	count := 0
	for _, err := range errs {
		if err != nil {
			count++
		}
	}
	if count == 0 {
		return slog.Attr{}
	}

	as := make([]slog.Attr, 0, count)
	for i, err := range errs {
		if err != nil {
			as = append(as, slog.Any(strconv.Itoa(i), err))
		}
	}
	return slog.Attr{Key: "errors", Value: slog.GroupValue(as...)}
}

// Error creates an attribute for a single error under the key "error".
// Returns empty Attr for nil errors, enabling safe usage without nil checks.
func Error(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.Any("error", err)
}

// ============================================================================
// Performance and Timing
// ============================================================================

// Duration creates an attribute for a duration.
func Duration(d time.Duration) slog.Attr {
	return slog.Duration("duration", d)
}

// Latency is an alias for Duration, commonly used in web contexts.
func Latency(d time.Duration) slog.Attr {
	return slog.Duration("latency", d)
}

// Elapsed calculates and logs the duration since the start time.
func Elapsed(start time.Time) slog.Attr {
	return slog.Duration("elapsed", time.Since(start))
}

// ============================================================================
// Generic Identifiers
// ============================================================================

// ID creates a generic identifier attribute with a custom key.
func ID(key string, value any) slog.Attr {
	if value == nil {
		return slog.Attr{}
	}
	return slog.Any(key, value)
}

// RequestID creates an attribute for HTTP request IDs.
func RequestID(id string) slog.Attr {
	if id == "" {
		return slog.Attr{}
	}
	return slog.String("request_id", id)
}

// TraceID creates an attribute for distributed tracing IDs.
func TraceID(id string) slog.Attr {
	if id == "" {
		return slog.Attr{}
	}
	return slog.String("trace_id", id)
}

// CorrelationID creates an attribute for correlation IDs.
func CorrelationID(id string) slog.Attr {
	if id == "" {
		return slog.Attr{}
	}
	return slog.String("correlation_id", id)
}

// ============================================================================
// Network and HTTP
// ============================================================================

// Method creates an attribute for HTTP methods.
func Method(method string) slog.Attr {
	return slog.String("method", method)
}

// Path creates an attribute for URL paths.
func Path(path string) slog.Attr {
	return slog.String("path", path)
}

// StatusCode creates an attribute for HTTP status codes.
func StatusCode(code int) slog.Attr {
	return slog.Int("status_code", code)
}

// ClientIP creates an attribute for client IP addresses.
func ClientIP(ip string) slog.Attr {
	return slog.String("client_ip", ip)
}

// UserAgent creates an attribute for user agent strings.
func UserAgent(ua string) slog.Attr {
	return slog.String("user_agent", ua)
}

// BytesIn creates an attribute for incoming bytes.
func BytesIn(n int64) slog.Attr {
	return slog.Int64("bytes_in", n)
}

// BytesOut creates an attribute for outgoing bytes.
func BytesOut(n int64) slog.Attr {
	return slog.Int64("bytes_out", n)
}

// ============================================================================
// Generic Metadata
// ============================================================================

// Component creates an attribute for component names.
func Component(name string) slog.Attr {
	return slog.String("component", name)
}

// Event creates an attribute for event names.
func Event(name string) slog.Attr {
	return slog.String("event", name)
}

// Type creates an attribute for type classification.
func Type(t string) slog.Attr {
	return slog.String("type", t)
}

// Action creates an attribute for action names.
func Action(action string) slog.Attr {
	return slog.String("action", action)
}

// Result creates an attribute for operation results (success/failure/pending).
func Result(result string) slog.Attr {
	return slog.String("result", result)
}

// Count creates a generic counter attribute.
func Count(key string, n int) slog.Attr {
	return slog.Int(key, n)
}

// Version creates an attribute for version information.
func Version(v string) slog.Attr {
	return slog.String("version", v)
}

// Key creates a generic key-value attribute.
func Key(key string, value any) slog.Attr {
	if value == nil {
		return slog.Attr{}
	}
	return slog.Any(key, value)
}

// RetryCount creates an attribute for retry attempts.
func RetryCount(count int) slog.Attr {
	return slog.Int("retry_count", count)
}

// ============================================================================
// Debugging
// ============================================================================

// Stack captures and returns the current stack trace.
func Stack() slog.Attr {
	const size = 64 << 10
	buf := make([]byte, size)
	buf = buf[:runtime.Stack(buf, false)]
	return slog.String("stack", string(buf))
}

// Caller returns information about the calling function.
func Caller() slog.Attr {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return slog.Attr{}
	}
	return slog.String("caller", file+":"+strconv.Itoa(line))
}
