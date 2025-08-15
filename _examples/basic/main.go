package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

// Simple middleware that adds a custom header
func addHeaderMiddleware(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
	return func(ctx *gokit.Context) gokit.Response {
		ctx.ResponseWriter().Header().Set("X-App-Name", "gokit-example")
		return next(ctx)
	}
}

func main() {
	// Create router with middleware - using the default Context type
	r := gokit.NewRouter[*gokit.Context](
		gokit.WithMiddleware(addHeaderMiddleware),
	)

	// Home page
	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Welcome to gokit! Try these endpoints:\n" +
			"- GET /hello/{name}\n" +
			"- POST /echo (with body)\n" +
			"- GET /redirect\n" +
			"- GET /panic\n")
	})

	// Greeting with URL parameter
	r.Get("/hello/{name}", func(ctx *gokit.Context) gokit.Response {
		name := ctx.Param("name")
		return gokit.String(fmt.Sprintf("Hello, %s!", name))
	})

	// Echo endpoint - returns what you send
	r.Post("/echo", func(ctx *gokit.Context) gokit.Response {
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return gokit.StringWithStatus("Error reading body", http.StatusBadRequest)
		}
		return gokit.String(fmt.Sprintf("You sent: %s", string(body)))
	})

	// Redirect example
	r.Get("/redirect", func(ctx *gokit.Context) gokit.Response {
		return gokit.Redirect("/")
	})

	// Panic endpoint to test error recovery
	r.Get("/panic", func(ctx *gokit.Context) gokit.Response {
		panic("This is a test panic!")
	})

	// Start server
	fmt.Println("Server starting on http://localhost:8080")
	fmt.Println("\nTest with:")
	fmt.Println("  curl http://localhost:8080/")
	fmt.Println("  curl http://localhost:8080/hello/world")
	fmt.Println("  curl -X POST http://localhost:8080/echo -d 'Hello gokit!'")
	fmt.Println("  curl -L http://localhost:8080/redirect")
	fmt.Println("  curl http://localhost:8080/panic")

	log.Fatal(http.ListenAndServe(":8080", r))
}
