package gokit

import (
	"encoding/json"
	"net/http"
)

// JSON creates an application/json response with 200 OK status.
// JSON encoding is performed directly to the response writer for optimal memory usage.
func JSON(v any) Response {
	return &jsonResponse{
		data:       v,
		statusCode: http.StatusOK,
	}
}

// JSONWithStatus creates an application/json response with custom status code.
// JSON encoding is performed directly to the response writer for optimal memory usage.
func JSONWithStatus(v any, status int) Response {
	return &jsonResponse{
		data:       v,
		statusCode: status,
	}
}

// jsonResponse implements Response interface for JSON encoding directly to the writer.
type jsonResponse struct {
	data       any
	statusCode int
}

// Render implements the Response interface by encoding JSON directly to the response writer.
func (r *jsonResponse) Render(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	status := r.statusCode
	if r.data == nil {
		status = http.StatusNoContent
	}

	switch status {
	case http.StatusNoContent:
		w.WriteHeader(status)
		return nil
	case http.StatusNotModified:
		w.WriteHeader(status)
		return nil
	default:
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
	}

	// Encode directly to response writer - no intermediate buffer
	// This is much more memory efficient for large JSON payloads
	return json.NewEncoder(w).Encode(r.data)
}
