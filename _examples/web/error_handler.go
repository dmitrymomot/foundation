package main

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/dmitrymomot/foundation/core/response"
)

// ErrorPageData is the data structure for error pages
type ErrorPageData struct {
	Title      string
	StatusCode int
	Message    string
}

// errorHandler creates a custom error handler that renders HTML error pages using response.TemplateWithStatus
func errorHandler(tmpl *template.Template) func(ctx *Context, err error) {
	return func(ctx *Context, err error) {
		// Default error data
		data := ErrorPageData{
			Title:      "Internal Server Error",
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong",
		}

		// Try to extract response.HTTPError information
		var httpErr response.HTTPError
		if errors.As(err, &httpErr) {
			data.StatusCode = httpErr.Status
			data.Title = httpErr.Code

			// Use custom message if available
			if httpErr.Message != "" {
				data.Message = httpErr.Message
			} else {
				data.Message = http.StatusText(httpErr.Status)
			}
		}

		// Render error template with appropriate status code
		response.Render(ctx, response.TemplateWithStatus(tmpl, data, data.StatusCode))
	}
}
