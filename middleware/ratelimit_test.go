package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/middleware"
	"github.com/dmitrymomot/foundation/pkg/ratelimiter"
)

func TestRateLimitBasicFunctionality(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       5,
		RefillRate:     1,
		RefillInterval: time.Second,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter:    limiter,
		SetHeaders: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		assert.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, strconv.Itoa(4-i), w.Header().Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "6th request should be rate limited")
	assert.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestRateLimitSkipFunction(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       1,
		RefillRate:     1,
		RefillInterval: time.Hour,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter: limiter,
		Skip: func(ctx handler.Context) bool {
			return ctx.Request().Header.Get("X-Skip-RateLimit") == "true"
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.100:54321"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code, "First request should succeed")

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.100:54321"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code, "Second request should be rate limited")

	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.100:54321"
	req3.Header.Set("X-Skip-RateLimit", "true")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code, "Request with skip header should succeed")
	assert.Empty(t, w3.Header().Get("X-RateLimit-Limit"), "Skipped requests should not have rate limit headers")
}

func TestRateLimitCustomKeyExtractor(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       2,
		RefillRate:     1,
		RefillInterval: time.Hour,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter: limiter,
		KeyExtractor: func(ctx handler.Context) string {
			return ctx.Request().Header.Get("X-API-Key")
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "user1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "User1 request %d should succeed", i+1)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-API-Key", "user1")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusTooManyRequests, w1.Code, "User1 should be rate limited")

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-API-Key", "user2")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "User2 should not be rate limited")
}

func TestRateLimitCustomErrorHandler(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       1,
		RefillRate:     1,
		RefillInterval: time.Hour,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter: limiter,
		ErrorHandler: func(ctx handler.Context, result *ratelimiter.Result) handler.Response {
			return response.JSONWithStatus(
				map[string]string{"error": "custom rate limit message"},
				http.StatusTooManyRequests,
			)
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.100:54321"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.100:54321"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.Contains(t, w2.Body.String(), "custom rate limit message")
}

func TestRateLimitDisableHeaders(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       5,
		RefillRate:     1,
		RefillInterval: time.Second,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter:    limiter,
		SetHeaders: false,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.Empty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.Empty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimitWithClientIPMiddleware(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       2,
		RefillRate:     1,
		RefillInterval: time.Hour,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.ClientIP[*router.Context]())
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter:    limiter,
		SetHeaders: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d from 10.0.0.1 should succeed", i+1)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Forwarded-For", "10.0.0.1")
	req1.RemoteAddr = "192.168.1.100:54321"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusTooManyRequests, w1.Code, "10.0.0.1 should be rate limited")

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Forwarded-For", "10.0.0.2")
	req2.RemoteAddr = "192.168.1.100:54321"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "10.0.0.2 should not be rate limited")
}

func TestRateLimitRefill(t *testing.T) {
	t.Parallel()

	store := ratelimiter.NewMemoryStore()
	defer store.Close()

	limiter, err := ratelimiter.NewBucket(store, ratelimiter.Config{
		Capacity:       2,
		RefillRate:     2,
		RefillInterval: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.RateLimit[*router.Context](middleware.RateLimitConfig{
		Limiter:    limiter,
		SetHeaders: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.100:54321"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusTooManyRequests, w1.Code, "Should be rate limited")

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d after refill should succeed", i+1)
	}
}
