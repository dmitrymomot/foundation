package logger_test

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/pkg/logger"
)

func TestGroup(t *testing.T) {
	t.Parallel()
	attr := logger.Group("req", slog.String("id", "1"), slog.Int("n", 2))
	require.Equal(t, "req", attr.Key)
	require.Equal(t, slog.KindGroup, attr.Value.Kind())
	g := attr.Value.Group()
	require.Len(t, g, 2)
	assert.Equal(t, "id", g[0].Key)
	assert.Equal(t, "n", g[1].Key)
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestErrors(t *testing.T) {
	t.Parallel()
	err1 := errors.New("first")
	err2 := errors.New("second")

	attr := logger.Errors(err1, nil, err2)
	require.Equal(t, "errors", attr.Key)
	require.Equal(t, slog.KindGroup, attr.Value.Kind())
	g := attr.Value.Group()
	require.Len(t, g, 2)
	assert.Equal(t, err1, g[0].Value.Any())
	assert.Equal(t, err2, g[1].Value.Any())

	empty := logger.Errors(nil)
	assert.True(t, empty.Equal(slog.Attr{}))
}

func TestError(t *testing.T) {
	t.Parallel()
	err := errors.New("boom")
	attr := logger.Error(err)
	require.Equal(t, "error", attr.Key)
	assert.Equal(t, err, attr.Value.Any())

	empty := logger.Error(nil)
	assert.True(t, empty.Equal(slog.Attr{}))
}

// ============================================================================
// Performance and Timing Tests
// ============================================================================

func TestDuration(t *testing.T) {
	t.Parallel()
	d := 5 * time.Second
	attr := logger.Duration(d)
	require.Equal(t, "duration", attr.Key)
	assert.Equal(t, d, attr.Value.Duration())
}

func TestLatency(t *testing.T) {
	t.Parallel()
	d := 100 * time.Millisecond
	attr := logger.Latency(d)
	require.Equal(t, "latency", attr.Key)
	assert.Equal(t, d, attr.Value.Duration())
}

func TestElapsed(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-500 * time.Millisecond)
	attr := logger.Elapsed(start)
	require.Equal(t, "elapsed", attr.Key)
	// Check that elapsed is at least 500ms
	assert.GreaterOrEqual(t, attr.Value.Duration(), 500*time.Millisecond)
}

// ============================================================================
// Generic Identifiers Tests
// ============================================================================

func TestID(t *testing.T) {
	t.Parallel()

	// Test with string
	attr := logger.ID("user_id", "123")
	require.Equal(t, "user_id", attr.Key)
	assert.Equal(t, "123", attr.Value.Any())

	// Test with int (slog converts to appropriate type)
	attr = logger.ID("count", 42)
	require.Equal(t, "count", attr.Key)
	// slog.Any may convert int to int64 internally
	assert.EqualValues(t, 42, attr.Value.Any())

	// Test with nil
	empty := logger.ID("key", nil)
	assert.True(t, empty.Equal(slog.Attr{}))
}

func TestRequestID(t *testing.T) {
	t.Parallel()
	attr := logger.RequestID("req-123")
	require.Equal(t, "request_id", attr.Key)
	assert.Equal(t, "req-123", attr.Value.String())

	// Test with empty string
	empty := logger.RequestID("")
	assert.True(t, empty.Equal(slog.Attr{}))
}

func TestTraceID(t *testing.T) {
	t.Parallel()
	attr := logger.TraceID("trace-456")
	require.Equal(t, "trace_id", attr.Key)
	assert.Equal(t, "trace-456", attr.Value.String())

	// Test with empty string
	empty := logger.TraceID("")
	assert.True(t, empty.Equal(slog.Attr{}))
}

func TestCorrelationID(t *testing.T) {
	t.Parallel()
	attr := logger.CorrelationID("corr-789")
	require.Equal(t, "correlation_id", attr.Key)
	assert.Equal(t, "corr-789", attr.Value.String())

	// Test with empty string
	empty := logger.CorrelationID("")
	assert.True(t, empty.Equal(slog.Attr{}))
}

// ============================================================================
// Network and HTTP Tests
// ============================================================================

func TestMethod(t *testing.T) {
	t.Parallel()
	attr := logger.Method("GET")
	require.Equal(t, "method", attr.Key)
	assert.Equal(t, "GET", attr.Value.String())
}

func TestPath(t *testing.T) {
	t.Parallel()
	attr := logger.Path("/api/users")
	require.Equal(t, "path", attr.Key)
	assert.Equal(t, "/api/users", attr.Value.String())
}

