package response_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit/core/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SSE Protocol Format Validation
func TestSSE_ProtocolFormat(t *testing.T) {
	t.Parallel()

	t.Run("basic_data_event_format", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- "test message"
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, ": connected\n\n")
		assert.Contains(t, output, "data: test message\n\n")
		assert.True(t, strings.HasSuffix(output, "\n\n"), "Events must end with double newline")
	})

	t.Run("event_with_name_and_id", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- "test data"
		close(events)

		response := response.SSE(events,
			response.WithEventName("update"),
			response.WithEventID("123"),
			response.WithoutKeepAlive(),
		)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "event: update\n")
		assert.Contains(t, output, "id: 123\n")
		assert.Contains(t, output, "data: test data\n\n")
	})

	t.Run("retry_header_set_correctly", func(t *testing.T) {
		t.Parallel()

		events := make(chan any)
		close(events)

		response := response.SSE(events,
			response.WithReconnectTime(5000),
			response.WithoutKeepAlive(),
		)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		assert.Equal(t, "5000", w.Header().Get("Retry"))
	})

	t.Run("required_headers_set", func(t *testing.T) {
		t.Parallel()

		events := make(chan any)
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
		assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// Tests for Keep-Alive Mechanism
func TestSSE_KeepAlive(t *testing.T) {
	t.Parallel()

	t.Run("keep_alive_messages_sent", func(t *testing.T) {
		t.Parallel()

		events := make(chan any)

		// Create context that will be cancelled after keep-alive should trigger
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		response := response.SSE(events, response.WithKeepAlive(50*time.Millisecond))
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, ": connected\n\n")
		assert.Contains(t, output, ": keepalive\n\n")
	})

	t.Run("keep_alive_timer_reset_on_real_event", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		response := response.SSE(events, response.WithKeepAlive(100*time.Millisecond))
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		// Send event after 50ms to reset keep-alive timer
		go func() {
			time.Sleep(50 * time.Millisecond)
			events <- "real event"
			close(events)
		}()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "data: real event\n\n")

		// Count keep-alive messages - should be minimal due to timer reset
		keepAliveCount := strings.Count(output, ": keepalive\n\n")
		assert.LessOrEqual(t, keepAliveCount, 1, "Keep-alive timer should reset after real event")
	})

	t.Run("no_keep_alive_when_disabled", func(t *testing.T) {
		t.Parallel()

		events := make(chan any)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.NotContains(t, output, ": keepalive\n\n")
	})
}

// Tests for Channel Closure and Context Handling
func TestSSE_ChannelAndContextHandling(t *testing.T) {
	t.Parallel()

	t.Run("clean_shutdown_on_channel_close", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 2)
		events <- "message1"
		events <- "message2"
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "data: message1\n\n")
		assert.Contains(t, output, "data: message2\n\n")
	})

	t.Run("shutdown_on_context_cancellation", func(t *testing.T) {
		t.Parallel()

		events := make(chan any)

		ctx, cancel := context.WithCancel(context.Background())

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		// Cancel context after short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, ": connected\n\n")
	})

	t.Run("nil_channel_handling", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		response := response.SSE(nil, response.WithoutKeepAlive())
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		// Should not panic and should handle gracefully
		err := response(w, req)
		require.NoError(t, err)
	})
}

// Tests for Event Options
func TestSSE_EventOptions(t *testing.T) {
	t.Parallel()

	t.Run("event_id_generator", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 2)
		events <- "first"
		events <- "second"
		close(events)

		counter := 0
		idGen := func(data any) string {
			counter++
			return fmt.Sprintf("id-%d", counter)
		}

		response := response.SSE(events,
			response.WithEventIDGenerator(idGen),
			response.WithoutKeepAlive(),
		)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "id: id-1\n")
		assert.Contains(t, output, "id: id-2\n")
	})

	t.Run("static_id_overridden_by_generator", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- "test"
		close(events)

		response := response.SSE(events,
			response.WithEventID("static"),
			response.WithEventIDGenerator(func(data any) string { return "generated" }),
			response.WithoutKeepAlive(),
		)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "id: generated\n")
		assert.NotContains(t, output, "id: static\n")
	})
}

