package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/middleware"
	"github.com/dmitrymomot/foundation/pkg/fingerprint"
)

func TestFingerprintDefaultConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.Fingerprint[*router.Context]()
	r.Use(fingerprintMiddleware)

	var capturedFP string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, ok := middleware.GetFingerprint(ctx)
		assert.True(t, ok, "Fingerprint should be present in context by default")
		capturedFP = fp
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, capturedFP, "Fingerprint should be generated")
	assert.Len(t, capturedFP, 35, "Fingerprint should be 35 characters (v1: + 32 hex)")
	assert.Regexp(t, "^v1:[a-f0-9]{32}$", capturedFP, "Fingerprint should be v1:hash format")
	assert.Empty(t, w.Header().Get("X-Device-Fingerprint"), "Default config should not set header")
}

func TestFingerprintStoreInHeader(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInHeader: true,
	})
	r.Use(fingerprintMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	headerFP := w.Header().Get("X-Device-Fingerprint")
	assert.NotEmpty(t, headerFP, "Fingerprint should be in response header")
	assert.Len(t, headerFP, 35, "Header fingerprint should be 35 characters (v1: + 32 hex)")
}

func TestFingerprintCustomHeaderName(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customHeaderName := "X-Client-ID"
	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		HeaderName:    customHeaderName,
		StoreInHeader: true,
	})
	r.Use(fingerprintMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get(customHeaderName), "Custom header should be set")
	assert.Empty(t, w.Header().Get("X-Device-Fingerprint"), "Default header should not be set")
}

func TestFingerprintSkipFunctionality(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		Skip: func(ctx handler.Context) bool {
			return strings.HasPrefix(ctx.Request().URL.Path, "/static")
		},
		StoreInContext: true,
	})
	r.Use(fingerprintMiddleware)

	r.Get("/static/css/style.css", func(ctx *router.Context) handler.Response {
		fp, ok := middleware.GetFingerprint(ctx)
		assert.False(t, ok, "Fingerprint should not be present for skipped routes")
		assert.Empty(t, fp)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Get("/api/data", func(ctx *router.Context) handler.Response {
		fp, ok := middleware.GetFingerprint(ctx)
		assert.True(t, ok, "Fingerprint should be present for non-skipped routes")
		assert.NotEmpty(t, fp)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	t.Run("skip static route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("process api route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestFingerprintValidateFunc(t *testing.T) {
	t.Parallel()

	t.Run("validation passes", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		var validatedFP string
		fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
			ValidateFunc: func(ctx handler.Context, fp string) error {
				validatedFP = fp
				// Simulate successful validation
				return nil
			},
		})
		r.Use(fingerprintMiddleware)

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
		assert.NotEmpty(t, validatedFP, "Fingerprint should be passed to validation func")
	})

	t.Run("validation fails", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		validationError := errors.New("fingerprint validation failed")
		fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
			ValidateFunc: func(ctx handler.Context, fp string) error {
				// Simulate validation failure
				return validationError
			},
		})
		r.Use(fingerprintMiddleware)

		handlerExecuted := false
		r.Get("/test", func(ctx *router.Context) handler.Response {
			handlerExecuted = true
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Error should be handled with BadRequest status
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.False(t, handlerExecuted, "Handler should not execute when validation fails")
	})
}

func TestFingerprintConsistency(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
	})
	r.Use(fingerprintMiddleware)

	fingerprints := make([]string, 0, 3)
	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, _ := middleware.GetFingerprint(ctx)
		fingerprints = append(fingerprints, fp)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Make multiple requests with same headers
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req.Header.Set("Accept", "text/html")
		req.Header.Set("Accept-Language", "en-US")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	require.Len(t, fingerprints, 3)
	assert.Equal(t, fingerprints[0], fingerprints[1], "Same request should produce same fingerprint")
	assert.Equal(t, fingerprints[1], fingerprints[2], "Same request should produce same fingerprint")
}

func TestFingerprintDifferentRequests(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
	})
	r.Use(fingerprintMiddleware)

	fingerprints := make([]string, 0, 2)
	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, _ := middleware.GetFingerprint(ctx)
		fingerprints = append(fingerprints, fp)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("User-Agent", "Mozilla/5.0 (Windows)")
	req1.Header.Set("Accept", "text/html")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	// Second request with different headers
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Mac)")
	req2.Header.Set("Accept", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	require.Len(t, fingerprints, 2)
	assert.NotEqual(t, fingerprints[0], fingerprints[1], "Different requests should produce different fingerprints")
}

func TestFingerprintWithMultipleMiddleware(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	var fingerprintInMiddleware2, fingerprintInHandler string

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
	})

	middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			fp, ok := middleware.GetFingerprint(ctx)
			assert.True(t, ok, "Fingerprint should be available in subsequent middleware")
			fingerprintInMiddleware2 = fp
			return next(ctx)
		}
	}

	r.Use(fingerprintMiddleware, middleware2)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, _ := middleware.GetFingerprint(ctx)
		fingerprintInHandler = fp
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, fingerprintInMiddleware2)
	assert.Equal(t, fingerprintInMiddleware2, fingerprintInHandler, "Fingerprint should be consistent across middleware")
}