func TestStatusCode(t *testing.T) {
	t.Parallel()
	attr := logger.StatusCode(200)
	require.Equal(t, "status_code", attr.Key)
	assert.Equal(t, int64(200), attr.Value.Int64())
}

func TestClientIP(t *testing.T) {
	t.Parallel()
	attr := logger.ClientIP("192.168.1.1")
	require.Equal(t, "client_ip", attr.Key)
	assert.Equal(t, "192.168.1.1", attr.Value.String())
}

func TestUserAgent(t *testing.T) {
	t.Parallel()
	attr := logger.UserAgent("Mozilla/5.0")
	require.Equal(t, "user_agent", attr.Key)
	assert.Equal(t, "Mozilla/5.0", attr.Value.String())
}

func TestBytesIn(t *testing.T) {
	t.Parallel()
	attr := logger.BytesIn(1024)
	require.Equal(t, "bytes_in", attr.Key)
	assert.Equal(t, int64(1024), attr.Value.Int64())
}

func TestBytesOut(t *testing.T) {
	t.Parallel()
	attr := logger.BytesOut(2048)
	require.Equal(t, "bytes_out", attr.Key)
	assert.Equal(t, int64(2048), attr.Value.Int64())
}

// ============================================================================
// Generic Metadata Tests
// ============================================================================

func TestComponent(t *testing.T) {
	t.Parallel()
	attr := logger.Component("auth")
	require.Equal(t, "component", attr.Key)
	assert.Equal(t, "auth", attr.Value.String())
}

func TestEvent(t *testing.T) {
	t.Parallel()
	attr := logger.Event("user_login")
	require.Equal(t, "event", attr.Key)
	assert.Equal(t, "user_login", attr.Value.String())
}

func TestType(t *testing.T) {
	t.Parallel()
	attr := logger.Type("notification")
	require.Equal(t, "type", attr.Key)
	assert.Equal(t, "notification", attr.Value.String())
}

func TestAction(t *testing.T) {
	t.Parallel()
	attr := logger.Action("create")
	require.Equal(t, "action", attr.Key)
	assert.Equal(t, "create", attr.Value.String())
}

func TestResult(t *testing.T) {
	t.Parallel()
	attr := logger.Result("success")
	require.Equal(t, "result", attr.Key)
	assert.Equal(t, "success", attr.Value.String())
}

func TestCount(t *testing.T) {
	t.Parallel()
	attr := logger.Count("attempts", 3)
	require.Equal(t, "attempts", attr.Key)
	assert.Equal(t, int64(3), attr.Value.Int64())
}

func TestVersion(t *testing.T) {
	t.Parallel()
	attr := logger.Version("1.2.3")
	require.Equal(t, "version", attr.Key)
	assert.Equal(t, "1.2.3", attr.Value.String())
}

func TestKey(t *testing.T) {
	t.Parallel()

	// Test with string value
	attr := logger.Key("custom", "value")
	require.Equal(t, "custom", attr.Key)
	assert.Equal(t, "value", attr.Value.Any())

	// Test with struct value
	type testStruct struct {
		Name string
	}
	s := testStruct{Name: "test"}
	attr = logger.Key("data", s)
	require.Equal(t, "data", attr.Key)
	assert.Equal(t, s, attr.Value.Any())

	// Test with nil
	empty := logger.Key("key", nil)
	assert.True(t, empty.Equal(slog.Attr{}))
}

func TestRetryCount(t *testing.T) {
	t.Parallel()
	attr := logger.RetryCount(5)
	require.Equal(t, "retry_count", attr.Key)
	assert.Equal(t, int64(5), attr.Value.Int64())
}

// ============================================================================
// Debugging Tests
// ============================================================================

func TestStack(t *testing.T) {
	t.Parallel()
	attr := logger.Stack()
	require.Equal(t, "stack", attr.Key)
	stack := attr.Value.String()
	// Check that stack trace contains this test function
	assert.Contains(t, stack, "TestStack")
	assert.Contains(t, stack, "attr_test.go")
}

func TestCaller(t *testing.T) {
	t.Parallel()
	attr := logger.Caller()
	require.Equal(t, "caller", attr.Key)
	caller := attr.Value.String()
	// Check that caller info contains this test file
	assert.Contains(t, caller, "attr_test.go")
	// Check that it contains a line number
	parts := strings.Split(caller, ":")
	assert.Len(t, parts, 2)
}
