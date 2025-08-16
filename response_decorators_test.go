package gokit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit"
)

func TestWithHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    map[string]string
	}{
		{
			name: "single header",
			headers: map[string]string{
				"X-Custom-Header": "custom-value",
			},
			want: map[string]string{
				"X-Custom-Header": "custom-value",
				"Content-Type":    "text/plain; charset=utf-8",
			},
		},
		{
			name: "multiple headers",
			headers: map[string]string{
				"X-Request-ID":  "123456",
				"X-API-Version": "v1",
				"X-Custom":      "test",
			},
			want: map[string]string{
				"X-Request-ID":  "123456",
				"X-API-Version": "v1",
				"X-Custom":      "test",
				"Content-Type":  "text/plain; charset=utf-8",
			},
		},
		{
			name:    "empty headers map",
			headers: map[string]string{},
			want: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
		},
		{
			name:    "nil headers map",
			headers: nil,
			want: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create base response
			baseResp := gokit.String("test content")

			// Wrap with headers
			resp := gokit.WithHeaders(baseResp, tt.headers)

			// Test rendering
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			err := resp.Render(w, req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// Check headers
			for key, want := range tt.want {
				got := w.Header().Get(key)
				if got != want {
					t.Errorf("Header[%s] = %v, want %v", key, got, want)
				}
			}

			// Check body
			if got := w.Body.String(); got != "test content" {
				t.Errorf("Body = %v, want %v", got, "test content")
			}
		})
	}
}