func TestFingerprintContextNotFound(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Handler without fingerprint middleware
	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, ok := middleware.GetFingerprint(ctx)
		assert.False(t, ok, "Fingerprint should not be found when middleware not used")
		assert.Empty(t, fp, "Fingerprint should be empty when not found")
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

func TestFingerprintStoreInContextFalse(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: false,
		StoreInHeader:  true, // Must do something with the fingerprint
	})
	r.Use(fingerprintMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, ok := middleware.GetFingerprint(ctx)
		assert.False(t, ok, "Fingerprint should not be in context when StoreInContext is false")
		assert.Empty(t, fp)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Device-Fingerprint"), "Fingerprint should still be in header")
}

func TestFingerprintSessionValidation(t *testing.T) {
	t.Parallel()

	// Simulate a session fingerprint storage
	sessionFingerprints := make(map[string]string)

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
		ValidateFunc: func(ctx handler.Context, fp string) error {
			// Get session ID from cookie (simulated)
			cookie, err := ctx.Request().Cookie("session")
			if err != nil {
				return nil // No session, skip validation
			}

			storedFP, exists := sessionFingerprints[cookie.Value]
			if exists && storedFP != fp {
				return errors.New("session fingerprint mismatch - possible hijacking")
			}

			// Store fingerprint for new sessions
			if !exists {
				sessionFingerprints[cookie.Value] = fp
			}
			return nil
		},
	})
	r.Use(fingerprintMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		}
	})

	// First request - establish session
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("User-Agent", "Mozilla/5.0 (Original)")
	req1.AddCookie(&http.Cookie{Name: "session", Value: "session123"})
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request - same session, same fingerprint
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Original)")
	req2.AddCookie(&http.Cookie{Name: "session", Value: "session123"})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Third request - same session, different fingerprint (potential hijacking)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.Header.Set("User-Agent", "Mozilla/5.0 (Hijacker)")
	req3.AddCookie(&http.Cookie{Name: "session", Value: "session123"})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusBadRequest, w3.Code, "Should reject mismatched fingerprint")
}

func TestFingerprintIntegrationWithActualGenerate(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
	})
	r.Use(fingerprintMiddleware)

	var capturedFP string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		fp, _ := middleware.GetFingerprint(ctx)
		capturedFP = fp
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify it matches what the fingerprint package would generate
	expectedFP := fingerprint.Generate(req)
	assert.Equal(t, expectedFP, capturedFP, "Middleware should use fingerprint.Generate correctly")
}

func TestFingerprintWithOptions(t *testing.T) {
	t.Parallel()

	t.Run("strict mode with WithIP option", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
			StoreInContext: true,
			Options:        []fingerprint.Option{fingerprint.WithIP()},
		})
		r.Use(fingerprintMiddleware)

		fingerprints := make([]string, 0, 2)
		r.Get("/test", func(ctx *router.Context) handler.Response {
			fp, _ := middleware.GetFingerprint(ctx)
			fingerprints = append(fingerprints, fp)
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		// First request from IP 192.168.1.100
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req1.Header.Set("Accept", "text/html")
		req1.RemoteAddr = "192.168.1.100:12345"
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		// Second request from different IP but same headers
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req2.Header.Set("Accept", "text/html")
		req2.RemoteAddr = "192.168.1.101:12345"
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		require.Len(t, fingerprints, 2)
		assert.NotEqual(t, fingerprints[0], fingerprints[1], "WithIP option should make different IPs produce different fingerprints")
	})

	t.Run("JWT mode with WithoutAcceptHeaders option", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
			StoreInContext: true,
			Options:        []fingerprint.Option{fingerprint.WithoutAcceptHeaders()},
		})
		r.Use(fingerprintMiddleware)

		fingerprints := make([]string, 0, 2)
		r.Get("/test", func(ctx *router.Context) handler.Response {
			fp, _ := middleware.GetFingerprint(ctx)
			fingerprints = append(fingerprints, fp)
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		// First request with Accept: text/html
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req1.Header.Set("Accept", "text/html")
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		// Second request with Accept: application/json
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req2.Header.Set("Accept", "application/json")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		require.Len(t, fingerprints, 2)
		assert.Equal(t, fingerprints[0], fingerprints[1], "WithoutAcceptHeaders option should ignore Accept header differences")
	})

	t.Run("default mode excludes IP", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		// Default configuration (no Options specified)
		fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
			StoreInContext: true,
		})
		r.Use(fingerprintMiddleware)

		fingerprints := make([]string, 0, 2)
		r.Get("/test", func(ctx *router.Context) handler.Response {
			fp, _ := middleware.GetFingerprint(ctx)
			fingerprints = append(fingerprints, fp)
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		// First request from IP 192.168.1.100
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req1.Header.Set("Accept", "text/html")
		req1.RemoteAddr = "192.168.1.100:12345"
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		// Second request from different IP but same headers
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
		req2.Header.Set("Accept", "text/html")
		req2.RemoteAddr = "192.168.1.101:12345"
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		require.Len(t, fingerprints, 2)
		assert.Equal(t, fingerprints[0], fingerprints[1], "Default mode should exclude IP and produce same fingerprint for different IPs")
	})
}

func BenchmarkFingerprintDefault(b *testing.B) {
	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
	})
	r.Use(fingerprintMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Benchmark)")
	req.Header.Set("Accept", "text/html")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkFingerprintWithValidation(b *testing.B) {
	r := router.New[*router.Context]()

	fingerprintMiddleware := middleware.FingerprintWithConfig[*router.Context](middleware.FingerprintConfig{
		StoreInContext: true,
		ValidateFunc: func(ctx handler.Context, fp string) error {
			// Simple validation simulation
			if fp == "" {
				return errors.New("invalid fingerprint")
			}
			return nil
		},
	})
	r.Use(fingerprintMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Benchmark)")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
