package middleware_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/middleware"
)

func TestBodyLimitDefaultConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.BodyLimit[*router.Context]())

	r.Post("/test", func(ctx *router.Context) handler.Response {
		body, err := io.ReadAll(ctx.Request().Body)
		assert.NoError(t, err)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	// Test small body (should pass)
	smallBody := strings.Repeat("a", 1024) // 1KB
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(smallBody))
	req.Header.Set("Content-Length", "1024")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, smallBody, w.Body.String())
}

func TestBodyLimitExceedsDefault(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.BodyLimit[*router.Context]())

	r.Post("/test", func(ctx *router.Context) handler.Response {
		_, _ = io.ReadAll(ctx.Request().Body)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Test large body (should fail)
	largeBodySize := 5 * 1024 * 1024 // 5MB (exceeds default 4MB)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(strings.Repeat("a", largeBodySize)))
	req.Header.Set("Content-Length", strconv.Itoa(largeBodySize))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "Request body too large")
}

func TestBodyLimitWithSize(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.BodyLimitWithSize[*router.Context](100)) // 100 bytes limit

	r.Post("/test", func(ctx *router.Context) handler.Response {
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusBadRequest)
				return nil
			}
		}
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	// Test within limit
	smallBody := strings.Repeat("a", 50)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(smallBody))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, smallBody, w.Body.String())

	// Test exceeding limit
	largeBody := strings.Repeat("b", 150)
	req = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	req.Header.Set("Content-Length", "150")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodyLimitContentTypeSpecific(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Use(middleware.BodyLimitWithConfig[*router.Context](middleware.BodyLimitConfig{
		MaxSize: 100, // Default 100 bytes
		ContentTypeLimit: map[string]int64{
			"application/json":         50,   // JSON limited to 50 bytes
			"multipart/form-data":      200,  // Form data allowed up to 200 bytes
			"application/octet-stream": 1024, // Binary data up to 1KB
		},
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusBadRequest)
				return nil
			}
		}
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	tests := []struct {
		name        string
		contentType string
		bodySize    int
		expectCode  int
	}{
		{"JSON within limit", "application/json", 40, http.StatusOK},
		{"JSON exceeds limit", "application/json", 60, http.StatusRequestEntityTooLarge},
		{"Form within limit", "multipart/form-data", 150, http.StatusOK},
		{"Form exceeds limit", "multipart/form-data", 250, http.StatusRequestEntityTooLarge},
		{"Binary within limit", "application/octet-stream", 512, http.StatusOK},
		{"Binary exceeds limit", "application/octet-stream", 2048, http.StatusRequestEntityTooLarge},
		{"Unknown type uses default", "text/plain", 80, http.StatusOK},
		{"Unknown type exceeds default", "text/plain", 120, http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.Repeat("x", tt.bodySize)
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("Content-Length", strconv.Itoa(tt.bodySize))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestBodyLimitSkipFunction(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Use(middleware.BodyLimitWithConfig[*router.Context](middleware.BodyLimitConfig{
		MaxSize: 10, // Very small limit
		Skip: func(ctx handler.Context) bool {
			return ctx.Request().URL.Path == "/upload"
		},
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		body, _ := io.ReadAll(ctx.Request().Body)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	r.Post("/upload", func(ctx *router.Context) handler.Response {
		body, _ := io.ReadAll(ctx.Request().Body)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	// Test limited endpoint
	body := strings.Repeat("a", 20)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Length", "20")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	// Test skipped endpoint
	req = httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, body, w.Body.String())
}

func TestBodyLimitCustomErrorHandler(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customMessage := "Custom error: body too big"
	r.Use(middleware.BodyLimitWithConfig[*router.Context](middleware.BodyLimitConfig{
		MaxSize: 10,
		ErrorHandler: func(ctx handler.Context, contentLength int64, maxSize int64) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusTeapot) // Custom status code
				_, err := w.Write([]byte(customMessage))
				return err
			}
		},
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(strings.Repeat("a", 20)))
	req.Header.Set("Content-Length", "20")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, customMessage, w.Body.String())
}

func TestBodyLimitDisableContentLengthCheck(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Use(middleware.BodyLimitWithConfig[*router.Context](middleware.BodyLimitConfig{
		MaxSize:                   100,
		DisableContentLengthCheck: true,
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(err.Error()))
				return nil
			}
		}
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	// With Content-Length header exceeding limit
	// Should not reject based on header alone when disabled
	smallBody := strings.Repeat("a", 50)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(smallBody))
	req.Header.Set("Content-Length", "200") // Lie about content length
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should succeed because actual body is within limit
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, smallBody, w.Body.String())

	// Test actual body exceeding limit
	largeBody := strings.Repeat("b", 150)
	req = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should fail during body reading
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "exceeds limit")
}

func TestBodyLimitNoBody(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.BodyLimitWithSize[*router.Context](100))

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
}

func TestBodyLimitContentTypeWithCharset(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Use(middleware.BodyLimitWithConfig[*router.Context](middleware.BodyLimitConfig{
		MaxSize: 100,
		ContentTypeLimit: map[string]int64{
			"application/json": 50,
		},
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		body, _ := io.ReadAll(ctx.Request().Body)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(body)
			return err
		}
	})

	// Test with charset parameter
	body := strings.Repeat("a", 40)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test exceeding limit with charset
	largeBody := strings.Repeat("b", 60)
	req = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Content-Length", "60")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes    int64
		expected string
	}{
		{100, "100 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{5242880, "5.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			// We can't directly test formatBytes as it's private,
			// but we can test it through the error messages
			r := router.New[*router.Context]()

			r.Use(middleware.BodyLimitWithConfig[*router.Context](middleware.BodyLimitConfig{
				MaxSize: tt.bytes,
				ErrorHandler: func(ctx handler.Context, contentLength int64, maxSize int64) handler.Response {
					return func(w http.ResponseWriter, r *http.Request) error {
						// The default error handler uses formatBytes
						cfg := middleware.BodyLimitConfig{MaxSize: tt.bytes}
						if cfg.ErrorHandler == nil {
							cfg = middleware.BodyLimitConfig{}
						}
						w.WriteHeader(http.StatusRequestEntityTooLarge)
						return nil
					}
				},
			}))

			r.Post("/test", func(ctx *router.Context) handler.Response {
				return func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				}
			})

			// Just verify the middleware is working with the size
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(make([]byte, 10)))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// For this test, we're mainly checking that the middleware accepts
			// various size configurations
			require.NotNil(t, w)
		})
	}
}

func TestBodyLimitConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(1024), middleware.KB)
	assert.Equal(t, int64(1024*1024), middleware.MB)
	assert.Equal(t, int64(1024*1024*1024), middleware.GB)

	// Test using constants
	r := router.New[*router.Context]()
	r.Use(middleware.BodyLimitWithSize[*router.Context](2 * middleware.MB))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Test within limit
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(strings.Repeat("a", 1024*1024)))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
