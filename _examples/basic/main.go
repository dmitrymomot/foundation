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

	// Simple text response
	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Hello, gokit!")
	})

	// URL parameter
	r.Get("/hello/{name}", func(ctx *gokit.Context) gokit.Response {
		name := ctx.Param("name")
		return gokit.String(fmt.Sprintf("Hello, %s!", name))
	})

	// Read and echo body
	r.Post("/echo", func(ctx *gokit.Context) gokit.Response {
		body, _ := io.ReadAll(ctx.Request().Body)
		return gokit.String(string(body))
	})

	fmt.Println("Basic routing example - http://localhost:8080")
	fmt.Println("Try: curl http://localhost:8080/hello/world")
	log.Fatal(http.ListenAndServe(":8080", r))
}
