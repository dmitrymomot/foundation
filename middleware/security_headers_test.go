package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
	"github.com/dmitrymomot/gokit/middleware"
)

func TestSecurityHeadersDefaultConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	r.Use(middleware.SecurityHeaders[*router.Context]())

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
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "geolocation=(), microphone=(), camera=()", w.Header().Get("Permissions-Policy"))
	assert.Equal(t, "same-origin", w.Header().Get("Cross-Origin-Opener-Policy"))
	assert.Equal(t, "require-corp", w.Header().Get("Cross-Origin-Embedder-Policy"))
	assert.Equal(t, "same-origin", w.Header().Get("Cross-Origin-Resource-Policy"))
}

func TestSecurityHeadersCustomConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customHeaders := map[string]string{
		"X-Custom-Header": "custom-value",
		"X-Another":       "another-value",
	}

	r.Use(middleware.SecurityHeadersWithConfig[*router.Context](middleware.SecurityHeadersConfig{
		ContentTypeOptions:      "custom-nosniff",
		FrameOptions:            "SAMEORIGIN",
		XSSProtection:           "0",
		StrictTransportSecurity: "",
		ContentSecurityPolicy:   "default-src 'self' https:",
		ReferrerPolicy:          "no-referrer",
		CustomHeaders:           customHeaders,
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
	assert.Equal(t, "custom-nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "0", w.Header().Get("X-XSS-Protection"))
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "default-src 'self' https:", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "another-value", w.Header().Get("X-Another"))
}

func TestSecurityHeadersDevelopmentMode(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Use(middleware.SecurityHeadersWithConfig[*router.Context](middleware.SecurityHeadersConfig{
		IsDevelopment: true,
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
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"), "HSTS should be disabled in development")
}

func TestSecurityHeadersSkipFunction(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	cfg := middleware.SecurityHeadersConfig{
		ContentTypeOptions: "nosniff",
		Skip: func(ctx handler.Context) bool {
			return strings.HasPrefix(ctx.Request().URL.Path, "/health")
		},
	}
	r.Use(middleware.SecurityHeadersWithConfig[*router.Context](cfg))

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

	// Test normal endpoint - should have headers
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"))

	// Test health endpoint - should skip headers
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-Content-Type-Options"))
}

func TestSecurityHeadersPresets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		preset            middleware.SecurityHeadersPreset
		checkFrameOptions string
		checkCSP          func(string) bool
	}{
		{
			name:              "Strict preset",
			preset:            middleware.SecurityPresetStrict,
			checkFrameOptions: "DENY",
			checkCSP: func(csp string) bool {
				return strings.Contains(csp, "default-src 'none'")
			},
		},
		{
			name:              "Balanced preset",
			preset:            middleware.SecurityPresetBalanced,
			checkFrameOptions: "SAMEORIGIN",
			checkCSP: func(csp string) bool {
				return strings.Contains(csp, "'unsafe-inline'")
			},
		},
		{
			name:              "Relaxed preset",
			preset:            middleware.SecurityPresetRelaxed,
			checkFrameOptions: "",
			checkCSP: func(csp string) bool {
				return csp == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.New[*router.Context]()
			r.Use(middleware.SecurityHeadersWithPreset[*router.Context](tt.preset))

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
			assert.Equal(t, tt.checkFrameOptions, w.Header().Get("X-Frame-Options"))
			assert.True(t, tt.checkCSP(w.Header().Get("Content-Security-Policy")))
		})
	}
}

func TestCSPBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		builder  func() *middleware.CSPBuilder
		expected string
	}{
		{
			name: "Basic CSP",
			builder: func() *middleware.CSPBuilder {
				return middleware.NewCSPBuilder().
					DefaultSrc("'self'").
					ScriptSrc("'self'", "'unsafe-inline'").
					StyleSrc("'self'", "'unsafe-inline'")
			},
			expected: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
		},
		{
			name: "Complex CSP",
			builder: func() *middleware.CSPBuilder {
				return middleware.NewCSPBuilder().
					DefaultSrc("'none'").
					ScriptSrc("'self'", "https://cdn.example.com").
					StyleSrc("'self'", "https://fonts.googleapis.com").
					ImgSrc("'self'", "data:", "https:").
					FontSrc("'self'", "https://fonts.gstatic.com").
					ConnectSrc("'self'", "https://api.example.com").
					FrameAncestors("'none'").
					BaseURI("'self'").
					FormAction("'self'")
			},
			expected: "default-src 'none'; script-src 'self' https://cdn.example.com; style-src 'self' https://fonts.googleapis.com; img-src 'self' data: https:; font-src 'self' https://fonts.gstatic.com; connect-src 'self' https://api.example.com; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csp := tt.builder().Build()
			assert.Equal(t, tt.expected, csp)
		})
	}
}

func TestSecurityHeadersWithCSPBuilder(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	csp := middleware.NewCSPBuilder().
		DefaultSrc("'self'").
		ScriptSrc("'self'", "'unsafe-inline'").
		StyleSrc("'self'", "'unsafe-inline'").
		Build()

	r.Use(middleware.SecurityHeadersWithConfig[*router.Context](middleware.SecurityHeadersConfig{
		ContentSecurityPolicy: csp,
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

	require.Equal(t, http.StatusOK, w.Code)
	actualCSP := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, actualCSP, "default-src 'self'")
	assert.Contains(t, actualCSP, "script-src 'self' 'unsafe-inline'")
	assert.Contains(t, actualCSP, "style-src 'self' 'unsafe-inline'")
}

func TestSecurityHeadersEmptyValues(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Use(middleware.SecurityHeadersWithConfig[*router.Context](middleware.SecurityHeadersConfig{
		ContentTypeOptions: "",
		FrameOptions:       "",
		XSSProtection:      "",
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
	assert.Empty(t, w.Header().Get("X-Content-Type-Options"), "Empty value should remain empty")
	assert.Empty(t, w.Header().Get("X-Frame-Options"), "Empty value should remain empty")
	assert.Empty(t, w.Header().Get("X-XSS-Protection"), "Empty value should remain empty")
}
