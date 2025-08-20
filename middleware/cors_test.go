package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
	"github.com/dmitrymomot/gokit/middleware"
)

func TestCORSDefaultConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORS[*router.Context]())

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Vary"), "Origin")
}

func TestCORSPreflightRequest(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins:     []string{"https://example.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	}))

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET,POST,PUT,DELETE", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type,Authorization", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "3600", w.Header().Get("Access-Control-Max-Age"))
	assert.Contains(t, w.Header().Values("Vary"), "Origin")
	assert.Contains(t, w.Header().Values("Vary"), "Access-Control-Request-Method")
	assert.Contains(t, w.Header().Values("Vary"), "Access-Control-Request-Headers")
}

func TestCORSPreflightRequestForbidden(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"https://allowed.com"},
		AllowMethods: []string{"GET", "POST"},
	}))

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	t.Run("forbidden origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://forbidden.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("forbidden method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://allowed.com")
		req.Header.Set("Access-Control-Request-Method", "DELETE")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCORSMultipleAllowedOrigins(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"https://example.com", "https://test.com", "http://localhost:3000"},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	testCases := []struct {
		origin   string
		expected string
		allowed  bool
	}{
		{"https://example.com", "https://example.com", true},
		{"https://test.com", "https://test.com", true},
		{"http://localhost:3000", "http://localhost:3000", true},
		{"https://forbidden.com", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tc.allowed {
				assert.Equal(t, tc.expected, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSWildcardOrigin(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSCredentialsWithWildcard(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSExposeHeaders(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins:  []string{"*"},
		ExposeHeaders: []string{"X-Request-ID", "X-Rate-Limit"},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-Request-ID", "123")
			w.Header().Set("X-Rate-Limit", "100")
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "X-Request-ID,X-Rate-Limit", w.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORSSkipFunctionality(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"https://example.com"},
		Skip: func(ctx handler.Context) bool {
			return strings.HasPrefix(ctx.Request().URL.Path, "/health")
		},
	}))

	r.Get("/health", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Get("/api/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	t.Run("skip health endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("process api endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCORSAllowOriginWildcardFunc(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOriginFunc:  middleware.AllowOriginWildcard(),
		AllowCredentials: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	testCases := []struct {
		origin      string
		expected    string
		credentials bool
	}{
		{"https://example.com", "https://example.com", true},
		{"http://localhost:3000", "http://localhost:3000", true},
		{"https://any-domain.org", "https://any-domain.org", true},
		{"", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tc.expected != "" {
				assert.Equal(t, tc.expected, w.Header().Get("Access-Control-Allow-Origin"))
				if tc.credentials {
					assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
				}
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
			}
		})
	}
}

func TestCORSAllowOriginSubdomainFunc(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOriginFunc:  middleware.AllowOriginSubdomain("example.com"),
		AllowCredentials: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	testCases := []struct {
		name     string
		origin   string
		expected string
		allowed  bool
	}{
		{"exact domain", "https://example.com", "https://example.com", true},
		{"exact domain with port", "https://example.com:8080", "https://example.com:8080", true},
		{"subdomain", "https://api.example.com", "https://api.example.com", true},
		{"nested subdomain", "https://v2.api.example.com", "https://v2.api.example.com", true},
		{"subdomain with port", "https://api.example.com:3000", "https://api.example.com:3000", true},
		{"different scheme", "http://api.example.com", "http://api.example.com", true},
		{"different domain", "https://different.com", "", false},
		{"partial match", "https://notexample.com", "", false},
		{"suffix but not subdomain", "https://fakeexample.com", "", false},
		{"empty origin", "", "", false},
		{"invalid URL", "not-a-url", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tc.allowed {
				assert.Equal(t, tc.expected, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
			}
		})
	}
}

func TestCORSAllowOriginSubdomainWithWildcardPrefix(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOriginFunc: middleware.AllowOriginSubdomain("*.example.com"),
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://api.example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://api.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSAllowOriginSubdomainWithDotPrefix(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOriginFunc: middleware.AllowOriginSubdomain(".example.com"),
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://www.example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://www.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSCustomAllowOriginFunc(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (string, bool) {
			if strings.HasPrefix(origin, "http://localhost:") {
				return origin, true
			}
			return "", false
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	testCases := []struct {
		origin   string
		expected string
		allowed  bool
	}{
		{"http://localhost:3000", "http://localhost:3000", true},
		{"http://localhost:8080", "http://localhost:8080", true},
		{"https://localhost:3000", "", false},
		{"http://example.com", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tc.allowed {
				assert.Equal(t, tc.expected, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSNoOriginHeader(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"https://example.com"},
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

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSVaryHeader(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORS[*router.Context]())

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	t.Run("regular request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		varyHeaders := w.Header().Values("Vary")
		assert.Contains(t, varyHeaders, "Origin")
	})

	t.Run("preflight request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		varyHeaders := w.Header().Values("Vary")
		assert.Contains(t, varyHeaders, "Origin")
		assert.Contains(t, varyHeaders, "Access-Control-Request-Method")
		assert.Contains(t, varyHeaders, "Access-Control-Request-Headers")
	})
}

func TestCORSMaxAge(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		maxAge int
		expect string
	}{
		{"no max age", 0, ""},
		{"1 hour", 3600, "3600"},
		{"1 day", 86400, "86400"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := router.New[*router.Context]()
			r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
				AllowOrigins: []string{"*"},
				MaxAge:       tc.maxAge,
			}))

			r.Options("/test", func(ctx *router.Context) handler.Response {
				return func(w http.ResponseWriter, r *http.Request) error {
					return nil
				}
			})

			req := httptest.NewRequest(http.MethodOptions, "/test", nil)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", "POST")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if tc.expect == "" {
				assert.Empty(t, w.Header().Get("Access-Control-Max-Age"))
			} else {
				assert.Equal(t, tc.expect, w.Header().Get("Access-Control-Max-Age"))
			}
		})
	}
}

func TestCORSMultipleMiddleware(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	executionOrder := []string{}

	corsMiddleware := middleware.CORS[*router.Context]()

	customMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "custom")
			return next(ctx)
		}
	}

	r.Use(corsMiddleware, customMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		executionOrder = append(executionOrder, "handler")
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"custom", "handler"}, executionOrder)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func BenchmarkCORSDefault(b *testing.B) {
	r := router.New[*router.Context]()
	r.Use(middleware.CORS[*router.Context]())

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORSPreflight(b *testing.B) {
	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"https://example.com"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:       3600,
	}))

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORSSubdomain(b *testing.B) {
	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOriginFunc: middleware.AllowOriginSubdomain("example.com"),
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://api.example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCORSMultipleOrigins(b *testing.B) {
	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{
			"https://example.com",
			"https://test.com",
			"https://api.example.com",
			"http://localhost:3000",
			"http://localhost:8080",
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func TestCORSCaseInsensitiveSubdomain(t *testing.T) {
	t.Parallel()

	t.Run("lowercase config", func(t *testing.T) {
		r := router.New[*router.Context]()
		r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
			AllowOriginFunc: middleware.AllowOriginSubdomain("example.com"),
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		testCases := []struct {
			name    string
			origin  string
			allowed bool
		}{
			{"lowercase domain", "https://api.example.com", true},
			{"uppercase domain", "https://API.EXAMPLE.COM", true},
			{"mixed case", "https://Api.Example.Com", true},
			{"uppercase base domain", "https://EXAMPLE.COM", true},
			{"mixed case base domain", "https://Example.Com", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Origin", tc.origin)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				if tc.allowed {
					assert.Equal(t, tc.origin, w.Header().Get("Access-Control-Allow-Origin"))
				} else {
					assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				}
			})
		}
	})

	t.Run("uppercase config", func(t *testing.T) {
		r := router.New[*router.Context]()
		r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
			AllowOriginFunc: middleware.AllowOriginSubdomain("EXAMPLE.COM"),
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		testCases := []struct {
			name    string
			origin  string
			allowed bool
		}{
			{"lowercase subdomain", "https://api.example.com", true},
			{"uppercase subdomain", "https://API.EXAMPLE.COM", true},
			{"mixed case", "https://Api.Example.Com", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Origin", tc.origin)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				if tc.allowed {
					assert.Equal(t, tc.origin, w.Header().Get("Access-Control-Allow-Origin"))
				} else {
					assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				}
			})
		}
	})
}

func TestCORSDefaultAllowedHeaders(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORS[*router.Context]())

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowedHeaders, "Accept")
	assert.Contains(t, allowedHeaders, "Content-Type")
	assert.Contains(t, allowedHeaders, "Authorization")
	assert.Contains(t, allowedHeaders, "X-Request-ID")
}

func TestCORSDefaultAllowedMethods(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORS[*router.Context]())

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	allowedMethods := w.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "HEAD", "PUT", "PATCH", "POST", "DELETE"} {
		assert.Contains(t, allowedMethods, method)
	}
}

func TestCORSMaxAgeString(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.CORSWithConfig[*router.Context](middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		MaxAge:       7200,
	}))

	r.Options("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	maxAgeHeader := w.Header().Get("Access-Control-Max-Age")
	assert.Equal(t, "7200", maxAgeHeader)

	maxAge, err := strconv.Atoi(maxAgeHeader)
	require.NoError(t, err)
	assert.Equal(t, 7200, maxAge)
}
