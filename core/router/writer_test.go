package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
)

func TestResponseWriterStatusTracking(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("created"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "created", w.Body.String())
}

func TestResponseWriterDefaultStatus(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Write without explicitly setting status - should default to 200
			w.Write([]byte("default status"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "default status", w.Body.String())
}

func TestResponseWriterMultipleWriteHeaders(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// First WriteHeader should work
			w.WriteHeader(http.StatusCreated)

			// Second WriteHeader should be ignored
			w.WriteHeader(http.StatusBadRequest)

			w.Write([]byte("first status wins"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should keep the first status
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "first status wins", w.Body.String())
}

func TestResponseWriterHeaderManipulation(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Set custom headers before writing
			w.Header().Set("X-Custom-Header", "custom-value")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Add("X-Multi-Header", "value1")
			w.Header().Add("X-Multi-Header", "value2")

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"status": "accepted"}`))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Check multi-value header
	multiHeader := w.Header()["X-Multi-Header"]
	require.Len(t, multiHeader, 2)
	assert.Contains(t, multiHeader, "value1")
	assert.Contains(t, multiHeader, "value2")
}

func TestResponseWriterFlusherInterface(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("chunk1"))

			// Test if Flusher interface is supported
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			w.Write([]byte("chunk2"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "chunk1chunk2", w.Body.String())

	// Note: httptest.ResponseRecorder doesn't implement Flusher,
	// but the router's responseWriter should handle this gracefully
}

func TestResponseWriterWrittenFlag(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	var responseWritten bool

	// Custom error handler that writes error response
	errorHandler := func(ctx *router.Context, err error) {
		w := ctx.ResponseWriter()
		// Just try to write - if response was already written,
		// this will append (which is a handler bug, not router's problem)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error handled"))
	}

	r = router.New[*router.Context](router.WithErrorHandler(errorHandler))

	r.Get("/write-then-error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			responseWritten = true
			// Return error after writing - buffered response will be discarded
			return assert.AnError
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/write-then-error", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, responseWritten)
	// Without buffering, once response is written, it's committed
	// Error handler will append to it (this is a handler bug)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "successerror handled", w.Body.String())
}

func TestResponseWriterEmptyWrite(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/empty", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusNoContent)
			// Write empty byte slice
			w.Write([]byte{})
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestResponseWriterLargeWrite(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/large", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Create large content
			largeContent := make([]byte, 10000)
			for i := range largeContent {
				largeContent[i] = byte('A' + (i % 26))
			}

			w.WriteHeader(http.StatusOK)
			w.Write(largeContent)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, w.Body.Bytes(), 10000)

	// Check that content is correct
	body := w.Body.Bytes()
	for i := range body {
		expected := byte('A' + (i % 26))
		assert.Equal(t, expected, body[i], "Byte at position %d should be %c but was %c", i, expected, body[i])

		// Only check first few bytes to avoid excessive output
		if i >= 100 {
			break
		}
	}
}

func TestResponseWriterMultipleWrites(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/multiple", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)

			// Multiple writes should all work
			w.Write([]byte("part1"))
			w.Write([]byte("-"))
			w.Write([]byte("part2"))
			w.Write([]byte("-"))
			w.Write([]byte("part3"))

			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/multiple", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "part1-part2-part3", w.Body.String())
}

func TestResponseWriterStatusAfterWrite(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/status-after-write", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Write content first (should default to 200)
			w.Write([]byte("content written first"))

			// Try to set status after write (should be ignored)
			w.WriteHeader(http.StatusBadRequest)

			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/status-after-write", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // Default status since Write was called first
	assert.Equal(t, "content written first", w.Body.String())
}

func TestResponseWriterHeaderAfterWrite(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/header-after-write", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Set initial header and write
			w.Header().Set("X-Before-Write", "before")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("content"))

			// Try to set header after write
			w.Header().Set("X-After-Write", "after")

			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/header-after-write", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "content", w.Body.String())
	assert.Equal(t, "before", w.Header().Get("X-Before-Write"))

	// Header set after write might or might not be present depending on implementation
	// This is testing edge case behavior
}

func TestResponseWriterStreamingResponse(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/stream", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)

			// Simulate streaming response
			for i := 0; i < 5; i++ {
				w.Write([]byte("chunk "))

				// In real streaming, you'd flush here
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}

			w.Write([]byte("done"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Equal(t, "chunk chunk chunk chunk chunk done", w.Body.String())
}

func TestResponseWriterInterfaceCompliance(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/interface-test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Test that the wrapped writer implements http.ResponseWriter
			var _ http.ResponseWriter = w

			// Test Flusher interface (optional)
			if flusher, ok := w.(http.Flusher); ok {
				_ = flusher // Use it to avoid unused variable
			}

			// Test CloseNotifier interface (deprecated but might be used)
			if notifier, ok := w.(http.CloseNotifier); ok {
				_ = notifier // Use it to avoid unused variable
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("interface compliance test"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/interface-test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "interface compliance test", w.Body.String())
}

func TestResponseWriterCustomStatus(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customStatuses := []int{
		http.StatusContinue,
		http.StatusSwitchingProtocols,
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusNotModified,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusTeapot,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
	}

	for _, status := range customStatuses {
		t.Run("status_"+http.StatusText(status), func(t *testing.T) {
			localStatus := status // Capture for closure
			r.Get("/status", func(ctx *router.Context) handler.Response {
				return func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(localStatus)
					w.Write([]byte(http.StatusText(localStatus)))
					return nil
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, localStatus, w.Code)
			assert.Equal(t, http.StatusText(localStatus), w.Body.String())
		})
	}
}

func TestResponseWriterHijackerInterface(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/hijack", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Test if Hijacker interface is supported
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("hijacker not supported"))
				return nil
			}

			// Since httptest.ResponseRecorder doesn't implement Hijacker,
			// we expect this to fail, but we're testing the interface is available
			conn, bufrw, err := hijacker.Hijack()
			if err != nil {
				// Expected for httptest.ResponseRecorder
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("hijacker interface available but not supported by test recorder"))
				return nil
			}

			// If we got here, we have a real connection (won't happen in test)
			defer conn.Close()
			bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n\r\n")
			bufrw.Flush()
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/hijack", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// httptest.ResponseRecorder doesn't implement Hijacker,
	// but our wrapper should still expose the interface
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hijacker interface available but not supported by test recorder", w.Body.String())
}

func TestResponseWriterHijackerNotSupported(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	hijackAttempted := false
	hijackError := ""

	r.Get("/no-hijack", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Try to hijack
			if hijacker, ok := w.(http.Hijacker); ok {
				hijackAttempted = true
				_, _, err := hijacker.Hijack()
				if err != nil {
					hijackError = err.Error()
				}
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("response"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/no-hijack", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, hijackAttempted)
	assert.Contains(t, hijackError, "hijack") // The error message should mention hijacking
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "response", w.Body.String())
}

func TestResponseWriterPusherInterface(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/push", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Test if Pusher interface is supported
			pusher, ok := w.(http.Pusher)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("pusher not supported"))
				return nil
			}

			// Try to push a resource
			err := pusher.Push("/static/style.css", &http.PushOptions{
				Header: http.Header{
					"Accept-Encoding": []string{"gzip"},
				},
			})

			if err == http.ErrNotSupported {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("pusher interface available but not supported"))
				return nil
			}

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("push failed: " + err.Error()))
				return nil
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("push succeeded"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/push", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// httptest.ResponseRecorder doesn't implement Pusher,
	// but our wrapper should still expose the interface
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pusher interface available but not supported", w.Body.String())
}

func TestResponseWriterPusherNotSupported(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	pushAttempted := false
	pushError := error(nil)

	r.Get("/no-push", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Try to push
			if pusher, ok := w.(http.Pusher); ok {
				pushAttempted = true
				pushError = pusher.Push("/resource", nil)
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("response"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/no-push", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, pushAttempted)
	assert.Equal(t, http.ErrNotSupported, pushError)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "response", w.Body.String())
}

func TestResponseWriterAllInterfaces(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/all-interfaces", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Verify all interfaces are available
			interfaces := []string{}

			// Check basic ResponseWriter
			var _ http.ResponseWriter = w
			interfaces = append(interfaces, "ResponseWriter")

			// Check Flusher
			if _, ok := w.(http.Flusher); ok {
				interfaces = append(interfaces, "Flusher")
			}

			// Check Hijacker
			if _, ok := w.(http.Hijacker); ok {
				interfaces = append(interfaces, "Hijacker")
			}

			// Check Pusher
			if _, ok := w.(http.Pusher); ok {
				interfaces = append(interfaces, "Pusher")
			}

			w.WriteHeader(http.StatusOK)
			for i, iface := range interfaces {
				if i > 0 {
					w.Write([]byte(","))
				}
				w.Write([]byte(iface))
			}
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/all-interfaces", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "ResponseWriter")
	assert.Contains(t, body, "Flusher")
	assert.Contains(t, body, "Hijacker")
	assert.Contains(t, body, "Pusher")
}

func TestResponseWriterWebSocketUpgrade(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/ws", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Check for WebSocket upgrade headers
			if r.Header.Get("Upgrade") != "websocket" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("not a websocket request"))
				return nil
			}

			// Verify we can access Hijacker for WebSocket upgrade
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("websocket upgrade not supported"))
				return nil
			}

			// In real WebSocket upgrade, we would:
			// 1. Send 101 Switching Protocols
			// 2. Hijack the connection
			// 3. Perform WebSocket handshake
			// But in test, we just verify the interface is available
			_ = hijacker
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("websocket upgrade ready"))
			return nil
		}
	})

	// Test with WebSocket headers
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "websocket upgrade ready", w.Body.String())

	// Test without WebSocket headers
	req2 := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusBadRequest, w2.Code)
	assert.Equal(t, "not a websocket request", w2.Body.String())
}
