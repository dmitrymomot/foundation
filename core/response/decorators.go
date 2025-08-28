package response

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dmitrymomot/foundation/core/handler"
)

// WithHeaders wraps a response with custom HTTP headers.
// Headers are set before the wrapped response is rendered.
func WithHeaders(response func(w http.ResponseWriter, r *http.Request) error, headers map[string]string) handler.Response {
	if response == nil {
		return nil
	}
	if len(headers) == 0 {
		return response
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Apply headers before response rendering
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		return response(w, r)
	}
}

// WithCookie wraps a handler.Response with an HTTP cookie.
// The cookie is set before the wrapped response is rendered.
func WithCookie(response handler.Response, cookie *http.Cookie) handler.Response {
	if response == nil || cookie == nil {
		return response
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Apply cookie before response rendering
		http.SetCookie(w, cookie)
		return response(w, r)
	}
}

// WithCache wraps a handler.Response with cache control headers.
// If maxAge > 0, sets Cache-Control and Expires headers for caching.
// If maxAge <= 0, sets headers to prevent caching.
func WithCache(response handler.Response, maxAge time.Duration) handler.Response {
	if response == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Configure caching headers based on maxAge parameter
		if maxAge > 0 {
			// Set headers for browser and proxy caching
			seconds := int(maxAge.Seconds())
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", seconds))
			expires := time.Now().Add(maxAge)
			w.Header().Set("Expires", expires.Format(http.TimeFormat))
		} else {
			// Prevent any caching with comprehensive no-cache headers
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		return response(w, r)
	}
}
