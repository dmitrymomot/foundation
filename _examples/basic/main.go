package main

import (
	"encoding/json"
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
			"- GET /panic\n" +
			"- GET /api/users\n" +
			"- GET /api/user/{id}\n" +
			"- POST /api/data (with JSON body)\n")
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

	// JSON API examples
	// Get all users
	r.Get("/api/users", func(ctx *gokit.Context) gokit.Response {
		users := []map[string]any{
			{"id": 1, "name": "Alice", "email": "alice@example.com", "active": true},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "active": true},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com", "active": false},
		}
		return gokit.JSON(users)
	})

	// Get user by ID
	r.Get("/api/user/{id}", func(ctx *gokit.Context) gokit.Response {
		id := ctx.Param("id")
		user := map[string]any{
			"id":    id,
			"name":  fmt.Sprintf("User %s", id),
			"email": fmt.Sprintf("user%s@example.com", id),
			"metadata": map[string]any{
				"created": "2024-01-01",
				"updated": "2024-01-15",
			},
		}
		return gokit.JSON(user)
	})

	// Accept JSON data and echo it back
	r.Post("/api/data", func(ctx *gokit.Context) gokit.Response {
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			return gokit.JSONWithStatus(map[string]string{
				"error": "Failed to read request body",
			}, http.StatusBadRequest)
		}

		var data any
		if err := json.Unmarshal(body, &data); err != nil {
			return gokit.JSONWithStatus(map[string]string{
				"error": "Invalid JSON format",
			}, http.StatusBadRequest)
		}

		// Echo back the data with additional metadata
		response := map[string]any{
			"received":  data,
			"timestamp": "2024-01-15T10:30:00Z",
			"processed": true,
		}
		return gokit.JSONWithStatus(response, http.StatusCreated)
	})

	// Start server
	fmt.Println("Server starting on http://localhost:8080")
	fmt.Println("\nTest with:")
	fmt.Println("  curl http://localhost:8080/")
	fmt.Println("  curl http://localhost:8080/hello/world")
	fmt.Println("  curl -X POST http://localhost:8080/echo -d 'Hello gokit!'")
	fmt.Println("  curl -L http://localhost:8080/redirect")
	fmt.Println("  curl http://localhost:8080/panic")
	fmt.Println("\nJSON endpoints:")
	fmt.Println("  curl http://localhost:8080/api/users")
	fmt.Println("  curl http://localhost:8080/api/user/123")
	fmt.Println(`  curl -X POST http://localhost:8080/api/data -H "Content-Type: application/json" -d '{"name":"test","value":42}'`)

	log.Fatal(http.ListenAndServe(":8080", r))
}
