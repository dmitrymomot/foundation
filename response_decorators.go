package gokit

import (
	"fmt"
	"net/http"
	"time"
)

// headersResponse wraps a Response and adds custom headers.
type headersResponse struct {
	wrapped Response
	headers map[string]string
}

// Render implements the Response interface.
func (r *headersResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Set custom headers before rendering the wrapped response
	for k, v := range r.headers {
		w.Header().Set(k, v)
	}
	return r.wrapped.Render(w, req)
}

// WithHeaders wraps a Response with custom HTTP headers.
// Headers are set before the wrapped response is rendered.
func WithHeaders(response Response, headers map[string]string) Response {
	if response == nil {
		return nil
	}
	if len(headers) == 0 {
		return response
	}
	return &headersResponse{
		wrapped: response,
		headers: headers,
	}
}

// cookieResponse wraps a Response and adds a cookie.
type cookieResponse struct {
	wrapped Response
	cookie  *http.Cookie
}

// Render implements the Response interface.
func (r *cookieResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Set cookie before rendering the wrapped response
	http.SetCookie(w, r.cookie)
	return r.wrapped.Render(w, req)
}

// WithCookie wraps a Response with an HTTP cookie.
// The cookie is set before the wrapped response is rendered.
func WithCookie(response Response, cookie *http.Cookie) Response {
	if response == nil || cookie == nil {
		return response
	}
	return &cookieResponse{
		wrapped: response,
		cookie:  cookie,
	}
}

// cacheResponse wraps a Response and adds cache control headers.
type cacheResponse struct {
	wrapped Response
	maxAge  time.Duration
}

// Render implements the Response interface.
func (r *cacheResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Set cache control headers before rendering the wrapped response
	if r.maxAge > 0 {
		// Enable caching
		seconds := int(r.maxAge.Seconds())
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", seconds))
		expires := time.Now().Add(r.maxAge)
		w.Header().Set("Expires", expires.Format(http.TimeFormat))
	} else {
		// Disable caching
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
	return r.wrapped.Render(w, req)
}

// WithCache wraps a Response with cache control headers.
// If maxAge > 0, sets Cache-Control and Expires headers for caching.
// If maxAge <= 0, sets headers to prevent caching.
func WithCache(response Response, maxAge time.Duration) Response {
	if response == nil {
		return nil
	}
	return &cacheResponse{
		wrapped: response,
		maxAge:  maxAge,
	}
}
