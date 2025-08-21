package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
	"github.com/dmitrymomot/gokit/middleware"
)

// testLogHandler captures log entries for testing
type testLogHandler struct {
	entries []map[string]any
}

func (h *testLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	entry := make(map[string]any)
	entry["level"] = r.Level.String()
	entry["msg"] = r.Message
	entry["time"] = r.Time

	r.Attrs(func(a slog.Attr) bool {
		entry[a.Key] = a.Value.Any()
		return true
	})

	h.entries = append(h.entries, entry)
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return h
}

func TestLoggingMiddleware(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithLogger[*router.Context](testLogger))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("test response"))
			return err
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test?param=value", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Check that logs were created
	require.Len(t, logHandler.entries, 2) // request start and response

	// Check request log
	reqLog := logHandler.entries[0]
	assert.Equal(t, "HTTP request started", reqLog["msg"])
	assert.Equal(t, "GET", reqLog["method"])
	assert.Equal(t, "/test", reqLog["path"])
	assert.Equal(t, "param=value", reqLog["query"])

	// Check response log
	respLog := logHandler.entries[1]
	assert.Equal(t, "HTTP request completed", respLog["msg"])
	assert.Equal(t, "GET", respLog["method"])
	assert.Equal(t, "/test", respLog["path"])
	assert.Equal(t, int64(200), respLog["status"])
	assert.Equal(t, int64(13), respLog["size"]) // "test response" = 13 bytes
	assert.NotNil(t, respLog["duration"])
}

func TestLoggingWithRequestID(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.RequestID[*router.Context]())
	r.Use(middleware.LoggingWithLogger[*router.Context](testLogger))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check that request ID is in logs
	for _, entry := range logHandler.entries {
		requestID, ok := entry["request_id"].(string)
		assert.True(t, ok, "request_id should be present")
		assert.NotEmpty(t, requestID)
	}
}

func TestLoggingSkipFunction(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithConfig[*router.Context](middleware.LoggingConfig{
		Logger: testLogger,
		Skip: func(ctx handler.Context) bool {
			return strings.HasPrefix(ctx.Request().URL.Path, "/health")
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Get("/health", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Test normal endpoint - should log
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logsAfterTest := len(logHandler.entries)
	assert.Greater(t, logsAfterTest, 0, "Should have logs for /test")

	// Test health endpoint - should not log
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, logsAfterTest, len(logHandler.entries), "Should not have new logs for /health")
}

func TestLoggingWithHeaders(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithConfig[*router.Context](middleware.LoggingConfig{
		Logger:     testLogger,
		LogHeaders: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-Custom-Header", "custom-value")
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Check request headers in log
	reqLog := logHandler.entries[0]
	headers, ok := reqLog["request_headers"].(map[string]any)
	require.True(t, ok, "Should have request headers")
	assert.Equal(t, "test-agent", headers["User-Agent"])
	assert.Equal(t, "[REDACTED]", headers["Authorization"], "Sensitive header should be redacted")
}

func TestLoggingWithRequestBody(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithConfig[*router.Context](middleware.LoggingConfig{
		Logger:         testLogger,
		LogRequestBody: true,
		MaxBodyLogSize: 50,
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		// Read the body to ensure it's still available
		body := make([]byte, 100)
		n, _ := ctx.Request().Body.Read(body)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body[:n])
			return err
		}
	})

	// Test small body
	smallBody := `{"test": "data"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(smallBody))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	reqLog := logHandler.entries[0]
	assert.Equal(t, smallBody, reqLog["request_body"])
	assert.Nil(t, reqLog["request_body_truncated"])

	// Reset logs
	logHandler.entries = nil

	// Test large body (should be truncated)
	largeBody := strings.Repeat("a", 100)
	req = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	reqLog = logHandler.entries[0]
	bodyInLog := reqLog["request_body"].(string)
	assert.Len(t, bodyInLog, 50, "Body should be truncated to MaxBodyLogSize")
	assert.True(t, reqLog["request_body_truncated"].(bool))
}

func TestLoggingErrorStatus(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithLogger[*router.Context](testLogger))

	// Test 4xx response
	r.Get("/notfound", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
	})

	// Test 5xx response
	r.Get("/error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusInternalServerError)
			return nil
		}
	})

	// Test 404
	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respLog := logHandler.entries[1]
	assert.Equal(t, "WARN", respLog["level"], "4xx should log at WARN level")

	// Reset logs
	logHandler.entries = nil

	// Test 500
	req = httptest.NewRequest(http.MethodGet, "/error", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respLog = logHandler.entries[1]
	assert.Equal(t, "ERROR", respLog["level"], "5xx should log at ERROR level")
}

func TestLoggingSlowRequest(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithConfig[*router.Context](middleware.LoggingConfig{
		Logger:               testLogger,
		SlowRequestThreshold: 50 * time.Millisecond,
	}))

	r.Get("/slow", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respLog := logHandler.entries[1]
	assert.Equal(t, "WARN", respLog["level"], "Slow request should log at WARN level")
	assert.True(t, respLog["slow_request"].(bool))
}

func TestLoggingDefaults(t *testing.T) {
	t.Parallel()

	// Create a middleware with empty config to test defaults
	cfg := middleware.LoggingConfig{}

	// Create a test handler to verify the middleware works
	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithConfig[*router.Context](cfg))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic with default config
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoggingConfigOptions(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithConfig[*router.Context](middleware.LoggingConfig{
		Logger:      testLogger,
		LogLevel:    slog.LevelDebug,
		Component:   "api",
		LogRequest:  true,
		LogResponse: false, // Only log requests
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should only have request log, not response
	assert.Len(t, logHandler.entries, 1)
	assert.Equal(t, "HTTP request started", logHandler.entries[0]["msg"])

	// Check component is set
	component := logHandler.entries[0]["component"]
	assert.Equal(t, "api", component)
}

func TestLoggingResponseSize(t *testing.T) {
	t.Parallel()

	logHandler := &testLogHandler{}
	testLogger := slog.New(logHandler)

	r := router.New[*router.Context]()
	r.Use(middleware.LoggingWithLogger[*router.Context](testLogger))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Write multiple chunks to test size calculation
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello "))
			w.Write([]byte("World!"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respLog := logHandler.entries[1]
	assert.Equal(t, int64(12), respLog["size"]) // "Hello World!" = 12 bytes
}
