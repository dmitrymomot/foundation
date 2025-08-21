// Package middleware provides HTTP request/response logging middleware with flexible configuration.
//
// The logging middleware supports structured logging with fine-grained control over what gets logged,
// including request/response details, body content, headers, and performance tracking.
//
// # Basic Usage
//
// Use the default logging middleware with minimal configuration:
//
//	handler := middleware.Logging[handler.Context]()
//	wrappedHandler := handler(originalHandler)
//
// # Advanced Configuration Examples
//
// ## 1. Debugging Configuration (Request Body Logging)
//
//	loggingMiddleware := middleware.LoggingWithConfig[handler.Context](middleware.LoggingConfig{
//		LogRequestBody:  true,   // Log request body for debugging
//		LogResponseBody: true,   // Log response body for detailed tracing
//		MaxBodyLogSize:  8192,   // Increase max body log size to 8KB
//		LogLevel:        slog.LevelDebug,
//	})
//
// ## 2. Production Configuration (Sensitive Header Redaction)
//
//	loggingMiddleware := middleware.LoggingWithConfig[handler.Context](middleware.LoggingConfig{
//		LogHeaders:         true,
//		SensitiveHeaders:   []string{"Authorization", "X-API-Key", "Cookie"},
//		LogLevel:           slog.LevelInfo,
//		SlowRequestThreshold: 2 * time.Second, // Log slow requests taking more than 2 seconds
//	})
//
// ## 3. Skipping Health Check Endpoints
//
//	loggingMiddleware := middleware.LoggingWithConfig[handler.Context](middleware.LoggingConfig{
//		Skip: func(ctx handler.Context) bool {
//			return ctx.Request().URL.Path == "/health" ||
//			       ctx.Request().URL.Path == "/metrics"
//		},
//	})
//
// ## 4. Customizing Logging Behavior
//
// The logging middleware is highly configurable. You can control log levels,
// enable/disable specific logging features, and set performance thresholds.
package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/logger"
)

// LoggingConfig configures the request/response logging middleware.
// It provides fine-grained control over what gets logged and how.
type LoggingConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool

	// Logger is the slog logger to use (default: slog.Default())
	Logger *slog.Logger

	// LogLevel for request logging (default: slog.LevelInfo)
	LogLevel slog.Level

	// LogRequest enables logging of request details (default: true)
	LogRequest bool

	// LogResponse enables logging of response details (default: true)
	LogResponse bool

	// LogRequestBody enables logging of request body (default: false for security)
	LogRequestBody bool

	// LogResponseBody enables logging of response body (default: false for performance)
	LogResponseBody bool

	// LogHeaders enables logging of request/response headers (default: false for security)
	LogHeaders bool

	// MaxBodyLogSize is the maximum size of body to log in bytes (default: 4KB)
	MaxBodyLogSize int

	// SensitiveHeaders is a list of header names to redact (default: common auth headers)
	SensitiveHeaders []string

	// SlowRequestThreshold logs slow requests at warning level (default: 5s)
	SlowRequestThreshold time.Duration

	// Component name for structured logging
	Component string
}

// Logging creates a request/response logging middleware with default configuration.
// It logs basic request and response information at info level.
func Logging[C handler.Context]() handler.Middleware[C] {
	return LoggingWithConfig[C](LoggingConfig{})
}

// LoggingWithLogger creates a logging middleware with a custom logger.
func LoggingWithLogger[C handler.Context](log *slog.Logger) handler.Middleware[C] {
	return LoggingWithConfig[C](LoggingConfig{
		Logger: log,
	})
}

