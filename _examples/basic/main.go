// Basic gokit example demonstrating text and JSON responses.
//
// Run:
//
//	go run main.go
//
// Try:
//
//	curl http://localhost:8080/hello/world
//	curl http://localhost:8080/api
//	curl http://localhost:8080/api/user/123
//	curl -X POST -d "Hello" http://localhost:8080/echo
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

func main() {
	r := gokit.NewRouter[*gokit.Context]()

	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Hello, gokit!")
	})

	r.Get("/hello/{name}", func(ctx *gokit.Context) gokit.Response {
		name := ctx.Param("name")
		return gokit.String(fmt.Sprintf("Hello, %s!", name))
	})

	r.Get("/api", func(ctx *gokit.Context) gokit.Response {
		data := map[string]any{
			"message": "Hello from JSON API",
			"version": "1.0.0",
		}
		return gokit.JSON(data)
	})

	r.Get("/api/user/{id}", func(ctx *gokit.Context) gokit.Response {
		user := map[string]any{
			"id":   ctx.Param("id"),
			"name": fmt.Sprintf("User %s", ctx.Param("id")),
		}
		return gokit.JSON(user)
	})

	r.Post("/echo", func(ctx *gokit.Context) gokit.Response {
		body, _ := io.ReadAll(ctx.Request().Body)
		return gokit.String(string(body))
	})

	log.Fatal(http.ListenAndServe(":8080", r))
}
