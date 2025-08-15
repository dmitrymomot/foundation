package gokit

import (
	"net/http"
)

// baseResponse implements Response interface with bytes content.
// It provides a flexible foundation for various content types.
type baseResponse struct {
	content     []byte
	statusCode  int
	contentType string
}

// Render implements the Response interface.
func (r baseResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Set content type if provided
	if r.contentType != "" {
		w.Header().Set("Content-Type", r.contentType)
	}

	// Set status code (default to 200 if not specified)
	status := r.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	// Write content
	if len(r.content) > 0 {
		_, err := w.Write(r.content)
		return err
	}

	return nil
}

// String creates a text/plain response with 200 OK status.
func String(content string) Response {
	return baseResponse{
		content:     []byte(content),
		statusCode:  http.StatusOK,
		contentType: "text/plain; charset=utf-8",
	}
}

// StringWithStatus creates a text/plain response with custom status code.
func StringWithStatus(content string, status int) Response {
	return baseResponse{
		content:     []byte(content),
		statusCode:  status,
		contentType: "text/plain; charset=utf-8",
	}
}

// HTML creates a text/html response with 200 OK status.
func HTML(content string) Response {
	return baseResponse{
		content:     []byte(content),
		statusCode:  http.StatusOK,
		contentType: "text/html; charset=utf-8",
	}
}

// HTMLWithStatus creates a text/html response with custom status code.
func HTMLWithStatus(content string, status int) Response {
	return baseResponse{
		content:     []byte(content),
		statusCode:  status,
		contentType: "text/html; charset=utf-8",
	}
}

// Bytes creates a response with custom content type and 200 OK status.
func Bytes(content []byte, contentType string) Response {
	return baseResponse{
		content:     content,
		statusCode:  http.StatusOK,
		contentType: contentType,
	}
}

// BytesWithStatus creates a response with custom content type and status code.
func BytesWithStatus(content []byte, contentType string, status int) Response {
	return baseResponse{
		content:     content,
		statusCode:  status,
		contentType: contentType,
	}
}

// NoContent creates a 204 No Content response.
func NoContent() Response {
	return baseResponse{
		statusCode: http.StatusNoContent,
	}
}

// Status creates an empty response with the specified status code.
func Status(code int) Response {
	return baseResponse{
		statusCode: code,
	}
}
