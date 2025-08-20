package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/ratelimiter"
)

type RateLimitConfig struct {
	Skip         func(ctx handler.Context) bool
	Limiter      ratelimiter.RateLimiter
	KeyExtractor func(ctx handler.Context) string
	ErrorHandler func(ctx handler.Context, result *ratelimiter.Result) handler.Response
	SetHeaders   bool
}

func RateLimit[C handler.Context](cfg RateLimitConfig) handler.Middleware[C] {
	if cfg.Limiter == nil {
		panic("ratelimit middleware: limiter is required")
	}

	if cfg.KeyExtractor == nil {
		cfg.KeyExtractor = func(ctx handler.Context) string {
			if ip, ok := GetClientIP(ctx); ok {
				return ip
			}
			return ctx.Request().RemoteAddr
		}
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(ctx handler.Context, result *ratelimiter.Result) handler.Response {
			err := response.ErrTooManyRequests
			if result != nil && result.RetryAfter() > 0 {
				err = err.WithDetails(map[string]any{
					"retry_after": fmt.Sprintf("%.0f", result.RetryAfter().Seconds()),
				})
			}
			return response.JSONWithStatus(err, err.Status)
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			key := cfg.KeyExtractor(ctx)
			result, err := cfg.Limiter.Allow(ctx.Request().Context(), key)
			if err != nil {
				return response.JSONWithStatus(
					response.ErrInternalServerError.WithError(err),
					response.ErrInternalServerError.Status,
				)
			}

			if !result.Allowed() {
				resp := cfg.ErrorHandler(ctx, result)
				if cfg.SetHeaders {
					return wrapWithRateLimitHeaders(resp, result)
				}
				return resp
			}

			resp := next(ctx)

			if cfg.SetHeaders {
				return wrapWithRateLimitHeaders(resp, result)
			}

			return resp
		}
	}
}

func wrapWithRateLimitHeaders(resp handler.Response, result *ratelimiter.Result) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, result.Remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed() && result.RetryAfter() > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter().Seconds())))
		}

		return resp(w, r)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
