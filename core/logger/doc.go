// Package logger provides structured logging utilities built on Go's standard slog package.
// It offers enhanced functionality including context-aware attribute extraction,
// environment-specific configurations, and a comprehensive set of pre-built attributes
// for common logging scenarios.
//
// # Features
//
//   - Built on Go's standard slog for compatibility and performance
//   - Context-aware attribute extraction for request-scoped data
//   - Environment-specific configurations (development, staging, production)
//   - Comprehensive attribute helpers for common logging patterns
//   - Support for both JSON and text output formats
//   - Handler decoration for automatic context attribute injection
//   - Zero-allocation patterns for high-performance logging
//   - Type-safe attribute creation with nil safety
//
// # Basic Usage
//
// Create loggers using the factory function with various configuration options:
//
//	import "github.com/dmitrymomot/foundation/core/logger"
//
//	// Create a development logger
//	log := logger.New(
//		logger.WithDevelopment("myapp"),
//		logger.WithLevel(slog.LevelDebug),
//	)
//
//	// Create a production logger
//	log := logger.New(
//		logger.WithProduction("myapp"),
//		logger.WithJSONFormatter(),
//	)
//
//	// Use the logger
//	log.Info("Server starting",
//		logger.Component("server"),
//		logger.Event("startup"),
//	)
//
// # Environment Configurations
//
// The package provides pre-configured setups for different environments:
//
//	// Development: text format, debug level, stdout
//	devLogger := logger.New(logger.WithDevelopment("myapp"))
//
//	// Production: JSON format, info level, stdout
//	prodLogger := logger.New(logger.WithProduction("myapp"))
//
//	// Staging: JSON format, info level, stdout
//	stageLogger := logger.New(logger.WithStaging("myapp"))
//
//	// Custom configuration
//	customLogger := logger.New(
//		logger.WithLevel(slog.LevelWarn),
//		logger.WithJSONFormatter(),
//		logger.WithAttr(slog.String("service", "api")),
//		logger.WithOutput(os.Stderr),
//	)
//
// # Context-Aware Logging
//
// Extract and inject attributes automatically from context values:
//
//	import "context"
//
//	// Create logger with context extractors
//	log := logger.New(
//		logger.WithProduction("myapp"),
//		logger.WithContextValue("request_id", "request_id"),
//		logger.WithContextValue("user_id", "user_id"),
//	)
//
//	// Set values in context
//	ctx := context.WithValue(context.Background(), "request_id", "req-12345")
//	ctx = context.WithValue(ctx, "user_id", "user-67890")
//
//	// Log with automatic context attribute injection
//	log.InfoContext(ctx, "Processing request")
//	// Output: {"level":"INFO","msg":"Processing request","request_id":"req-12345","user_id":"user-67890"}
//
// # Custom Context Extractors
//
// Define custom logic for extracting attributes from context:
//
//	func requestIDExtractor(ctx context.Context) (slog.Attr, bool) {
//		if req, ok := ctx.Value("request").(*http.Request); ok {
//			if id := req.Header.Get("X-Request-ID"); id != "" {
//				return logger.RequestID(id), true
//			}
//		}
//		return slog.Attr{}, false
//	}
//
//	func userExtractor(ctx context.Context) (slog.Attr, bool) {
//		if user, ok := ctx.Value("user").(*User); ok {
//			return logger.Group("user",
//				slog.String("id", user.ID),
//				slog.String("email", user.Email),
//			), true
//		}
//		return slog.Attr{}, false
//	}
//
//	// Create logger with custom extractors
//	log := logger.New(
//		logger.WithProduction("myapp"),
//		logger.WithContextExtractors(requestIDExtractor, userExtractor),
//	)
//
// # Attribute Helpers
//
// The package provides numerous helper functions for creating common attributes:
//
//	// Error handling
//	log.Error("Operation failed",
//		logger.Error(err),
//		logger.Action("database_query"),
//		logger.Component("user_service"),
//	)
//
//	// Multiple errors
//	log.Error("Multiple failures",
//		logger.Errors(err1, err2, err3),
//		logger.RetryCount(3),
//	)
//
//	// HTTP logging
//	log.Info("Request processed",
//		logger.Method("POST"),
//		logger.Path("/api/users"),
//		logger.StatusCode(201),
//		logger.ClientIP("192.168.1.1"),
//		logger.Latency(time.Since(start)),
//		logger.BytesIn(1024),
//		logger.BytesOut(512),
//	)
//
//	// Timing and performance
//	start := time.Now()
//	// ... do work ...
//	log.Info("Task completed",
//		logger.Duration(time.Since(start)),
//		logger.Elapsed(start),
//		logger.Component("worker"),
//	)
//
//	// Identifiers and tracing
//	log.Info("Processing payment",
//		logger.RequestID("req-12345"),
//		logger.TraceID("trace-67890"),
//		logger.CorrelationID("corr-abcde"),
//		logger.ID("payment_id", paymentID),
//	)
//
// # Structured Logging Patterns
//
// Create well-structured logs for different scenarios:
//
//	// Service startup
//	log.Info("Service initialized",
//		logger.Component("auth_service"),
//		logger.Event("startup"),
//		logger.Version("v1.2.3"),
//		logger.Group("config",
//			slog.String("port", "8080"),
//			slog.String("env", "production"),
//		),
//	)
//
//	// Database operations
//	log.Debug("Executing query",
//		logger.Component("database"),
//		logger.Action("select"),
//		logger.Key("table", "users"),
//		logger.Key("query", query),
//	)
//
//	// Background jobs
//	log.Info("Job completed",
//		logger.Component("job_processor"),
//		logger.Event("job_finished"),
//		logger.Type("email_notification"),
//		logger.Result("success"),
//		logger.Duration(time.Since(jobStart)),
//		logger.Count("emails_sent", emailCount),
//	)
//
// # Error Logging Best Practices
//
// Use the error helpers for consistent error logging:
//
//	func processUser(userID string) error {
//		user, err := getUserFromDB(userID)
//		if err != nil {
//			log.Error("Failed to fetch user",
//				logger.Error(err),
//				logger.ID("user_id", userID),
//				logger.Component("user_service"),
//				logger.Action("fetch_user"),
//			)
//			return err
//		}
//
//		if err := validateUser(user); err != nil {
//			log.Warn("User validation failed",
//				logger.Error(err),
//				logger.ID("user_id", userID),
//				logger.Component("validator"),
//			)
//			return err
//		}
//
//		return nil
//	}
//
// # HTTP Request Logging
//
// Create comprehensive HTTP request logs:
//
//	func logHTTPRequest(r *http.Request, status int, duration time.Duration, bytes int64) {
//		log.Info("HTTP request processed",
//			logger.Method(r.Method),
//			logger.Path(r.URL.Path),
//			logger.StatusCode(status),
//			logger.ClientIP(getClientIP(r)),
//			logger.UserAgent(r.UserAgent()),
//			logger.Latency(duration),
//			logger.BytesOut(bytes),
//			logger.Group("headers",
//				slog.String("accept", r.Header.Get("Accept")),
//				slog.String("content_type", r.Header.Get("Content-Type")),
//			),
//		)
//	}
//
// # Performance Monitoring
//
// Log performance metrics and timing information:
//
//	func monitorOperation(name string, fn func() error) error {
//		start := time.Now()
//		log.Debug("Operation starting",
//			logger.Event("operation_start"),
//			logger.Action(name),
//		)
//
//		err := fn()
//		duration := time.Since(start)
//
//		if err != nil {
//			log.Error("Operation failed",
//				logger.Error(err),
//				logger.Action(name),
//				logger.Duration(duration),
//				logger.Result("failure"),
//			)
//		} else {
//			log.Info("Operation completed",
//				logger.Action(name),
//				logger.Duration(duration),
//				logger.Result("success"),
//			)
//		}
//
//		return err
//	}
//
// # Debugging and Diagnostics
//
// Use debugging helpers for troubleshooting:
//
//	func debugFunction() {
//		log.Debug("Debug information",
//			logger.Stack(),        // Full stack trace
//			logger.Caller(),       // Calling function info
//			logger.Component("debug"),
//		)
//	}
//
//	// Conditional debugging
//	if log.Enabled(context.Background(), slog.LevelDebug) {
//		expensiveData := generateDebugData()
//		log.Debug("Expensive debug info",
//			logger.Key("debug_data", expensiveData),
//		)
//	}
//
// # Global Logger Setup
//
// Set up a global default logger for your application:
//
//	func initLogging() {
//		var log *slog.Logger
//
//		switch os.Getenv("APP_ENV") {
//		case "development":
//			log = logger.New(logger.WithDevelopment("myapp"))
//		case "staging":
//			log = logger.New(logger.WithStaging("myapp"))
//		case "production":
//			log = logger.New(logger.WithProduction("myapp"))
//		default:
//			log = logger.New(logger.WithDevelopment("myapp"))
//		}
//
//		// Set as global default
//		logger.SetAsDefault(log)
//	}
//
//	// Use anywhere in your application
//	slog.Info("Using global logger", logger.Component("global"))
//
// # Testing with Custom Output
//
// Capture logs during testing:
//
//	import (
//		"bytes"
//		"testing"
//	)
//
//	func TestLogging(t *testing.T) {
//		var buf bytes.Buffer
//		log := logger.New(
//			logger.WithJSONFormatter(),
//			logger.WithOutput(&buf),
//		)
//
//		log.Info("Test message", logger.Component("test"))
//
//		output := buf.String()
//		assert.Contains(t, output, "Test message")
//		assert.Contains(t, output, `"component":"test"`)
//	}
//
// # Advanced Configuration
//
// Fine-tune logger behavior with advanced options:
//
//	log := logger.New(
//		logger.WithLevel(slog.LevelInfo),
//		logger.WithJSONFormatter(),
//		logger.WithHandlerOptions(&slog.HandlerOptions{
//			AddSource: true,  // Add source file info
//			Level:     slog.LevelDebug,
//			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
//				// Custom attribute processing
//				if a.Key == slog.TimeKey {
//					return slog.String("timestamp", a.Value.Time().Format(time.RFC3339))
//				}
//				return a
//			},
//		}),
//		logger.WithAttr(
//			slog.String("service", "myapp"),
//			slog.String("version", "1.0.0"),
//			slog.String("environment", "production"),
//		),
//	)
//
// # Best Practices
//
//   - Use structured logging with consistent attribute naming
//   - Include context information (request IDs, user IDs, etc.)
//   - Use appropriate log levels (Debug for development, Info for production)
//   - Log errors with sufficient context for debugging
//   - Use the attribute helpers for consistency across your application
//   - Set up context extractors for automatic attribute injection
//   - Configure different formats for different environments
//   - Monitor log volume and adjust levels appropriately
//   - Use groups to organize related attributes
//   - Include timing information for performance monitoring
package logger
