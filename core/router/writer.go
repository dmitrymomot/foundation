package router

import (
	"net/http"
)

// responseWriter is a minimal wrapper around http.ResponseWriter
// that tracks whether a response has been written.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

// newResponseWriter creates a new response writer wrapper
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
	}
}

func (w *responseWriter) WriteHeader(status int) {
	if !w.written {
		w.status = status
		w.written = true
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Written returns true if WriteHeader has been called
func (w *responseWriter) Written() bool {
	return w.written
}

// Status returns the HTTP status code
func (w *responseWriter) Status() int {
	return w.status
}

// Flush implements http.Flusher interface if the underlying ResponseWriter supports it.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
