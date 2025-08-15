package gokit_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dmitrymomot/gokit"
)

// Benchmark basic response rendering
func BenchmarkJSON(b *testing.B) {
	data := map[string]any{
		"id":      123,
		"name":    "test",
		"enabled": true,
		"tags":    []string{"tag1", "tag2", "tag3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := gokit.JSON(data)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

func BenchmarkString(b *testing.B) {
	content := "Hello, World! This is a test response."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := gokit.String(content)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

// Benchmark streaming
func BenchmarkStreamJSON(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items := make(chan any, 100)
		go func() {
			for j := 0; j < 100; j++ {
				items <- map[string]int{"id": j, "value": j * 2}
			}
			close(items)
		}()

		response := gokit.StreamJSON(items)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

// Benchmark SSE
func BenchmarkSSE(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events := make(chan any, 10)
		go func() {
			for j := 0; j < 10; j++ {
				events <- map[string]int{"count": j}
			}
			close(events)
		}()

		response := gokit.SSE(events)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

// Benchmark router
func BenchmarkRouter(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	router.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("home")
	})

	router.Get("/users/{id}", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(map[string]string{"id": ctx.Param("id")})
	})

	router.Post("/users", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSONWithStatus(map[string]string{"status": "created"}, http.StatusCreated)
	})

	b.ResetTimer()
	b.Run("static_route", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			router.ServeHTTP(w, req)
		}
	})

	b.Run("parameterized_route", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/users/123", nil)
			router.ServeHTTP(w, req)
		}
	})

	b.Run("post_route", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/users", nil)
			router.ServeHTTP(w, req)
		}
	})
}

// Benchmark middleware chain
func BenchmarkMiddleware(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	// Add multiple middlewares
	router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
		return func(ctx *gokit.Context) gokit.Response {
			ctx.Request().Header.Set("X-Middleware-1", "true")
			return next(ctx)
		}
	})

	router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
		return func(ctx *gokit.Context) gokit.Response {
			ctx.Request().Header.Set("X-Middleware-2", "true")
			return next(ctx)
		}
	})

	router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
		return func(ctx *gokit.Context) gokit.Response {
			ctx.Request().Header.Set("X-Middleware-3", "true")
			return next(ctx)
		}
	})

	router.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("response")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

// Benchmark error handling
func BenchmarkErrorHandling(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	router.Get("/panic", func(ctx *gokit.Context) gokit.Response {
		panic(gokit.ErrInternalServerError.WithMessage("test error"))
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/panic", nil)
		router.ServeHTTP(w, req)
	}
}

// Benchmark large JSON encoding
func BenchmarkLargeJSON(b *testing.B) {
	// Create a large data structure
	data := make([]map[string]any, 1000)
	for i := range data {
		data[i] = map[string]any{
			"id":         i,
			"name":       "User " + string(rune(i)),
			"email":      "user@example.com",
			"active":     true,
			"score":      float64(i) * 1.5,
			"tags":       []string{"tag1", "tag2", "tag3"},
			"metadata":   map[string]string{"key1": "value1", "key2": "value2"},
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:00Z",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := gokit.JSON(data)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

// Benchmark file response
func BenchmarkFileResponse(b *testing.B) {
	// Create a temporary file
	content := bytes.Repeat([]byte("test content\n"), 100)
	tmpfile, err := os.CreateTemp("", "benchmark*.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(content); err != nil {
		b.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := gokit.File(tmpfile.Name())
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

// Benchmark attachment response
func BenchmarkAttachment(b *testing.B) {
	data := bytes.Repeat([]byte("test data\n"), 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := gokit.Attachment(data, "test.txt", "text/plain")
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		_ = response.Render(w, req)
	}
}

// Helper to measure memory allocations
func BenchmarkMemoryAllocations(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	router.Get("/json", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(map[string]string{"status": "ok"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/json", nil)
		router.ServeHTTP(w, req)
	}
}
