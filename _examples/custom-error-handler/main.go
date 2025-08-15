package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

// Custom error handler that returns JSON
func errorHandler(ctx *gokit.Context, err error) {
	w := ctx.ResponseWriter()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"error":"%v"}`, err)
}

func main() {
	r := gokit.NewRouter[*gokit.Context](
		gokit.WithErrorHandler(errorHandler),
	)

	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Server is running!")
	})

	r.Get("/panic", func(ctx *gokit.Context) gokit.Response {
		panic("Something went wrong!")
	})

	fmt.Println("Custom error handler example - http://localhost:8084")
	fmt.Println("Try: curl http://localhost:8084/panic")
	log.Fatal(http.ListenAndServe(":8084", r))
}
