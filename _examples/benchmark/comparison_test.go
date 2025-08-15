package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/go-chi/chi/v5"
	"github.com/labstack/echo/v4"
)

// Test data structures
type User struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Active    bool              `json:"active"`
	Score     float64           `json:"score"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func generateUsers(count int) []User {
	users := make([]User, count)
	for i := range users {
		users[i] = User{
			ID:        i,
			Name:      fmt.Sprintf("User %d", i),
			Email:     fmt.Sprintf("user%d@example.com", i),
			Active:    true,
			Score:     float64(i) * 1.5,
			Tags:      []string{"tag1", "tag2", "tag3"},
			Metadata:  map[string]string{"key1": "value1", "key2": "value2"},
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-01T00:00:00Z",
		}
	}
	return users
}

// ============================================================================
// GOKIT BENCHMARKS
// ============================================================================

func BenchmarkGoKit_StaticRoute(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()
	router.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("Hello World")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_ParameterizedRoute(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()
	router.Get("/users/{id}", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(map[string]string{"id": ctx.Param("id")})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users/123", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_JSONResponse(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()
	user := User{
		ID:    1,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	router.Get("/user", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(user)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/user", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_LargeJSON(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()
	users := generateUsers(1000)

	router.Get("/users", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(users)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_Middleware3(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
		return func(ctx *gokit.Context) gokit.Response {
			ctx.Request().Header.Set("X-Request-ID", "123")
			return next(ctx)
		}
	})

	router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
		return func(ctx *gokit.Context) gokit.Response {
			ctx.Request().Header.Set("X-Auth-User", "user123")
			return next(ctx)
		}
	})

	router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
		return func(ctx *gokit.Context) gokit.Response {
			ctx.Request().Header.Set("X-Trace-ID", "trace123")
			return next(ctx)
		}
	})

	router.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("OK")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_Middleware5(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	for j := 0; j < 5; j++ {
		idx := j
		router.Use(func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
			return func(ctx *gokit.Context) gokit.Response {
				ctx.Request().Header.Set(fmt.Sprintf("X-Middleware-%d", idx), "true")
				return next(ctx)
			}
		})
	}

	router.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.String("OK")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_ParseJSON(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	router.Post("/users", func(ctx *gokit.Context) gokit.Response {
		var req CreateUserRequest
		if err := json.NewDecoder(ctx.Request().Body).Decode(&req); err != nil {
			return gokit.JSONWithStatus(map[string]string{"error": err.Error()}, http.StatusBadRequest)
		}
		return gokit.JSONWithStatus(map[string]string{"id": "123", "name": req.Name}, http.StatusCreated)
	})

	body, _ := json.Marshal(CreateUserRequest{Name: "John", Email: "john@example.com"})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGoKit_ComplexRouting(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()

	// Add many routes to test routing performance
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/api/v1/resource%d/{id}", i)
		router.Get(path, func(ctx *gokit.Context) gokit.Response {
			return gokit.JSON(map[string]string{"resource": "data"})
		})
	}

	// Test route that will be matched
	router.Get("/api/v1/resource50/{id}", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(map[string]string{"id": ctx.Param("id")})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/resource50/123", nil)
		router.ServeHTTP(w, req)
	}
}

// ============================================================================
// CHI BENCHMARKS
// ============================================================================

func BenchmarkChi_StaticRoute(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_ParameterizedRoute(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users/123", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_JSONResponse(b *testing.B) {
	router := chi.NewRouter()
	user := User{
		ID:    1,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	router.Get("/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/user", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_LargeJSON(b *testing.B) {
	router := chi.NewRouter()
	users := generateUsers(1000)

	router.Get("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_Middleware3(b *testing.B) {
	router := chi.NewRouter()

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", "123")
			next.ServeHTTP(w, r)
		})
	})

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Auth-User", "user123")
			next.ServeHTTP(w, r)
		})
	})

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Trace-ID", "trace123")
			next.ServeHTTP(w, r)
		})
	})

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_Middleware5(b *testing.B) {
	router := chi.NewRouter()

	for j := 0; j < 5; j++ {
		idx := j
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Header.Set(fmt.Sprintf("X-Middleware-%d", idx), "true")
				next.ServeHTTP(w, r)
			})
		})
	}

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_ParseJSON(b *testing.B) {
	router := chi.NewRouter()

	router.Post("/users", func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123", "name": req.Name})
	})

	body, _ := json.Marshal(CreateUserRequest{Name: "John", Email: "john@example.com"})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
	}
}

func BenchmarkChi_ComplexRouting(b *testing.B) {
	router := chi.NewRouter()

	// Add many routes to test routing performance
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/api/v1/resource%d/{id}", i)
		router.Get(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"resource": "data"})
		})
	}

	// Test route that will be matched
	router.Get("/api/v1/resource50/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/resource50/123", nil)
		router.ServeHTTP(w, req)
	}
}

// ============================================================================
// ECHO BENCHMARKS
// ============================================================================

func BenchmarkEcho_StaticRoute(b *testing.B) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello World")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_ParameterizedRoute(b *testing.B) {
	e := echo.New()
	e.GET("/users/:id", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users/123", nil)
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_JSONResponse(b *testing.B) {
	e := echo.New()
	user := User{
		ID:    1,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	e.GET("/user", func(c echo.Context) error {
		return c.JSON(http.StatusOK, user)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/user", nil)
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_LargeJSON(b *testing.B) {
	e := echo.New()
	users := generateUsers(1000)

	e.GET("/users", func(c echo.Context) error {
		return c.JSON(http.StatusOK, users)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users", nil)
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_Middleware3(b *testing.B) {
	e := echo.New()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Request().Header.Set("X-Request-ID", "123")
			return next(c)
		}
	})

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Request().Header.Set("X-Auth-User", "user123")
			return next(c)
		}
	})

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Request().Header.Set("X-Trace-ID", "trace123")
			return next(c)
		}
	})

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_Middleware5(b *testing.B) {
	e := echo.New()

	for j := 0; j < 5; j++ {
		idx := j
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Request().Header.Set(fmt.Sprintf("X-Middleware-%d", idx), "true")
				return next(c)
			}
		})
	}

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_ParseJSON(b *testing.B) {
	e := echo.New()

	e.POST("/users", func(c echo.Context) error {
		var req CreateUserRequest
		if err := c.Bind(&req); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, map[string]string{"id": "123", "name": req.Name})
	})

	body, _ := json.Marshal(CreateUserRequest{Name: "John", Email: "john@example.com"})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_ComplexRouting(b *testing.B) {
	e := echo.New()

	// Add many routes to test routing performance
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/api/v1/resource%d/:id", i)
		e.GET(path, func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"resource": "data"})
		})
	}

	// Test route that will be matched
	e.GET("/api/v1/resource50/:id", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/resource50/123", nil)
		e.ServeHTTP(w, req)
	}
}

// ============================================================================
// PARALLEL BENCHMARKS - Test concurrent performance
// ============================================================================

func BenchmarkGoKit_Parallel(b *testing.B) {
	router := gokit.NewRouter[*gokit.Context]()
	router.Get("/", func(ctx *gokit.Context) gokit.Response {
		return gokit.JSON(map[string]string{"status": "ok"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			router.ServeHTTP(w, req)
		}
	})
}

func BenchmarkChi_Parallel(b *testing.B) {
	router := chi.NewRouter()
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			router.ServeHTTP(w, req)
		}
	})
}

func BenchmarkEcho_Parallel(b *testing.B) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			e.ServeHTTP(w, req)
		}
	})
}
