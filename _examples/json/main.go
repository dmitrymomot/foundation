package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

func main() {
	r := gokit.NewRouter[*gokit.Context]()

	// Return JSON object
	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		data := map[string]any{
			"message": "Hello from JSON API",
			"version": "1.0.0",
		}
		return gokit.JSON(data)
	})

	// JSON with URL parameter
	r.Get("/user/{id}", func(ctx *gokit.Context) gokit.Response {
		user := map[string]any{
			"id":   ctx.Param("id"),
			"name": fmt.Sprintf("User %s", ctx.Param("id")),
		}
		return gokit.JSON(user)
	})

	fmt.Println("JSON example - http://localhost:8081")
	fmt.Println("Try: curl http://localhost:8081/user/123")
	log.Fatal(http.ListenAndServe(":8081", r))
}
