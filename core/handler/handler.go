package handler

import "net/http"

// Response is a function that renders HTTP responses.
// It sets headers, status code, and writes the response body.
// Rendering errors are handled by the framework's error handler.
type Response func(w http.ResponseWriter, r *http.Request) error

// HandlerFunc is a type-safe HTTP request handler with custom context support.
type HandlerFunc[C Context] func(ctx C) Response

// ErrorHandler handles errors during request processing.
type ErrorHandler[C Context] func(ctx C, err error)

// Middleware wraps handlers to add cross-cutting functionality.
type Middleware[C Context] func(next HandlerFunc[C]) HandlerFunc[C]
