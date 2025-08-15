package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

// customErrorHandler demonstrates how to handle errors including panics.
// It logs the error and returns a JSON response.
func customErrorHandler(ctx *gokit.Context, err error) {
	// Log the error for debugging
	log.Printf("ERROR handled: %v", err)

	w := ctx.ResponseWriter()

	// Set JSON content type
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Always return 500 for this example (you could check error type for different codes)
	w.WriteHeader(http.StatusInternalServerError)

	// Return JSON error response
	fmt.Fprintf(w, `{"error":"Internal Server Error","message":"%v","status":500}`, err)
}

func main() {
	// Create router with custom error handler
	r := gokit.NewRouter[*gokit.Context](
		gokit.WithErrorHandler(customErrorHandler),
	)

	// Home endpoint - works normally
	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("✅ Server is running!\n\nTest the error handler:\n" +
			"- curl http://localhost:8080/panic  (triggers panic)\n")
	})

	// Panic endpoint - demonstrates error recovery
	r.Get("/panic", func(ctx *gokit.Context) gokit.Response {
		// This panic will be caught by the error handler
		panic("💥 Something went terribly wrong! Database connection lost!")
	})

	// Start server
	fmt.Println("🚀 Custom Error Handler Example")
	fmt.Println("================================")
	fmt.Println("Server starting on http://localhost:8080")
	fmt.Println()
	fmt.Println("Test commands:")
	fmt.Println("  curl http://localhost:8080/          # Normal response")
	fmt.Println("  curl http://localhost:8080/panic     # Triggers panic (returns JSON error)")
	fmt.Println()
	fmt.Println("Watch the server logs to see error handling in action!")
	fmt.Println()

	log.Fatal(http.ListenAndServe(":8080", r))
}
