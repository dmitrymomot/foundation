// Package logger provides structured logging utilities built on Go's standard slog package.
// It offers environment-specific configurations, context-aware attribute extraction,
// and a comprehensive set of pre-built attributes for common logging scenarios.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/core/logger"
//
//	// Create a development logger (text format, debug level)
//	log := logger.New(logger.WithDevelopment("myapp"))
//
//	// Create a production logger (JSON format, info level)
//	log := logger.New(logger.WithProduction("myapp"))
//
//	// Use the logger with attribute helpers
//	log.Info("Server starting",
//		logger.Component("server"),
//		logger.Event("startup"),
//	)
//
// # Environment Configurations
//
// The package provides three pre-configured setups:
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
// # Custom Configuration
//
// Fine-tune logger behavior with various options:
//
//	log := logger.New(
//		logger.WithLevel(slog.LevelWarn),
//		logger.WithJSONFormatter(),
//		logger.WithAttr(slog.String("service", "api")),
//		logger.WithOutput(os.Stderr),
//	)
//
// # Context-Aware Logging
//
// Extract attributes automatically from context values:
//
//	log := logger.New(
//		logger.WithProduction("myapp"),
//		logger.WithContextValue("request_id", "request_id"),
//		logger.WithContextValue("user_id", "user_id"),
//	)
//
//	ctx := context.WithValue(context.Background(), "request_id", "req-12345")
//	ctx = context.WithValue(ctx, "user_id", "user-67890")
//
//	log.InfoContext(ctx, "Processing request")
//	// Output: {"level":"INFO","msg":"Processing request","request_id":"req-12345","user_id":"user-67890"}
//
// Custom extractors can be defined for more complex scenarios:
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
//	log := logger.New(
//		logger.WithProduction("myapp"),
//		logger.WithContextExtractors(requestIDExtractor),
//	)
//
// # Attribute Helpers
//
// The package provides many helper functions for creating structured attributes:
//
//	// Error handling
//	log.Error("Operation failed",
//		logger.Error(err),
//		logger.Action("database_query"),
//		logger.Component("user_service"),
//	)
//
//	// HTTP logging
//	log.Info("Request processed",
//		logger.Method("POST"),
//		logger.Path("/api/users"),
//		logger.StatusCode(201),
//		logger.ClientIP("192.168.1.1"),
//		logger.Latency(time.Since(start)),
//	)
//
//	// Performance timing
//	start := time.Now()
//	log.Info("Task completed",
//		logger.Duration(time.Since(start)),
//		logger.Component("worker"),
//	)
//
//	// Identifiers and tracing
//	log.Info("Processing payment",
//		logger.RequestID("req-12345"),
//		logger.TraceID("trace-67890"),
//		logger.CorrelationID("corr-abcde"),
//	)
//
// # Global Logger
//
// Set a global default logger for your application:
//
//	func initLogging() {
//		log := logger.New(logger.WithProduction("myapp"))
//		logger.SetAsDefault(log)
//	}
//
//	// Use anywhere in your application
//	slog.Info("Using global logger", logger.Component("app"))
package logger
