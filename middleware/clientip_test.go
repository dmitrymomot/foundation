package middleware_test

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
	"github.com/dmitrymomot/gokit/middleware"
	"github.com/dmitrymomot/gokit/pkg/clientip"
)

func TestClientIPDefaultConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIP[*router.Context]()
	r.Use(clientIPMiddleware)

	var capturedIP string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, ok := middleware.GetClientIP(ctx)
		assert.True(t, ok, "Client IP should be present in context by default")
		capturedIP = ip
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "192.168.1.100", capturedIP, "Should extract IP from RemoteAddr")
	assert.Empty(t, w.Header().Get("X-Client-IP"), "Default config should not set header")
}

func TestClientIPStoreInHeader(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInHeader: true,
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "10.0.0.5", w.Header().Get("X-Client-IP"), "IP should be in response header")
}

func TestClientIPCustomHeaderName(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customHeaderName := "X-Real-Client-IP"
	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		HeaderName:    customHeaderName,
		StoreInHeader: true,
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "172.16.0.10:8080"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "172.16.0.10", w.Header().Get(customHeaderName), "Custom header should be set")
	assert.Empty(t, w.Header().Get("X-Client-IP"), "Default header should not be set")
}

func TestClientIPSkipFunctionality(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		Skip: func(ctx handler.Context) bool {
			return strings.HasPrefix(ctx.Request().URL.Path, "/healthz")
		},
		StoreInContext: true,
	})
	r.Use(clientIPMiddleware)

	r.Get("/healthz", func(ctx *router.Context) handler.Response {
		ip, ok := middleware.GetClientIP(ctx)
		assert.False(t, ok, "Client IP should not be present for skipped routes")
		assert.Empty(t, ip)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Get("/api/users", func(ctx *router.Context) handler.Response {
		ip, ok := middleware.GetClientIP(ctx)
		assert.True(t, ok, "Client IP should be present for non-skipped routes")
		assert.NotEmpty(t, ip)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	t.Run("skip health endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("process api endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestClientIPValidateFunc(t *testing.T) {
	t.Parallel()

	t.Run("validation passes", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		allowedIPs := map[string]bool{
			"192.168.1.100": true,
			"10.0.0.5":      true,
		}

		clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
			ValidateFunc: func(ctx handler.Context, ip string) error {
				if !allowedIPs[ip] {
					return errors.New("IP not allowed")
				}
				return nil
			},
		})
		r.Use(clientIPMiddleware)

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("validation fails", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		blockedIPs := map[string]bool{
			"192.168.1.50": true,
			"10.0.0.1":     true,
		}

		clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
			ValidateFunc: func(ctx handler.Context, ip string) error {
				if blockedIPs[ip] {
					return errors.New("IP is blocked")
				}
				return nil
			},
		})
		r.Use(clientIPMiddleware)

		handlerExecuted := false
		r.Get("/test", func(ctx *router.Context) handler.Response {
			handlerExecuted = true
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.50:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, handlerExecuted, "Handler should not execute when validation fails")
	})
}

func TestClientIPProxyHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "Cloudflare CF-Connecting-IP",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.195",
				"X-Forwarded-For":  "192.168.1.1",
				"X-Real-IP":        "10.0.0.1",
			},
			remoteAddr: "172.16.0.1:54321",
			expectedIP: "203.0.113.195",
		},
		{
			name: "DigitalOcean DO-Connecting-IP",
			headers: map[string]string{
				"DO-Connecting-IP": "198.51.100.178",
				"X-Forwarded-For":  "192.168.1.1",
				"X-Real-IP":        "10.0.0.1",
			},
			remoteAddr: "172.16.0.1:54321",
			expectedIP: "198.51.100.178",
		},
		{
			name: "X-Forwarded-For with multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.178, 203.0.113.195, 192.168.1.1",
				"X-Real-IP":       "10.0.0.1",
			},
			remoteAddr: "172.16.0.1:54321",
			expectedIP: "198.51.100.178",
		},
		{
			name: "X-Real-IP fallback",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.195",
			},
			remoteAddr: "172.16.0.1:54321",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "RemoteAddr only",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.100:54321",
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := router.New[*router.Context]()

			clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
				StoreInContext: true,
			})
			r.Use(clientIPMiddleware)

			var capturedIP string
			r.Get("/test", func(ctx *router.Context) handler.Response {
				ip, _ := middleware.GetClientIP(ctx)
				capturedIP = ip
				return func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedIP, capturedIP, "Should extract correct IP based on header priority")
		})
	}
}

