package health

import (
	"context"
	"log/slog"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/logger"
	"github.com/dmitrymomot/foundation/core/response"
)

// Readiness verifies all service dependencies are functioning.
// Returns "READY" if all checks pass, 503 Service Unavailable if any fail.
//
// Example:
//
//	readinessHandler := health.Readiness[*myapp.Context](
//		logger,
//		pg.Healthcheck(dbConn),
//		redis.Healthcheck(redisConn),
//	)
//	router.GET("/health/ready", readinessHandler)
func Readiness[C handler.Context](log *slog.Logger, fn ...func(context.Context) error) handler.HandlerFunc[C] {
	return func(ctx C) handler.Response {
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