// Tests for Data Type Handling
func TestSSE_DataTypes(t *testing.T) {
	t.Parallel()

	t.Run("string_data", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- "Hello, World!"
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "data: Hello, World!\n\n")
	})

	t.Run("byte_slice_data", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- []byte("byte data")
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "data: byte data\n\n")
	})

	t.Run("json_struct_data", func(t *testing.T) {
		t.Parallel()

		type TestStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		events := make(chan any, 1)
		events <- TestStruct{Name: "test", Value: 42}
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, `data: {"name":"test","value":42}`)
	})

	t.Run("json_marshal_error_handling", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- make(chan int) // channels can't be JSON marshaled
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err) // Should handle error gracefully

		// Stream should terminate due to marshal error
		output := w.Body.String()
		assert.Contains(t, output, ": connected\n\n")
	})
}

// nonFlushingResponseWriter wraps a ResponseWriter but doesn't implement Flusher
type nonFlushingResponseWriter struct {
	header  http.Header
	body    *strings.Builder
	code    int
	written bool
}

func newNonFlushingResponseWriter() *nonFlushingResponseWriter {
	return &nonFlushingResponseWriter{
		header: make(http.Header),
		body:   &strings.Builder{},
	}
}

func (w *nonFlushingResponseWriter) Header() http.Header {
	return w.header
}

func (w *nonFlushingResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(b)
}

func (w *nonFlushingResponseWriter) WriteHeader(statusCode int) {
	if !w.written {
		w.code = statusCode
		w.written = true
	}
}

// Tests for HTTP Interface Compliance
func TestSSE_HTTPCompliance(t *testing.T) {
	t.Parallel()

	t.Run("flusher_interface_required", func(t *testing.T) {
		t.Parallel()

		// Create a ResponseWriter that truly doesn't implement Flusher
		nonFlusher := newNonFlushingResponseWriter()

		events := make(chan any)
		close(events)

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)

		err := response(nonFlusher, req)
		require.NoError(t, err)

		// Should return error response for non-flusher
		assert.Equal(t, http.StatusInternalServerError, nonFlusher.code)
		assert.Contains(t, nonFlusher.body.String(), "Streaming unsupported")
	})
}

// Tests for Concurrent Safety
func TestSSE_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	t.Run("concurrent_event_sending", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 10)

		// Send events from multiple goroutines
		var wg sync.WaitGroup
		for i := range 5 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				events <- fmt.Sprintf("message-%d", id)
			}(i)
		}

		go func() {
			wg.Wait()
			close(events)
		}()

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		// Verify all messages were sent
		for i := range 5 {
			assert.Contains(t, output, fmt.Sprintf("message-%d", i))
		}
	})

	t.Run("channel_close_during_processing", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		events <- "message"

		response := response.SSE(events, response.WithoutKeepAlive())
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		// Close channel while processing
		go func() {
			time.Sleep(10 * time.Millisecond)
			close(events)
		}()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "data: message\n\n")
	})
}

// Integration test for realistic usage
func TestSSE_Integration(t *testing.T) {
	t.Parallel()

	t.Run("realistic_event_stream", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 3)

		// Simulate a realistic event stream
		go func() {
			events <- map[string]any{
				"type":      "user_joined",
				"user":      "alice",
				"timestamp": time.Now().Unix(),
			}
			time.Sleep(10 * time.Millisecond)

			events <- map[string]any{
				"type":      "message",
				"user":      "alice",
				"text":      "Hello everyone!",
				"timestamp": time.Now().Unix(),
			}
			time.Sleep(10 * time.Millisecond)

			events <- map[string]any{
				"type":      "user_left",
				"user":      "alice",
				"timestamp": time.Now().Unix(),
			}

			close(events)
		}()

		response := response.SSE(events,
			response.WithEventName("chat"),
			response.WithEventIDGenerator(func(data any) string {
				if m, ok := data.(map[string]any); ok {
					return fmt.Sprintf("%v", m["timestamp"])
				}
				return ""
			}),
			response.WithKeepAlive(1*time.Second),
		)

		req := httptest.NewRequest("GET", "/events", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()

		// Verify SSE format compliance
		assert.Contains(t, output, ": connected\n\n")
		assert.Contains(t, output, "event: chat\n")
		assert.Contains(t, output, "user_joined")
		assert.Contains(t, output, "Hello everyone!")
		assert.Contains(t, output, "user_left")

		// Verify proper JSON encoding
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: {") {
				// Extract JSON data
				jsonStr := strings.TrimPrefix(line, "data: ")
				var data map[string]any
				err := json.Unmarshal([]byte(jsonStr), &data)
				assert.NoError(t, err, "All JSON data should be valid")
			}
		}
	})
}

