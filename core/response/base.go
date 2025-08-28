package response

import (
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
)

// Render executes the given response handler with the provided context.
// If the response handler returns an error, it writes an HTTP 500 Internal Server Error response.
func Render(ctx handler.Context, resp handler.Response) {
	if err := resp(ctx.ResponseWriter(), ctx.Request()); err != nil {
		http.Error(ctx.ResponseWriter(), err.Error(), http.StatusInternalServerError)
	}
}

// String creates a text/plain response with 200 OK status.
func String(content string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if content != "" {
			_, err := w.Write([]byte(content))
			return err
		}
		return nil
	}
}

// StringWithStatus creates a text/plain response with custom status code.
func StringWithStatus(content string, status int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if content != "" {
			_, err := w.Write([]byte(content))
			return err
		}
		return nil
	}
}

// HTML creates a text/html response with 200 OK status.
func HTML(content string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if content != "" {
			_, err := w.Write([]byte(content))
			return err
		}
		return nil
	}
}

// HTMLWithStatus creates a text/html response with custom status code.
func HTMLWithStatus(content string, status int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if content != "" {
			_, err := w.Write([]byte(content))
			return err
		}
		return nil
	}
}

// Bytes creates a response with custom content type and 200 OK status.
func Bytes(content []byte, contentType string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(http.StatusOK)
		if len(content) > 0 {
			_, err := w.Write(content)
			return err
		}
		return nil
	}
}

// BytesWithStatus creates a response with custom content type and status code.
func BytesWithStatus(content []byte, contentType string, status int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if len(content) > 0 {
			_, err := w.Write(content)
			return err
		}
		return nil
	}
}

// NoContent creates a 204 No Content response.
func NoContent() handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// Status creates an empty response with the specified status code.
func Status(code int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		if code == 0 {
			code = http.StatusOK
		}
		w.WriteHeader(code)
		return nil
	}
}
