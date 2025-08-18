package response_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for Stream Response
func TestStream_BasicFunctionality(t *testing.T) {
	t.Parallel()

	t.Run("simple_streaming", func(t *testing.T) {
		t.Parallel()

		response := response.Stream(func(w io.Writer) error {
			for i := range 3 {
				_, _ = fmt.Fprintf(w, "chunk-%d\n", i)
			}
			return nil
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		output := w.Body.String()
		assert.Contains(t, output, "chunk-0\n")
		assert.Contains(t, output, "chunk-1\n")
		assert.Contains(t, output, "chunk-2\n")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("headers_set_correctly", func(t *testing.T) {
		t.Parallel()

		response := response.Stream(func(w io.Writer) error {
			return nil
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		assert.Equal(t, "chunked", w.Header().Get("Transfer-Encoding"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	})

	t.Run("writer_error_handling", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("write failed")
		response := response.Stream(func(w io.Writer) error {
			_, _ = fmt.Fprint(w, "partial data")
			return expectedErr
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.Equal(t, expectedErr, err)

		// Data before error should still be written
		assert.Contains(t, w.Body.String(), "partial data")
	})

	t.Run("binary_data_streaming", func(t *testing.T) {
		t.Parallel()

		binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
		response := response.Stream(func(w io.Writer) error {
			_, err := w.Write(binaryData)
			return err
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		assert.Equal(t, binaryData, w.Body.Bytes())
	})
}

// Tests for StreamJSON Response
func TestStreamJSON_BasicFunctionality(t *testing.T) {
	t.Parallel()

	t.Run("simple_json_streaming", func(t *testing.T) {
		t.Parallel()

		items := make(chan any, 3)
		items <- map[string]string{"id": "1", "name": "Alice"}
		items <- map[string]string{"id": "2", "name": "Bob"}
		items <- map[string]string{"id": "3", "name": "Charlie"}
		close(items)

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify NDJSON format
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, 3)

		// Each line should be valid JSON
		for i, line := range lines {
			var data map[string]string
			err := json.Unmarshal([]byte(line), &data)
			require.NoError(t, err, "Line %d should be valid JSON", i)
			assert.NotEmpty(t, data["id"])
			assert.NotEmpty(t, data["name"])
		}

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("headers_set_correctly", func(t *testing.T) {
		t.Parallel()

		items := make(chan any)
		close(items)

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		assert.Equal(t, "application/x-ndjson", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	})

	t.Run("different_data_types", func(t *testing.T) {
		t.Parallel()

		type User struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Admin bool   `json:"admin"`
		}

		items := make(chan any, 4)
		items <- User{ID: 1, Name: "Alice", Admin: true}
		items <- map[string]any{"type": "event", "timestamp": 1234567890}
		items <- []int{1, 2, 3, 4, 5}
		items <- "plain string"
		close(items)

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, 4)

		// Verify first line (struct)
		var user User
		err = json.Unmarshal([]byte(lines[0]), &user)
		require.NoError(t, err)
		assert.Equal(t, 1, user.ID)

		// Verify second line (map)
		var event map[string]any
		err = json.Unmarshal([]byte(lines[1]), &event)
		require.NoError(t, err)
		assert.Equal(t, "event", event["type"])

		// Verify third line (array)
		var numbers []int
		err = json.Unmarshal([]byte(lines[2]), &numbers)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3, 4, 5}, numbers)

		// Verify fourth line (string)
		var str string
		err = json.Unmarshal([]byte(lines[3]), &str)
		require.NoError(t, err)
		assert.Equal(t, "plain string", str)
	})

	t.Run("empty_channel", func(t *testing.T) {
		t.Parallel()

		items := make(chan any)
		close(items)

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Should have no content
		assert.Empty(t, w.Body.String())
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("nil_channel", func(t *testing.T) {
		t.Parallel()

		var items chan any // nil channel

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		response := response.StreamJSON(items)
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Should handle gracefully
		assert.Empty(t, w.Body.String())
	})
}

// Tests for Context Cancellation
func TestStream_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("stream_json_context_cancellation", func(t *testing.T) {
		t.Parallel()

		items := make(chan any)
		ctx, cancel := context.WithCancel(context.Background())

		response := response.StreamJSON(items)
		req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
		w := httptest.NewRecorder()

		// Cancel after short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		// Send items slowly
		go func() {
			for i := range 100 {
				select {
				case items <- i:
					time.Sleep(20 * time.Millisecond)
				case <-ctx.Done():
					close(items)
					return
				}
			}
		}()

		err := response(w, req)
		require.NoError(t, err)

		// Should have some data but not all
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.NotEmpty(t, lines)
		assert.Less(t, len(lines), 100)
	})
}

// Tests for HTTP Compliance
func TestStream_HTTPCompliance(t *testing.T) {
	t.Parallel()

	t.Run("flusher_required_for_stream", func(t *testing.T) {
		t.Parallel()

		// Custom non-flushing writer
		nf := newNonFlushingResponseWriter()

		response := response.Stream(func(w io.Writer) error {
			_, _ = fmt.Fprint(w, "data")
			return nil
		})

		req := httptest.NewRequest("GET", "/", nil)
		err := response(nf, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusInternalServerError, nf.code)
		assert.Contains(t, nf.body.String(), "Streaming unsupported")
	})

	t.Run("flusher_required_for_streamjson", func(t *testing.T) {
		t.Parallel()

		// Custom non-flushing writer
		nf := newNonFlushingResponseWriter()

		items := make(chan any, 1)
		items <- "test"
		close(items)

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		err := response(nf, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusInternalServerError, nf.code)
		assert.Contains(t, nf.body.String(), "Streaming unsupported")
	})
}

// Tests for Concurrent Safety
func TestStream_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	t.Run("concurrent_json_streaming", func(t *testing.T) {
		t.Parallel()

		items := make(chan any, 100)

		// Send items from multiple goroutines
		var wg sync.WaitGroup
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 10 {
					items <- map[string]int{"id": id, "value": j}
				}
			}(i)
		}

		go func() {
			wg.Wait()
			close(items)
		}()

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify all items were streamed
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, 100)

		// Each line should be valid JSON
		for _, line := range lines {
			var data map[string]int
			err := json.Unmarshal([]byte(line), &data)
			require.NoError(t, err)
		}
	})

	t.Run("concurrent_stream_writes", func(t *testing.T) {
		t.Parallel()

		// Create a thread-safe writer wrapper
		type safeWriter struct {
			mu sync.Mutex
			w  io.Writer
		}

		response := response.Stream(func(w io.Writer) error {
			sw := &safeWriter{w: w}
			var wg sync.WaitGroup
			errors := make(chan error, 5)

			for i := range 5 {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					sw.mu.Lock()
					_, err := fmt.Fprintf(sw.w, "goroutine-%d\n", id)
					sw.mu.Unlock()
					if err != nil {
						errors <- err
					}
				}(i)
			}

			wg.Wait()
			close(errors)

			// Check for any errors
			for err := range errors {
				return err
			}
			return nil
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify all goroutines wrote their data
		output := w.Body.String()
		for i := range 5 {
			assert.Contains(t, output, fmt.Sprintf("goroutine-%d", i))
		}
	})
}

// Tests for Large Dataset Streaming
func TestStream_LargeDatasets(t *testing.T) {
	t.Parallel()

	t.Run("stream_large_json_dataset", func(t *testing.T) {
		t.Parallel()

		const itemCount = 1000
		items := make(chan any, 100)

		go func() {
			defer close(items)
			for i := range itemCount {
				items <- map[string]any{
					"id":        i,
					"timestamp": time.Now().Unix(),
					"data":      fmt.Sprintf("item-%d", i),
				}
			}
		}()

		response := response.StreamJSON(items)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify all items were streamed
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, itemCount)

		// Spot check some lines
		for i := range 10 {
			var data map[string]any
			err := json.Unmarshal([]byte(lines[i*100]), &data)
			require.NoError(t, err)
			assert.Equal(t, float64(i*100), data["id"])
		}
	})

	t.Run("stream_csv_export", func(t *testing.T) {
		t.Parallel()

		response := response.Stream(func(w io.Writer) error {
			// Write CSV header
			_, _ = fmt.Fprintln(w, "id,name,email")

			// Write CSV data
			for i := range 100 {
				_, _ = fmt.Fprintf(w, "%d,User%d,user%d@example.com\n", i, i, i)
			}
			return nil
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify CSV format
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, 101) // header + 100 data rows

		// Check header
		assert.Equal(t, "id,name,email", lines[0])

		// Check first data row
		assert.Equal(t, "0,User0,user0@example.com", lines[1])
	})
}

// Integration Tests
func TestStream_Integration(t *testing.T) {
	t.Parallel()

	t.Run("realistic_log_streaming", func(t *testing.T) {
		t.Parallel()

		type LogEntry struct {
			Timestamp string `json:"timestamp"`
			Level     string `json:"level"`
			Message   string `json:"message"`
			Source    string `json:"source"`
		}

		logs := make(chan any, 5)

		// Simulate log generation
		go func() {
			defer close(logs)
			levels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
			for i := range 20 {
				logs <- LogEntry{
					Timestamp: time.Now().Format(time.RFC3339),
					Level:     levels[i%len(levels)],
					Message:   fmt.Sprintf("Log message %d", i),
					Source:    fmt.Sprintf("service-%d", i%3),
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()

		response := response.StreamJSON(logs)
		req := httptest.NewRequest("GET", "/logs", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify NDJSON format
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, 20)

		// Parse and verify structure
		for _, line := range lines {
			var entry LogEntry
			err := json.Unmarshal([]byte(line), &entry)
			require.NoError(t, err)
			assert.NotEmpty(t, entry.Timestamp)
			assert.Contains(t, []string{"INFO", "WARN", "ERROR", "DEBUG"}, entry.Level)
			assert.NotEmpty(t, entry.Message)
			assert.NotEmpty(t, entry.Source)
		}
	})

	t.Run("realistic_data_export", func(t *testing.T) {
		t.Parallel()

		// Simulate database cursor pattern
		response := response.Stream(func(w io.Writer) error {
			// Simulate batched database reads
			batchSize := 10
			totalRecords := 50

			for offset := 0; offset < totalRecords; offset += batchSize {
				// Simulate database query
				for i := offset; i < offset+batchSize && i < totalRecords; i++ {
					record := map[string]any{
						"id":         i,
						"created_at": time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
						"status":     "active",
					}

					data, err := json.Marshal(record)
					if err != nil {
						return err
					}

					fmt.Fprintf(w, "%s\n", data)
				}

				// Flush batch
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}

			return nil
		})

		req := httptest.NewRequest("GET", "/export", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		require.NoError(t, err)

		// Verify export format
		lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
		assert.Len(t, lines, 50)

		// Verify it's valid NDJSON
		for i, line := range lines {
			var record map[string]any
			err := json.Unmarshal([]byte(line), &record)
			require.NoError(t, err, "Line %d should be valid JSON", i)
			assert.Equal(t, float64(i), record["id"])
			assert.NotEmpty(t, record["created_at"])
			assert.Equal(t, "active", record["status"])
		}
	})
}

// Tests for Error Handlers
func TestStreamJSON_ErrorHandler(t *testing.T) {
	t.Parallel()

	t.Run("error_handler_called_on_encode_error", func(t *testing.T) {
		t.Parallel()

		items := make(chan any, 2)
		var capturedCtx context.Context
		var capturedError error
		var errorCount int
		mu := sync.Mutex{}

		response := response.StreamJSON(items, response.WithStreamErrorHandler(func(ctx context.Context, err error) {
			mu.Lock()
			defer mu.Unlock()
			capturedCtx = ctx
			capturedError = err
			errorCount++
		}))

		// Send valid item first
		items <- map[string]string{"valid": "data"}
		// Send invalid item that will fail to marshal (channels can't be marshaled)
		items <- make(chan int)
		close(items)

		ctx := context.WithValue(context.Background(), "request_id", "test-123")
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.NoError(t, err) // Should not return error

		// Wait a bit for async processing
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, errorCount, "Error handler should be called once")
		assert.NotNil(t, capturedError)
		assert.Contains(t, capturedError.Error(), "failed to encode item")
		assert.NotNil(t, capturedCtx)
		assert.Equal(t, "test-123", capturedCtx.Value("request_id"))

		// Should have the valid item in output
		output := w.Body.String()
		assert.Contains(t, output, `{"valid":"data"}`)
	})

	t.Run("no_error_handler_continues_silently", func(t *testing.T) {
		t.Parallel()

		items := make(chan any, 2)

		// Create response without error handler
		response := response.StreamJSON(items)

		items <- map[string]string{"valid": "data"}
		items <- make(chan int) // Will fail to marshal
		close(items)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.NoError(t, err)

		// Should still have the valid item
		output := w.Body.String()
		assert.Contains(t, output, `{"valid":"data"}`)
	})

	t.Run("context_cancellation_stops_streaming", func(t *testing.T) {
		t.Parallel()

		items := make(chan any)
		errorCalled := false

		response := response.StreamJSON(items, response.WithStreamErrorHandler(func(ctx context.Context, err error) {
			errorCalled = true
		}))

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		// Cancel context quickly
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := response(w, req)
		assert.NoError(t, err)
		assert.False(t, errorCalled, "Error handler should not be called on context cancellation")
	})

	t.Run("multiple_errors_all_handled", func(t *testing.T) {
		t.Parallel()

		items := make(chan any, 5)
		var errors []error
		mu := sync.Mutex{}

		response := response.StreamJSON(items, response.WithStreamErrorHandler(func(ctx context.Context, err error) {
			mu.Lock()
			defer mu.Unlock()
			errors = append(errors, err)
		}))

		// Mix valid and invalid items
		items <- map[string]string{"item": "1"}
		items <- make(chan int) // Invalid
		items <- map[string]string{"item": "2"}
		items <- func() {} // Invalid
		items <- map[string]string{"item": "3"}
		close(items)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.NoError(t, err)

		// Wait for processing
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 2, len(errors), "Should have 2 errors for invalid items")

		// Check valid items were still written
		output := w.Body.String()
		assert.Contains(t, output, `{"item":"1"}`)
		assert.Contains(t, output, `{"item":"2"}`)
		assert.Contains(t, output, `{"item":"3"}`)
	})
}
