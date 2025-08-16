// Error handling example demonstrating built-in error responses,
// custom error handlers, and panic recovery.
//
// Run:
//
//	go run main.go
//
// Try:
//
//	curl http://localhost:8080/user/0
//	curl -X POST http://localhost:8080/login
//	curl -X POST http://localhost:8080/register
//	curl http://localhost:8080/panic
//	curl http://localhost:8080/coffee
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

func customErrorHandler(ctx *gokit.Context, err error) {
	w := ctx.ResponseWriter()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"error":"%v","custom":true}`, err)
}

func main() {
	r := gokit.NewRouter[*gokit.Context](
		gokit.WithErrorHandler(customErrorHandler),
	)

	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Error handling example")
	})

	r.Get("/user/{id}", func(ctx *gokit.Context) gokit.Response {
		if ctx.Param("id") == "0" {
			return gokit.ErrNotFound.WithMessage("User not found")
		}
		return gokit.JSON(map[string]string{
			"id":   ctx.Param("id"),
			"name": "John Doe",
		})
	})

	r.Post("/login", func(ctx *gokit.Context) gokit.Response {
		return gokit.ErrUnauthorized.
			WithMessage("Invalid credentials").
			WithDetails(map[string]any{
				"attempts_remaining": 3,
				"locked":             false,
			})
	})

	r.Post("/register", func(ctx *gokit.Context) gokit.Response {
		return gokit.ErrBadRequest.
			WithMessage("Validation failed").
			WithDetails(map[string]any{
				"errors": []string{
					"email is required",
					"password must be at least 8 characters",
				},
			})
	})

	r.Get("/panic", func(ctx *gokit.Context) gokit.Response {
		panic("Something went terribly wrong!")
	})

	r.Get("/coffee", func(ctx *gokit.Context) gokit.Response {
		return gokit.ErrTeapot.WithMessage("I'm a teapot, not a coffee machine")
	})

	log.Fatal(http.ListenAndServe(":8080", r))
}
