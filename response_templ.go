package gokit

import (
	"context"
	"io"
	"net/http"
)

// templResponse wraps a templComponent for rendering HTML responses.
type templResponse struct {
	component  templComponent
	statusCode int
}

type templComponent interface {
	Render(ctx context.Context, w io.Writer) error
}

// Render implements the Response interface by rendering the templ component.
func (r *templResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Set HTML content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Determine status code
	status := r.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	// Render the templ component with request context
	// This allows components to access request-scoped values
	return r.component.Render(req.Context(), w)
}

// Templ creates an HTML response using a templ component with 200 OK status.
// The component will be rendered with the request's context, allowing access
// to request-scoped values like authentication info or request IDs.
func Templ(component templComponent) Response {
	if component == nil {
		return nil
	}
	return &templResponse{
		component:  component,
		statusCode: http.StatusOK,
	}
}

// TemplWithStatus creates an HTML response using a templ component with custom status code.
// This is useful for error pages or other non-200 responses while still using templ components.
func TemplWithStatus(component templComponent, status int) Response {
	if component == nil {
		return nil
	}
	return &templResponse{
		component:  component,
		statusCode: status,
	}
}
