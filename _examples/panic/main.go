package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

func main() {
	r := gokit.NewRouter[*gokit.Context]()

	// Normal endpoint
	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Server is running!")
	})

	// Panic endpoint - automatically recovered
	r.Get("/panic", func(ctx *gokit.Context) gokit.Response {
		panic("Something went wrong!")
	})

	fmt.Println("Panic recovery example - http://localhost:8082")
	fmt.Println("Try: curl http://localhost:8082/panic")
	log.Fatal(http.ListenAndServe(":8082", r))
}
