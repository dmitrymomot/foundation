package health

import (
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
)

// Liveness indicates if the service process is running.
// Always returns "ALIVE" with 200 OK. No dependency checks.
//
// Example:
//
//	router.GET("/health/live", health.Liveness[*myapp.Context])
func Liveness[C handler.Context](C) handler.Response {
	return response.String("ALIVE")
}

// NoContent returns HTTP 204 without body. Ideal for high-frequency checks.
//
// Example:
//
//	router.GET("/ping", health.NoContent[*myapp.Context])
func NoContent[C handler.Context](C) handler.Response {
	return response.NoContent()
}
