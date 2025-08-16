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

	// Determine final status code
	status := r.statusCode
	if status == 0 {
		if r.data == nil {
			status = http.StatusNoContent // 204 for nil data with unspecified status
		} else {
			status = http.StatusOK // 200 for non-nil data with unspecified status
		}
	}

	// Write status header
	w.WriteHeader(status)

	// Handle special status codes that shouldn't have body per HTTP spec
	switch status {
	case http.StatusNoContent, http.StatusNotModified:
		return nil // No body for 204 or 304
	}

	// For all other statuses, encode the data (nil encodes as "null")
	// This is much more memory efficient for large JSON payloads
	return json.NewEncoder(w).Encode(r.data)
}
