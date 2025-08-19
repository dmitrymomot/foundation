package response

import (
	"encoding/json"
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
)

// JSON creates an application/json response with 200 OK status.
// JSON encoding is performed directly to the response writer for optimal memory usage.
func JSON(v any) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return json.NewEncoder(w).Encode(v)
	}
}

// JSONWithStatus creates an application/json response with custom status code.
// JSON encoding is performed directly to the response writer for optimal memory usage.
func JSONWithStatus(v any, status int) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		// Determine final status code
		if status == 0 {
			if v == nil {
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
		return json.NewEncoder(w).Encode(v)
	}
}
