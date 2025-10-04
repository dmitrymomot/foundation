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

// errorHandler creates a custom error handler that renders HTML error pages
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

		// Set content type and status
		ctx.ResponseWriter().Header().Set("Content-Type", "text/html; charset=utf-8")
		ctx.ResponseWriter().WriteHeader(data.StatusCode)

		// Render error template
		if err := tmpl.Execute(ctx.ResponseWriter(), data); err != nil {
			// Fallback to plain text if template fails
			http.Error(ctx.ResponseWriter(), data.Message, data.StatusCode)
		}
	}
}
