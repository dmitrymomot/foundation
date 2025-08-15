package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

func main() {
	r := gokit.NewRouter[*gokit.Context]()

	// Error implements Response interface
	r.Get("/user/{id}", func(ctx *gokit.Context) gokit.Response {
		if ctx.Param("id") == "0" {
			return gokit.ErrNotFound.WithMessage("User not found")
		}
		return gokit.JSON(map[string]string{"id": ctx.Param("id"), "name": "John"})
	})

	// Error with details
	r.Post("/login", func(ctx *gokit.Context) gokit.Response {
		return gokit.ErrUnauthorized.
			WithMessage("Invalid credentials").
			WithDetails(map[string]any{"attempts": 3})
	})

	// Teapot easter egg
	r.Get("/coffee", func(ctx *gokit.Context) gokit.Response {
		return gokit.ErrTeapot
	})

	fmt.Println("Error response example - http://localhost:8085")
	fmt.Println("Try: curl http://localhost:8085/user/0")
	log.Fatal(http.ListenAndServe(":8085", r))
}
