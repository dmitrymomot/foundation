package response

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
)

// Redirect creates a 302 Found (temporary redirect) response.
// This is the most common redirect type for temporary redirects.
// For HTMX requests (detected via HX-Request header), it uses HX-Location
// header with 200 OK status instead of standard HTTP redirect.
func Redirect(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Check for HTMX request
		if r.Header.Get(HeaderHXRequest) == "true" {
			// Use HX-Location for HTMX clients
			w.Header().Set(HeaderHXLocation, url)
			w.WriteHeader(http.StatusOK)
			return nil
		}
		// Standard redirect for regular requests
		http.Redirect(w, r, url, http.StatusFound)
		return nil
	}
}

// RedirectPermanent creates a 301 Moved Permanently response.
// Use this when a resource has permanently moved to a new location.
// For HTMX requests, it uses HX-Location header with 200 OK status.
func RedirectPermanent(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Check for HTMX request
		if r.Header.Get(HeaderHXRequest) == "true" {
			// Use HX-Location for HTMX clients
			w.Header().Set(HeaderHXLocation, url)
			w.WriteHeader(http.StatusOK)
			return nil
		}
		// Standard redirect for regular requests
		http.Redirect(w, r, url, http.StatusMovedPermanently)
		return nil
	}
}

// RedirectSeeOther creates a 303 See Other response.
// This is useful after a POST request to redirect to a GET request.
// For HTMX requests, it uses HX-Location header with 200 OK status.
func RedirectSeeOther(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Check for HTMX request
		if r.Header.Get(HeaderHXRequest) == "true" {
			// Use HX-Location for HTMX clients
			w.Header().Set(HeaderHXLocation, url)
			w.WriteHeader(http.StatusOK)
			return nil
		}
		// Standard redirect for regular requests
		http.Redirect(w, r, url, http.StatusSeeOther)
		return nil
	}
}

// RedirectTemporary creates a 307 Temporary Redirect response.
// Unlike 302, this preserves the request method (e.g., POST remains POST).
// For HTMX requests, it uses HX-Location header with 200 OK status.
func RedirectTemporary(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Check for HTMX request
		if r.Header.Get(HeaderHXRequest) == "true" {
			// Use HX-Location for HTMX clients
			w.Header().Set(HeaderHXLocation, url)
			w.WriteHeader(http.StatusOK)
			return nil
		}
		// Standard redirect for regular requests
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return nil
	}
}

// RedirectPermanentPreserve creates a 308 Permanent Redirect response.
// Like 301 but preserves the request method (e.g., POST remains POST).
// For HTMX requests, it uses HX-Location header with 200 OK status.
func RedirectPermanentPreserve(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Check for HTMX request
		if r.Header.Get(HeaderHXRequest) == "true" {
			// Use HX-Location for HTMX clients
			w.Header().Set(HeaderHXLocation, url)
			w.WriteHeader(http.StatusOK)
			return nil
		}
		// Standard redirect for regular requests
		http.Redirect(w, r, url, http.StatusPermanentRedirect)
		return nil
	}
}

// RedirectWithStatus creates a redirect with a custom status code.
// The status should be in the 3xx range.
// For HTMX requests, it uses HX-Location header with 200 OK status.
func RedirectWithStatus(url string, status int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Check for HTMX request
		if r.Header.Get(HeaderHXRequest) == "true" {
			// Use HX-Location for HTMX clients
			w.Header().Set(HeaderHXLocation, url)
			w.WriteHeader(http.StatusOK)
			return nil
		}

		// Validate status code, default to 302 if invalid
		if status < 300 || status >= 400 {
			status = http.StatusFound
		}

		// Perform the redirect
		http.Redirect(w, r, url, status)
		return nil
	}
}
