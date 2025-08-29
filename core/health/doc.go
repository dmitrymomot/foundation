// Package health provides HTTP handlers for service health monitoring.
//
// Handlers:
//   - Liveness: Process is running (no dependency checks)
//   - Readiness: All dependencies are available
//   - NoContent: Returns 204 for minimal overhead
//
// Usage:
//
//	// Setup health routes
//	r.GET("/health/live", health.Liveness[*AppContext])
//	r.GET("/health/ready", health.Readiness[*AppContext](
//		logger,
//		db.Ping,
//		cache.Ping,
//	))
//	r.GET("/ping", health.NoContent[*AppContext])
//
// Dependency checks must follow func(context.Context) error signature:
//
//	func checkDB(ctx context.Context) error {
//		return db.PingContext(ctx)
//	}
package health
