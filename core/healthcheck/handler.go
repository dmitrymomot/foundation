package healthcheck

import (
	"context"
	"log/slog"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/logger"
	"github.com/dmitrymomot/foundation/core/response"
)

// Handler creates a health check handler function that can serve as both a liveness
// and readiness probe depending on the provided dependency functions.
//
// When no dependency functions are provided, it acts as a liveness probe and
// returns "ALIVE" to indicate the service is running.
//
// When dependency functions are provided, it acts as a readiness probe and
// executes each function in sequence. If all succeed, it returns "READY".
// If any function fails, it logs the error and returns a service unavailable error.
//
// Example:
//
//	// Liveness probe - no dependencies
//	livenessHandler := healthcheck.Handler[*myapp.Context](logger)
//
//	// Readiness probe - with database and cache checks
//	readinessHandler := healthcheck.Handler[*myapp.Context](
//		logger,
//		pg.Healthcheck(dbConnInstance),
//		redis.Healthcheck(redisConnInstance)
//	)
//
//	router.GET("/health/live", livenessHandler)
//	router.GET("/health/ready", readinessHandler)
func Handler[C handler.Context](log *slog.Logger, fn ...func(context.Context) error) handler.HandlerFunc[C] {
	return func(ctx C) handler.Response {
		// Liveness probe: no dependency functions supplied.
		if len(fn) == 0 {
			return response.String("ALIVE")
		}

		// Readiness probe: verify all dependency functions succeed.
		for _, f := range fn {
			if err := f(ctx); err != nil {
				log.ErrorContext(ctx, "Readiness check failed", logger.Error(err))
				return response.Error(response.ErrServiceUnavailable)
			}
		}

		return response.String("READY")
	}
}
