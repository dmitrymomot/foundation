// Middleware example demonstrating how to create and use middleware.
//
// Run:
//
//	go run main.go
//
// Try:
//
//	curl -i http://localhost:8080/
package main

import (
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

func addHeader(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
	return func(ctx *gokit.Context) gokit.Response {
		ctx.ResponseWriter().Header().Set("X-Powered-By", "gokit")
		return next(ctx)
	}
}

func main() {
	r := gokit.NewRouter[*gokit.Context](
		gokit.WithMiddleware(addHeader),
	)

	r.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Check the response headers!")
	})

	log.Fatal(http.ListenAndServe(":8080", r))
}
