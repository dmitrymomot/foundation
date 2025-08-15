package gokit

import (
	"net/http"
)

// redirectResponse implements Response for HTTP redirects.
type redirectResponse struct {
	url    string
	status int
}

// Render implements the Response interface for redirects.
func (r redirectResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Validate status code, default to 302 if invalid
	status := r.status
	if status < 300 || status >= 400 {
		status = http.StatusFound
	}

	// Perform the redirect
	http.Redirect(w, req, r.url, status)
	return nil
}

// Redirect creates a 302 Found (temporary redirect) response.
// This is the most common redirect type for temporary redirects.
func Redirect(url string) Response {
	return redirectResponse{
		url:    url,
		status: http.StatusFound,
	}
}

// RedirectPermanent creates a 301 Moved Permanently response.
// Use this when a resource has permanently moved to a new location.
func RedirectPermanent(url string) Response {
	return redirectResponse{
		url:    url,
		status: http.StatusMovedPermanently,
	}
}

// RedirectSeeOther creates a 303 See Other response.
// This is useful after a POST request to redirect to a GET request.
func RedirectSeeOther(url string) Response {
	return redirectResponse{
		url:    url,
		status: http.StatusSeeOther,
	}
}

// RedirectTemporary creates a 307 Temporary Redirect response.
// Unlike 302, this preserves the request method (e.g., POST remains POST).
func RedirectTemporary(url string) Response {
	return redirectResponse{
		url:    url,
		status: http.StatusTemporaryRedirect,
	}
}

// RedirectPermanentPreserve creates a 308 Permanent Redirect response.
// Like 301 but preserves the request method (e.g., POST remains POST).
func RedirectPermanentPreserve(url string) Response {
	return redirectResponse{
		url:    url,
		status: http.StatusPermanentRedirect,
	}
}

// RedirectWithStatus creates a redirect with a custom status code.
// The status should be in the 3xx range.
func RedirectWithStatus(url string, status int) Response {
	return redirectResponse{
		url:    url,
		status: status,
	}
}
