package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/ratelimiter"
)

// RateLimitConfig configures the rate limiting middleware.
type RateLimitConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool
	// Limiter is the rate limiting implementation to use
	Limiter ratelimiter.RateLimiter
	// KeyExtractor defines how to extract the rate limiting key from requests (default: client IP)
	KeyExtractor func(ctx handler.Context) string
	// ErrorHandler defines how to handle rate limit violations (default: 429 Too Many Requests)
	ErrorHandler func(ctx handler.Context, result *ratelimiter.Result) handler.Response
	// SetHeaders determines whether to include rate limit information in response headers
	SetHeaders bool
}

// RateLimit creates a rate limiting middleware with the provided configuration.
// It enforces request rate limits based on configurable keys (typically client IP)
// and returns appropriate HTTP responses when limits are exceeded.
// Panics if no limiter is provided.
func RateLimit[C handler.Context](cfg RateLimitConfig) handler.Middleware[C] {
	if cfg.Limiter == nil {
		panic("ratelimit middleware: limiter is required")
	}

	// Default to using client IP as the rate limiting key
	if cfg.KeyExtractor == nil {
		cfg.KeyExtractor = func(ctx handler.Context) string {
			if ip, ok := GetClientIP(ctx); ok {
				return ip
			}
			return ctx.Request().RemoteAddr
		}
	}

	// Default error handler returns 429 with retry information
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

// wrapWithRateLimitHeaders adds standard rate limiting headers to the response.
// Headers include current limit, remaining requests, reset time, and retry-after when applicable.
func wrapWithRateLimitHeaders(resp handler.Response, result *ratelimiter.Result) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		// Ensure remaining count doesn't go below zero for cleaner API responses
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, result.Remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed() && result.RetryAfter() > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter().Seconds())))
		}

		return resp(w, r)
	}
}

// max returns the larger of two integers.
// Used to ensure remaining rate limit count doesn't go negative.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
