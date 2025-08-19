package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
)

func TestContextImplementsHandlerContext(t *testing.T) {
	t.Parallel()

	ctx := &router.Context{}
	var _ handler.Context = ctx
	var _ context.Context = ctx

	assert.NotNil(t, ctx)
}

func TestContextRequest(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	var capturedRequest *http.Request

	r.Get("/test", func(ctx *router.Context) handler.Response {
		capturedRequest = ctx.Request()
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	originalReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	originalReq.Header.Set("X-Test-Header", "test-value")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, originalReq)

	require.NotNil(t, capturedRequest)
	assert.Equal(t, originalReq.Method, capturedRequest.Method)
	assert.Equal(t, originalReq.URL, capturedRequest.URL)
	assert.Equal(t, "test-value", capturedRequest.Header.Get("X-Test-Header"))
	assert.Equal(t, originalReq, capturedRequest) // Should be the same instance
}

func TestContextResponseWriter(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	var capturedWriter http.ResponseWriter

	r.Get("/test", func(ctx *router.Context) handler.Response {
		capturedWriter = ctx.ResponseWriter()
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.NotNil(t, capturedWriter)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// The captured writer should be wrapped responseWriter, not the original
	assert.NotEqual(t, w, capturedWriter)
}

func TestContextParam(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/users/{id}/posts/{postId}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			userID := ctx.Param("id")
			postID := ctx.Param("postId")
			nonExistent := ctx.Param("nonexistent")

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("user:" + userID + ",post:" + postID + ",missing:" + nonExistent))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user:123,post:456,missing:", w.Body.String())
}

func TestContextParamWithEmptyParams(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/static", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Route has no parameters
			param := ctx.Param("id")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("param:" + param))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/static", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "param:", w.Body.String()) // Empty string for non-existent param
}

func TestContextParamSpecialCharacters(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/files/{filename}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			filename := ctx.Param("filename")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("file:" + filename))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/files/test.txt", "file:test.txt"},
		{"/files/test_file.txt", "file:test_file.txt"},
		{"/files/test-file.txt", "file:test-file.txt"},
		{"/files/test%20with%20spaces.txt", "file:test%20with%20spaces.txt"}, // URL encoded
		{"/files/file.backup.old", "file:file.backup.old"},
	}

	for _, test := range tests {
		t.Run("special_"+test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			// Preserve the raw path for URL-encoded tests
			if test.path == "/files/test%20with%20spaces.txt" {
				req.URL.RawPath = test.path
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}

func TestContextStandardContextMethods(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Test context.Context interface methods
			deadline, hasDeadline := ctx.Deadline()
			done := ctx.Done()
			err := ctx.Err()
			value := ctx.Value("test-key")

			_ = hasDeadline
			_ = deadline
			_ = done
			_ = err
			_ = value

			w.WriteHeader(http.StatusOK)
			// For a context without deadline (like context.Background()), Done() returns nil
			// This is correct behavior per Go's context documentation
			if !hasDeadline && deadline.IsZero() && done == nil && err == nil && value == nil {
				w.Write([]byte("context_ok"))
			} else {
				w.Write([]byte("context_fail"))
			}
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "context_ok", w.Body.String())
}

func TestContextWithRequestContext(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Test that context methods delegate to request context
			reqCtx := r.Context()

			// These should be equal since Context delegates to request context
			deadline1, ok1 := ctx.Deadline()
			deadline2, ok2 := reqCtx.Deadline()

			done1 := ctx.Done()
			done2 := reqCtx.Done()

			err1 := ctx.Err()
			err2 := reqCtx.Err()

			w.WriteHeader(http.StatusOK)
			if ok1 == ok2 && deadline1.Equal(deadline2) && done1 == done2 && err1 == err2 {
				w.Write([]byte("delegation_ok"))
			} else {
				w.Write([]byte("delegation_fail"))
			}
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "delegation_ok", w.Body.String())
}

func TestContextWithCancelledRequest(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Check if context reports cancellation properly
			select {
			case <-ctx.Done():
				w.WriteHeader(http.StatusRequestTimeout)
				w.Write([]byte("cancelled"))
			default:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not_cancelled"))
			}
			return nil
		}
	})

	// Create request with cancelled context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	cancelCtx, cancel := context.WithCancel(req.Context())
	cancel() // Cancel immediately
	req = req.WithContext(cancelCtx)

	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, "cancelled", w.Body.String())
}

func TestContextWithTimeout(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			deadline, hasDeadline := ctx.Deadline()
			w.WriteHeader(http.StatusOK)
			if hasDeadline && !deadline.IsZero() {
				w.Write([]byte("has_deadline"))
			} else {
				w.Write([]byte("no_deadline"))
			}
			return nil
		}
	})

	// Create request with timeout context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	timeoutCtx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "has_deadline", w.Body.String())
}

func TestContextValue(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			value := ctx.Value("test-key")
			w.WriteHeader(http.StatusOK)
			if value != nil {
				w.Write([]byte("value:" + value.(string)))
			} else {
				w.Write([]byte("no_value"))
			}
			return nil
		}
	})

	// Create request with value in context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	valueCtx := context.WithValue(req.Context(), "test-key", "test-value")
	req = req.WithContext(valueCtx)

	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "value:test-value", w.Body.String())
}

func TestContextParamWithWildcard(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/files/{dir}/*", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			dir := ctx.Param("dir")
			wildcard := ctx.Param("*")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("dir:" + dir + ",path:" + wildcard))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/files/uploads/image.jpg", "dir:uploads,path:image.jpg"},
		{"/files/documents/pdf/manual.pdf", "dir:documents,path:pdf/manual.pdf"},
		{"/files/media/", "dir:media,path:"},
	}

	for _, test := range tests {
		t.Run("wildcard_"+test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}

func TestContextParamRegexpConstraints(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/users/{id:[0-9]+}/posts/{slug:[a-z-]+}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			id := ctx.Param("id")
			slug := ctx.Param("slug")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("id:" + id + ",slug:" + slug))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
		matches  bool
	}{
		{"/users/123/posts/hello-world", "id:123,slug:hello-world", true},
		{"/users/456/posts/go-programming", "id:456,slug:go-programming", true},
		{"/users/abc/posts/hello-world", "", false}, // non-numeric id
		{"/users/123/posts/Hello-World", "", false}, // uppercase not allowed
	}

	for _, test := range tests {
		t.Run("regexp_"+test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if test.matches {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, test.expected, w.Body.String())
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code) // Not found with default handler
			}
		})
	}
}

func TestContextMultipleParamsExtraction(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/{p1}/{p2}/{p3}/{p4}/{p5}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			params := []string{
				ctx.Param("p1"),
				ctx.Param("p2"),
				ctx.Param("p3"),
				ctx.Param("p4"),
				ctx.Param("p5"),
			}
			response := ""
			for i, param := range params {
				if i > 0 {
					response += ","
				}
				response += param
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/a/b/c/d/e", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "a,b,c,d,e", w.Body.String())
}
