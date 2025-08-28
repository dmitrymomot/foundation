package response

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
)

// templComponent interface for templ components that can be rendered.
type templComponent interface {
	Render(ctx context.Context, w io.Writer) error
}

// Templ creates an HTML response using a templ component with 200 OK status.
// The component will be rendered with the request's context, allowing access
// to request-scoped values like authentication info or request IDs.
func Templ(component templComponent) handler.Response {
	if component == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		w.WriteHeader(http.StatusOK)

		// Render the templ component with request context
		// This allows components to access request-scoped values
		if err := component.Render(r.Context(), w); err != nil {
			return fmt.Errorf("templ component render error: %w", err)
		}
		return nil
	}
}

// TemplWithStatus creates an HTML response using a templ component with custom status code.
// This is useful for error pages or other non-200 responses while still using templ components.
func TemplWithStatus(component templComponent, status int) handler.Response {
	if component == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Determine status code
		resolvedStatus := status
		if resolvedStatus == 0 {
			resolvedStatus = http.StatusOK
		}
		w.WriteHeader(resolvedStatus)

		// Render the templ component with request context
		// This allows components to access request-scoped values
		if err := component.Render(r.Context(), w); err != nil {
			return fmt.Errorf("templ component render error: %w", err)
		}
		return nil
	}
}
