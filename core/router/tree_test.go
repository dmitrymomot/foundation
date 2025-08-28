package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
)

func TestTreeStaticRoutes(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register static routes
	routes := []string{
		"/",
		"/users",
		"/users/profile",
		"/admin",
		"/admin/users",
		"/api/v1/posts",
		"/api/v2/posts",
	}

	for _, route := range routes {
		r.Get(route, func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(route))
				return nil
			}
		})
	}

	// Test each route
	for _, route := range routes {
		t.Run("route_"+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, route, w.Body.String())
		})
	}
}

func TestTreeParameterRoutes(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register parameter routes
	r.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("user:" + ctx.Param("id")))
			return nil
		}
	})

	r.Get("/users/{id}/posts/{postId}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("user:" + ctx.Param("id") + ",post:" + ctx.Param("postId")))
			return nil
		}
	})

	r.Get("/products/{category}/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("category:" + ctx.Param("category") + ",id:" + ctx.Param("id")))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/users/123", "user:123"},
		{"/users/abc", "user:abc"},
		{"/users/123/posts/456", "user:123,post:456"},
		{"/users/john/posts/hello-world", "user:john,post:hello-world"},
		{"/products/electronics/laptop", "category:electronics,id:laptop"},
		{"/products/books/golang-guide", "category:books,id:golang-guide"},
	}

	for _, test := range tests {
		name := strings.ReplaceAll(test.path, "/", "_")
		name = strings.ReplaceAll(name, " ", "_")
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}

func TestTreeRegexpRoutes(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register regexp routes
	r.Get("/users/{id:[0-9]+}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("numeric_user:" + ctx.Param("id")))
			return nil
		}
	})

	r.Get("/files/{filename:[a-zA-Z0-9._-]+}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("file:" + ctx.Param("filename")))
			return nil
		}
	})

	r.Get("/posts/{slug:[a-z-]+}/{id:[0-9]+}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("slug:" + ctx.Param("slug") + ",id:" + ctx.Param("id")))
			return nil
		}
	})

	tests := []struct {
		path        string
		expected    string
		shouldMatch bool
	}{
		// Matching cases
		{"/users/123", "numeric_user:123", true},
		{"/users/456789", "numeric_user:456789", true},
		{"/files/document.pdf", "file:document.pdf", true},
		{"/files/image_file.jpg", "file:image_file.jpg", true},
		{"/files/test-file.txt", "file:test-file.txt", true},
		{"/posts/hello-world/123", "slug:hello-world,id:123", true},
		{"/posts/go-programming/456", "slug:go-programming,id:456", true},

		// Non-matching cases (should return 500 - not found with default error handler)
		{"/users/abc", "", false},             // non-numeric user id
		{"/users/12a3", "", false},            // mixed alphanumeric
		{"/files/bad%20file.txt", "", false},  // spaces not allowed (URL encoded)
		{"/files/bad@file.txt", "", false},    // @ not allowed
		{"/posts/hello_world/123", "", false}, // underscore not allowed in slug
		{"/posts/hello-world/abc", "", false}, // non-numeric id
	}

	for _, test := range tests {
		name := strings.ReplaceAll(test.path, "/", "_")
		name = strings.ReplaceAll(name, " ", "_")
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if test.shouldMatch {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, test.expected, w.Body.String())
			} else {
				assert.Equal(t, http.StatusInternalServerError, w.Code) // Default error handler
			}
		})
	}
}