// Tests for SSE Error Handlers
func TestSSE_ErrorHandler(t *testing.T) {
	t.Parallel()

	t.Run("error_handler_called_on_write_error", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 2)
		var capturedCtx context.Context
		var capturedError error
		errorCount := 0

		response := response.SSE(events,
			response.WithSSEErrorHandler(func(ctx context.Context, err error) {
				capturedCtx = ctx
				capturedError = err
				errorCount++
			}),
			response.WithoutKeepAlive(),
		)

		// Add a valid event
		events <- map[string]string{"message": "test"}
		// Add an invalid event that will fail to marshal
		events <- make(chan int)
		close(events)

		ctx := context.WithValue(context.Background(), "trace_id", "abc-123")
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.NoError(t, err)

		// Check error handler was called
		assert.Equal(t, 1, errorCount, "Error handler should be called once")
		assert.NotNil(t, capturedError)
		assert.Contains(t, capturedError.Error(), "failed to write event")
		assert.NotNil(t, capturedCtx)
		assert.Equal(t, "abc-123", capturedCtx.Value("trace_id"))

		// Check valid event was still written
		output := w.Body.String()
		assert.Contains(t, output, ": connected")
		assert.Contains(t, output, `data: {"message":"test"}`)
	})

	t.Run("no_error_handler_continues_silently", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 2)

		// Create response without error handler
		response := response.SSE(events, response.WithoutKeepAlive())

		events <- map[string]string{"valid": "data"}
		events <- make(chan int) // Will fail to marshal
		close(events)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.NoError(t, err)

		// Should still have the valid event
		output := w.Body.String()
		assert.Contains(t, output, `data: {"valid":"data"}`)
	})

	t.Run("keepalive_error_stops_streaming", func(t *testing.T) {
		t.Parallel()

		// Custom writer that fails after initial writes
		failWriter := &failingWriter{
			failAfter: 2, // Allow connection message and first keepalive
			writer:    httptest.NewRecorder(),
		}

		events := make(chan any)
		var keepAliveError error

		response := response.SSE(events,
			response.WithSSEErrorHandler(func(ctx context.Context, err error) {
				if strings.Contains(err.Error(), "keepalive") {
					keepAliveError = err
				}
			}),
			response.WithKeepAlive(10*time.Millisecond),
		)

		req := httptest.NewRequest("GET", "/", nil)

		go func() {
			time.Sleep(50 * time.Millisecond)
			close(events)
		}()

		_ = response(failWriter, req)

		// Should have captured keepalive error
		assert.NotNil(t, keepAliveError)
		assert.Contains(t, keepAliveError.Error(), "failed to send keepalive")
	})

	t.Run("context_values_accessible", func(t *testing.T) {
		t.Parallel()

		events := make(chan any, 1)
		var contextValues []string

		response := response.SSE(events,
			response.WithSSEErrorHandler(func(ctx context.Context, err error) {
				if userID := ctx.Value("user_id"); userID != nil {
					contextValues = append(contextValues, userID.(string))
				}
				if requestID := ctx.Value("request_id"); requestID != nil {
					contextValues = append(contextValues, requestID.(string))
				}
			}),
			response.WithoutKeepAlive(),
		)

		// Send invalid event to trigger error
		events <- make(chan int)
		close(events)

		// Create context with multiple values
		ctx := context.WithValue(context.Background(), "user_id", "user-456")
		ctx = context.WithValue(ctx, "request_id", "req-789")
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.NoError(t, err)

		// Check context values were accessible
		assert.Contains(t, contextValues, "user-456")
		assert.Contains(t, contextValues, "req-789")
	})
}

// Helper type for testing write failures
type failingWriter struct {
	failAfter int
	writes    int
	writer    *httptest.ResponseRecorder
}

func (f *failingWriter) Header() http.Header {
	return f.writer.Header()
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.writes++
	if f.writes > f.failAfter {
		return 0, errors.New("write failed")
	}
	return f.writer.Write(p)
}

func (f *failingWriter) WriteHeader(statusCode int) {
	f.writer.WriteHeader(statusCode)
}

func (f *failingWriter) Flush() {
	// httptest.ResponseRecorder implements Flush directly
	f.writer.Flush()
}