func TestClientIPWithMultipleMiddleware(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	var ipInMiddleware2, ipInHandler string

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
	})

	middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			ip, ok := middleware.GetClientIP(ctx)
			assert.True(t, ok, "Client IP should be available in subsequent middleware")
			ipInMiddleware2 = ip
			return next(ctx)
		}
	}

	r.Use(clientIPMiddleware, middleware2)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, _ := middleware.GetClientIP(ctx)
		ipInHandler = ip
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, ipInMiddleware2)
	assert.Equal(t, ipInMiddleware2, ipInHandler, "IP should be consistent across middleware")
}

func TestClientIPContextNotFound(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Handler without client IP middleware
	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, ok := middleware.GetClientIP(ctx)
		assert.False(t, ok, "Client IP should not be found when middleware not used")
		assert.Empty(t, ip, "IP should be empty when not found")
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

func TestClientIPStoreInContextFalse(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: false,
		StoreInHeader:  true, // Must do something with the IP
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, ok := middleware.GetClientIP(ctx)
		assert.False(t, ok, "Client IP should not be in context when StoreInContext is false")
		assert.Empty(t, ip)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "192.168.1.100", w.Header().Get("X-Client-IP"), "IP should still be in header")
}

func TestClientIPIPv6Support(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
		StoreInHeader:  true,
	})
	r.Use(clientIPMiddleware)

	var capturedIP string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, _ := middleware.GetClientIP(ctx)
		capturedIP = ip
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2001:db8::1", capturedIP, "Should handle IPv6 addresses")
	assert.Equal(t, "2001:db8::1", w.Header().Get("X-Client-IP"))
}

func TestClientIPIntegrationWithActualGetIP(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
	})
	r.Use(clientIPMiddleware)

	var capturedIP string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, _ := middleware.GetClientIP(ctx)
		capturedIP = ip
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.195")
	req.RemoteAddr = "172.16.0.1:54321"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Verify it matches what the clientip package would return
	expectedIP := clientip.GetIP(req)
	assert.Equal(t, expectedIP, capturedIP, "Middleware should use clientip.GetIP correctly")
}

func TestClientIPSubnetValidation(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Allow only private network IPs
	privateSubnets := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	var cidrs []*net.IPNet
	for _, subnet := range privateSubnets {
		_, cidr, _ := net.ParseCIDR(subnet)
		cidrs = append(cidrs, cidr)
	}

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		ValidateFunc: func(ctx handler.Context, ipStr string) error {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return errors.New("invalid IP")
			}

			for _, cidr := range cidrs {
				if cidr.Contains(ip) {
					return nil
				}
			}
			return errors.New("IP not in allowed subnets")
		},
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		}
	})

	t.Run("private IP allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("public IP blocked", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "8.8.8.8:54321"
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestClientIPMultipleRequests(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
	})
	r.Use(clientIPMiddleware)

	ips := make([]string, 0, 3)
	r.Get("/test", func(ctx *router.Context) handler.Response {
		ip, _ := middleware.GetClientIP(ctx)
		ips = append(ips, ip)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Make requests from different IPs
	testIPs := []string{"192.168.1.100", "10.0.0.5", "172.16.0.10"}
	for _, ip := range testIPs {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip + ":54321"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	require.Len(t, ips, 3)
	assert.Equal(t, testIPs, ips, "Should capture different IPs for different requests")
}

func BenchmarkClientIPDefault(b *testing.B) {
	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkClientIPWithProxyHeaders(b *testing.B) {
	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.195")
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	req.RemoteAddr = "172.16.0.1:54321"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkClientIPWithValidation(b *testing.B) {
	r := router.New[*router.Context]()

	clientIPMiddleware := middleware.ClientIPWithConfig[*router.Context](middleware.ClientIPConfig{
		StoreInContext: true,
		ValidateFunc: func(ctx handler.Context, ip string) error {
			// Simple validation
			if ip == "" {
				return errors.New("invalid IP")
			}
			return nil
		},
	})
	r.Use(clientIPMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