func TestTreeWildcardRoutes(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/static/*", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("static:" + ctx.Param("*")))
			return nil
		}
	})

	r.Get("/files/{dir}/*", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("dir:" + ctx.Param("dir") + ",path:" + ctx.Param("*")))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/static/css/main.css", "static:css/main.css"},
		{"/static/js/app.js", "static:js/app.js"},
		{"/static/images/logo.png", "static:images/logo.png"},
		{"/static/fonts/roboto/regular.woff", "static:fonts/roboto/regular.woff"},
		{"/files/uploads/image.jpg", "dir:uploads,path:image.jpg"},
		{"/files/documents/pdf/manual.pdf", "dir:documents,path:pdf/manual.pdf"},
		{"/files/media/video/demo.mp4", "dir:media,path:video/demo.mp4"},
	}

	for _, test := range tests {
		name := strings.ReplaceAll(test.path, "/", "_")
		name = strings.ReplaceAll(name, " ", "_")
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}

func TestTreeRoutePriority(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register routes in order that tests priority
	// Static routes should have highest priority
	r.Get("/users/admin", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("admin"))
			return nil
		}
	})

	// Regex should have higher priority than regular params
	r.Get("/users/{id:[0-9]+}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("numeric:" + ctx.Param("id")))
			return nil
		}
	})

	// Regular parameter should have lower priority
	r.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("param:" + ctx.Param("id")))
			return nil
		}
	})

	// Wildcard should have lowest priority
	r.Get("/users/*", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("wildcard:" + ctx.Param("*")))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/users/admin", "admin"},                            // Static route wins
		{"/users/123", "numeric:123"},                        // Regex route wins over param
		{"/users/abc", "param:abc"},                          // Regular param wins over wildcard
		{"/users/something/else", "wildcard:something/else"}, // Wildcard catches all else
	}

	for _, test := range tests {
		t.Run("priority_"+test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}

func TestTreeComplexParameterExtraction(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Route with multiple parameter types
	r.Get("/api/{version}/users/{id:[0-9]+}/posts/{slug:[a-z-]+}/comments/{commentId}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			params := map[string]string{
				"version":   ctx.Param("version"),
				"id":        ctx.Param("id"),
				"slug":      ctx.Param("slug"),
				"commentId": ctx.Param("commentId"),
			}
			response := ""
			for k, v := range params {
				if response != "" {
					response += ","
				}
				response += k + ":" + v
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/posts/hello-world/comments/comment456", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()

	// Check that all parameters are present
	assert.Contains(t, body, "version:v1")
	assert.Contains(t, body, "id:123")
	assert.Contains(t, body, "slug:hello-world")
	assert.Contains(t, body, "commentId:comment456")
}

func TestTreeParameterConstraintsValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid regex patterns panic", func(t *testing.T) {
		assert.Panics(t, func() {
			r := router.New[*router.Context]()
			// Invalid regex pattern should panic
			r.Get("/test/{id:[}", func(ctx *router.Context) handler.Response { return nil })
		})
	})

	t.Run("duplicate parameter names panic", func(t *testing.T) {
		assert.Panics(t, func() {
			r := router.New[*router.Context]()
			// Duplicate parameter names should panic
			r.Get("/test/{id}/{id}", func(ctx *router.Context) handler.Response { return nil })
		})
	})

	t.Run("wildcard not at end panics", func(t *testing.T) {
		assert.Panics(t, func() {
			r := router.New[*router.Context]()
			// Wildcard not at end should panic
			r.Get("/test/*/more", func(ctx *router.Context) handler.Response { return nil })
		})
	})

	t.Run("missing parameter delimiter panics", func(t *testing.T) {
		assert.Panics(t, func() {
			r := router.New[*router.Context]()
			// Missing closing brace should panic
			r.Get("/test/{id", func(ctx *router.Context) handler.Response { return nil })
		})
	})
}

func TestTreeParameterEdgeCases(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Route with parameter at end without delimiter
	r.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("id:" + ctx.Param("id")))
			return nil
		}
	})

	// Route with parameter followed by static segment
	r.Get("/posts/{id}/edit", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("edit:" + ctx.Param("id")))
			return nil
		}
	})

	// Test parameter extraction at path end
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "id:123", w.Body.String())

	// Test parameter followed by static segment
	req = httptest.NewRequest(http.MethodGet, "/posts/456/edit", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "edit:456", w.Body.String())
}

func TestTreeEmptyParameterHandling(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/test/{param}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			param := ctx.Param("param")
			if param == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("empty"))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("param:" + param))
			}
			return nil
		}
	})

	// Test with missing parameter - should not match route
	req := httptest.NewRequest(http.MethodGet, "/test/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Should get not found error (500 with default error handler)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Test with actual parameter
	req = httptest.NewRequest(http.MethodGet, "/test/value", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "param:value", w.Body.String())
}

func TestTreeRouteOverwriting(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register initial route
	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("first"))
			return nil
		}
	})

	// Overwrite with same route - should replace handler
	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("second"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "second", w.Body.String())
}

func TestTreeLongestPrefixMatching(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register routes that test longest prefix matching
	r.Get("/api", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("api"))
			return nil
		}
	})

	r.Get("/api/v1", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("api-v1"))
			return nil
		}
	})

	r.Get("/api/v1/users", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("api-v1-users"))
			return nil
		}
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/api", "api"},
		{"/api/v1", "api-v1"},
		{"/api/v1/users", "api-v1-users"},
	}

	for _, test := range tests {
		t.Run("prefix_"+test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}

func TestTreeSpecialCharactersInParams(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/files/{filename}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("file:" + ctx.Param("filename")))
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
		{"/files/test%20file.txt", "file:test%20file.txt"}, // URL encoded space
		{"/files/test.backup.txt", "file:test.backup.txt"},
	}

	for _, test := range tests {
		t.Run("special_chars_"+test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			// Preserve encoded path if it contains encoded characters
			if strings.Contains(test.path, "%") {
				req.URL.RawPath = test.path
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, test.expected, w.Body.String())
		})
	}
}
