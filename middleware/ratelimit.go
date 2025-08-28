package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/pkg/ratelimiter"
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
//
// Rate limiting protects your application from abuse, DoS attacks, and ensures
// fair usage among clients. It tracks requests per key (IP, user, etc.) and
// blocks requests that exceed configured limits.
//
// Basic Usage:
//
//	// Create rate limiter (example with Redis backend)
//	limiter, err := ratelimiter.NewRedis(redisClient, ratelimiter.RedisConfig{
//		Limit:  100,        // 100 requests
//		Window: time.Hour,  // per hour
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Apply rate limiting by IP address
//	cfg := middleware.RateLimitConfig{
//		Limiter:    limiter,
//		SetHeaders: true, // Include rate limit info in response headers
//	}
//	r.Use(middleware.RateLimit[*MyContext](cfg))
//
//	// Different limits for different endpoints
//	api := r.Group("/api")
//	api.Use(middleware.RateLimit[*MyContext](middleware.RateLimitConfig{
//		Limiter: apiLimiter, // 1000 requests per hour for API
//	}))
//
//	auth := r.Group("/auth")
//	auth.Use(middleware.RateLimit[*MyContext](middleware.RateLimitConfig{
//		Limiter: authLimiter, // 10 requests per minute for auth
//	}))
//
// The middleware automatically:
// - Extracts rate limiting key (default: client IP)
// - Checks current usage against limits
// - Updates counters for allowed requests
// - Returns 429 Too Many Requests when limits exceeded
// - Includes retry-after information in error responses
//
// Rate limiting strategies:
// - By IP: Protect against individual client abuse
// - By user: Ensure fair usage among authenticated users
// - By API key: Control third-party API usage
// - By endpoint: Different limits for different operations
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
			return response.Error(err)
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
				return response.Error(response.ErrInternalServerError.WithError(err))
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
//
// Standard headers added:
// - X-RateLimit-Limit: Maximum requests allowed in the time window
// - X-RateLimit-Remaining: Requests remaining in current window (clamped to 0)
// - X-RateLimit-Reset: Unix timestamp when the limit resets
// - Retry-After: Seconds to wait before retrying (only when blocked)
//
// These headers help clients understand rate limiting status and implement
// proper retry logic. They follow common industry standards used by APIs
// like GitHub, Twitter, and others.
func wrapWithRateLimitHeaders(resp handler.Response, result *ratelimiter.Result) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		// Clamp remaining count to zero to prevent confusing negative values in API responses
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, result.Remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed() && result.RetryAfter() > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter().Seconds())))
		}

		return resp(w, r)
	}
}
