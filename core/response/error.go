package response

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
)

// Error returns a handler response that propagates the given error.
// This is useful for creating error responses in HTTP handlers where
// you want to pass through an error to be handled by middleware or
// other error handling mechanisms.
func Error(err error) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		return err
	}
}
