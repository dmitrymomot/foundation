package response

import (
	"net/http"

	"github.com/dmitrymomot/gokit/handler"
)

// Redirect creates a 302 Found (temporary redirect) response.
// This is the most common redirect type for temporary redirects.
func Redirect(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, url, http.StatusFound)
		return nil
	}
}

// RedirectPermanent creates a 301 Moved Permanently response.
// Use this when a resource has permanently moved to a new location.
func RedirectPermanent(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, url, http.StatusMovedPermanently)
		return nil
	}
}

// RedirectSeeOther creates a 303 See Other response.
// This is useful after a POST request to redirect to a GET request.
func RedirectSeeOther(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, url, http.StatusSeeOther)
		return nil
	}
}

// RedirectTemporary creates a 307 Temporary Redirect response.
// Unlike 302, this preserves the request method (e.g., POST remains POST).
func RedirectTemporary(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return nil
	}
}

// RedirectPermanentPreserve creates a 308 Permanent Redirect response.
// Like 301 but preserves the request method (e.g., POST remains POST).
func RedirectPermanentPreserve(url string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, url, http.StatusPermanentRedirect)
		return nil
	}
}

// RedirectWithStatus creates a redirect with a custom status code.
// The status should be in the 3xx range.
func RedirectWithStatus(url string, status int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Validate status code, default to 302 if invalid
		if status < 300 || status >= 400 {
			status = http.StatusFound
		}

		// Perform the redirect
		http.Redirect(w, r, url, status)
		return nil
	}
}
