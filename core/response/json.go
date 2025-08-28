package response

import (
	"encoding/json"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
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

		// Apply HTTP status code logic for JSON responses
		if status == 0 {
			if v == nil {
				status = http.StatusNoContent
			} else {
				status = http.StatusOK
			}
		}

		// Write status header
		w.WriteHeader(status)

		// Respect HTTP spec: certain status codes must not include response body
		switch status {
		case http.StatusNoContent, http.StatusNotModified:
			return nil
		}

		// Stream JSON directly to response writer for memory efficiency
		return json.NewEncoder(w).Encode(v)
	}
}
