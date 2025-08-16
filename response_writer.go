package gokit

import "net/http"

// responseWriter wraps http.ResponseWriter to track response state.
type responseWriter struct {
	http.ResponseWriter
	written bool
	status  int
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

// Written returns true if the response has been written.
func (w *responseWriter) Written() bool {
	return w.written
}

// Status returns the HTTP status code of the response.
func (w *responseWriter) Status() int {
	return w.status
}

// Flush implements http.Flusher interface if the underlying ResponseWriter supports it.
func (w *responseWriter) Flush() bool {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
		return true
	}
	return false
}