func TestWithCookie(t *testing.T) {
	tests := []struct {
		name   string
		cookie *http.Cookie
		want   string
	}{
		{
			name: "simple cookie",
			cookie: &http.Cookie{
				Name:  "session",
				Value: "abc123",
			},
			want: "session=abc123",
		},
		{
			name: "cookie with attributes",
			cookie: &http.Cookie{
				Name:     "auth",
				Value:    "token123",
				Path:     "/",
				Domain:   "example.com",
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteStrictMode,
			},
			want: "auth=token123; Path=/; Domain=example.com; HttpOnly; Secure; SameSite=Strict",
		},
		{
			name: "cookie with expiry",
			cookie: &http.Cookie{
				Name:    "temp",
				Value:   "value",
				Expires: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			want: "temp=value; Expires=Wed, 01 Jan 2025 00:00:00 GMT",
		},
		{
			name: "cookie with max-age",
			cookie: &http.Cookie{
				Name:   "persistent",
				Value:  "data",
				MaxAge: 3600,
			},
			want: "persistent=data; Max-Age=3600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create base response
			baseResp := gokit.String("test content")

			// Wrap with cookie
			resp := gokit.WithCookie(baseResp, tt.cookie)

			// Test rendering
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			err := resp.Render(w, req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// Check cookie header
			cookies := w.Header().Get("Set-Cookie")
			if cookies == "" {
				t.Fatal("Set-Cookie header not found")
			}

			// Check if expected cookie string is contained
			if !strings.Contains(cookies, tt.want) {
				t.Errorf("Set-Cookie = %v, want to contain %v", cookies, tt.want)
			}
		})
	}
}

func TestWithCookie_Nil(t *testing.T) {
	// Test with nil cookie
	baseResp := gokit.String("test content")
	resp := gokit.WithCookie(baseResp, nil)

	// Test that response still works correctly
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	err := resp.Render(w, req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Check that no Set-Cookie header was added
	if cookie := w.Header().Get("Set-Cookie"); cookie != "" {
		t.Errorf("Set-Cookie header should not be set, got: %v", cookie)
	}

	// Check body
	if got := w.Body.String(); got != "test content" {
		t.Errorf("Body = %v, want %v", got, "test content")
	}
}

func TestWithCache(t *testing.T) {
	tests := []struct {
		name          string
		maxAge        time.Duration
		wantCacheCtrl string
		wantPragma    string
		hasExpires    bool
		expiresZero   bool
	}{
		{
			name:          "positive max-age",
			maxAge:        time.Hour,
			wantCacheCtrl: "public, max-age=3600",
			hasExpires:    true,
		},
		{
			name:          "one day cache",
			maxAge:        24 * time.Hour,
			wantCacheCtrl: "public, max-age=86400",
			hasExpires:    true,
		},
		{
			name:          "zero max-age disables cache",
			maxAge:        0,
			wantCacheCtrl: "no-cache, no-store, must-revalidate",
			wantPragma:    "no-cache",
			hasExpires:    true,
			expiresZero:   true,
		},
		{
			name:          "negative max-age disables cache",
			maxAge:        -1 * time.Second,
			wantCacheCtrl: "no-cache, no-store, must-revalidate",
			wantPragma:    "no-cache",
			hasExpires:    true,
			expiresZero:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create base response
			baseResp := gokit.String("test content")

			// Wrap with cache
			resp := gokit.WithCache(baseResp, tt.maxAge)

			// Test rendering
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			err := resp.Render(w, req)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// Check Cache-Control header
			if got := w.Header().Get("Cache-Control"); got != tt.wantCacheCtrl {
				t.Errorf("Cache-Control = %v, want %v", got, tt.wantCacheCtrl)
			}

			// Check Pragma header
			if tt.wantPragma != "" {
				if got := w.Header().Get("Pragma"); got != tt.wantPragma {
					t.Errorf("Pragma = %v, want %v", got, tt.wantPragma)
				}
			} else {
				if got := w.Header().Get("Pragma"); got != "" {
					t.Errorf("Pragma = %v, want empty", got)
				}
			}

			// Check Expires header
			if tt.hasExpires {
				expires := w.Header().Get("Expires")
				if expires == "" {
					t.Error("Expires header not found")
				} else if tt.expiresZero {
					if expires != "0" {
						t.Errorf("Expires = %v, want 0", expires)
					}
				} else {
					// Just verify the expires header is parseable
					_, err := http.ParseTime(expires)
					if err != nil {
						t.Errorf("Failed to parse Expires header: %v", err)
					}
				}
			}
		})
	}
}

func TestDecoratorsComposition(t *testing.T) {
	// Create base response
	baseResp := gokit.String("test content")

	// Apply multiple decorators
	resp := gokit.WithCache(
		gokit.WithHeaders(
			gokit.WithCookie(
				baseResp,
				&http.Cookie{
					Name:  "session",
					Value: "xyz789",
				},
			),
			map[string]string{
				"X-API-Version": "v2",
				"X-Request-ID":  "req-123",
			},
		),
		time.Hour,
	)

	// Test rendering
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	err := resp.Render(w, req)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Check all headers are present
	expectedHeaders := map[string]string{
		"X-API-Version": "v2",
		"X-Request-ID":  "req-123",
		"Cache-Control": "public, max-age=3600",
		"Content-Type":  "text/plain; charset=utf-8",
	}

	for key, want := range expectedHeaders {
		if got := w.Header().Get(key); got != want {
			t.Errorf("Header[%s] = %v, want %v", key, got, want)
		}
	}

	// Check cookie is set
	if cookie := w.Header().Get("Set-Cookie"); !strings.Contains(cookie, "session=xyz789") {
		t.Errorf("Set-Cookie = %v, want to contain session=xyz789", cookie)
	}

	// Check Expires header exists
	if expires := w.Header().Get("Expires"); expires == "" {
		t.Error("Expires header not found")
	}

	// Check body
	if got := w.Body.String(); got != "test content" {
		t.Errorf("Body = %v, want %v", got, "test content")
	}
}

func TestWithHeaders_NilResponse(t *testing.T) {
	// Test with nil response
	resp := gokit.WithHeaders(nil, map[string]string{"X-Test": "value"})
	if resp != nil {
		t.Error("WithHeaders(nil, headers) should return nil")
	}
}

func TestWithCache_NilResponse(t *testing.T) {
	// Test with nil response
	resp := gokit.WithCache(nil, time.Hour)
	if resp != nil {
		t.Error("WithCache(nil, maxAge) should return nil")
	}
}
