package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dmitrymomot/gokit"
)

// Simple middleware that adds a header
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

	fmt.Println("Middleware example - http://localhost:8083")
	fmt.Println("Try: curl -i http://localhost:8083/")
	log.Fatal(http.ListenAndServe(":8083", r))
}