// LoggingWithConfig creates a request/response logging middleware with custom configuration.
// It provides detailed logging of HTTP requests and responses for debugging and monitoring.
func LoggingWithConfig[C handler.Context](cfg LoggingConfig) handler.Middleware[C] {
	// Set defaults
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.LogLevel == 0 {
		cfg.LogLevel = slog.LevelInfo
	}

	// Default to logging request and response (but not bodies)
	if !cfg.LogRequest && !cfg.LogResponse {
		cfg.LogRequest = true
		cfg.LogResponse = true
	}

	if cfg.MaxBodyLogSize <= 0 {
		cfg.MaxBodyLogSize = 4 * 1024 // 4KB default
	}

	if cfg.SensitiveHeaders == nil {
		cfg.SensitiveHeaders = []string{
			"Authorization",
			"Cookie",
			"Set-Cookie",
			"X-Api-Key",
			"X-Auth-Token",
			"X-Csrf-Token",
		}
	}

	if cfg.SlowRequestThreshold <= 0 {
		cfg.SlowRequestThreshold = 5 * time.Second
	}

	if cfg.Component == "" {
		cfg.Component = "http"
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			start := time.Now()
			req := ctx.Request()

			// Extract request ID if available
			requestID, _ := GetRequestID(ctx)

			// Build request attributes
			attrs := []slog.Attr{
				logger.Component(cfg.Component),
				logger.Event("request"),
				logger.Method(req.Method),
				logger.Path(req.URL.Path),
				logger.RemoteAddr(req.RemoteAddr),
			}

			if requestID != "" {
				attrs = append(attrs, logger.RequestID(requestID))
			}

			if req.URL.RawQuery != "" {
				attrs = append(attrs, logger.Query(req.URL.RawQuery))
			}

			// Log request body if enabled
			var requestBody []byte
			if cfg.LogRequestBody && req.Body != nil {
				requestBody, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(requestBody))

				if len(requestBody) > 0 {
					bodyToLog := requestBody
					if len(bodyToLog) > cfg.MaxBodyLogSize {
						bodyToLog = bodyToLog[:cfg.MaxBodyLogSize]
						attrs = append(attrs, slog.Bool("request_body_truncated", true))
					}
					attrs = append(attrs, slog.String("request_body", string(bodyToLog)))
				}
			}

			// Log headers if enabled
			if cfg.LogHeaders {
				headers := make(map[string]any)
				for key, values := range req.Header {
					if !slices.Contains(cfg.SensitiveHeaders, key) {
						if len(values) == 1 {
							headers[key] = values[0]
						} else {
							headers[key] = values
						}
					} else {
						headers[key] = "[REDACTED]"
					}
				}
				if len(headers) > 0 {
					attrs = append(attrs, slog.Any("request_headers", headers))
				}
			}

			// Log the request
			if cfg.LogRequest {
				cfg.Logger.LogAttrs(req.Context(), cfg.LogLevel, "HTTP request started", attrs...)
			}

			// Wrap response writer to capture status and size
			wrapped := &responseWriter{
				ResponseWriter: ctx.ResponseWriter(),
				statusCode:     http.StatusOK,
			}

			// Create a new context with the wrapped response writer
			// Note: This requires the context implementation to support this
			// For now, we'll process the response after

			response := next(ctx)

			// Execute the response to capture status
			capturedResponse := func(w http.ResponseWriter, r *http.Request) error {
				// Record start of response writing
				wrapped.ResponseWriter = w
				err := response(wrapped, r)

				duration := time.Since(start)

				// Build response attributes
				respAttrs := []slog.Attr{
					logger.Component(cfg.Component),
					logger.Event("response"),
					logger.Method(req.Method),
					logger.Path(req.URL.Path),
					logger.StatusCode(wrapped.statusCode),
					logger.BytesOut(int64(wrapped.size)),
					logger.Duration(duration),
				}

				if requestID != "" {
					respAttrs = append(respAttrs, logger.RequestID(requestID))
				}

				// Log response headers if enabled
				if cfg.LogHeaders && wrapped.headerWritten {
					headers := make(map[string]any)
					for key, values := range w.Header() {
						if !slices.Contains(cfg.SensitiveHeaders, key) {
							if len(values) == 1 {
								headers[key] = values[0]
							} else {
								headers[key] = values
							}
						} else {
							headers[key] = "[REDACTED]"
						}
					}
					if len(headers) > 0 {
						respAttrs = append(respAttrs, slog.Any("response_headers", headers))
					}
				}

				// Determine log level based on status and duration
				level := cfg.LogLevel
				if wrapped.statusCode >= 500 {
					level = slog.LevelError
					respAttrs = append(respAttrs, logger.Error(err))
				} else if wrapped.statusCode >= 400 {
					level = slog.LevelWarn
				} else if duration > cfg.SlowRequestThreshold {
					level = slog.LevelWarn
					respAttrs = append(respAttrs, slog.Bool("slow_request", true))
				}

				// Log the response
				if cfg.LogResponse {
					cfg.Logger.LogAttrs(req.Context(), level, "HTTP request completed", respAttrs...)
				}

				return err
			}

			return capturedResponse
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture response details
type responseWriter struct {
	http.ResponseWriter
	statusCode    int
	size          int
	headerWritten bool
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.headerWritten = true
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}
