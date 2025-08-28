package router_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
)

// Simple handler for benchmarks
func benchHandler(ctx *router.Context) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return nil
	}
}

// Parameter handler for benchmarks
func benchParamHandler(ctx *router.Context) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := ctx.Param("id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ID: " + id))
		return nil
	}
}

func BenchmarkRouterStaticRoutes(b *testing.B) {
	r := router.New[*router.Context]()

	// Register static routes
	staticRoutes := []string{
		"/",
		"/health",
		"/api",
		"/api/users",
		"/api/posts",
		"/api/comments",
		"/admin",
		"/admin/dashboard",
		"/admin/users",
		"/admin/settings",
	}

	for _, route := range staticRoutes {
		r.Get(route, benchHandler)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterParameterRoutes(b *testing.B) {
	r := router.New[*router.Context]()

	// Register parameter routes
	r.Get("/users/{id}", benchParamHandler)
	r.Get("/users/{id}/posts", benchHandler)
	r.Get("/users/{id}/posts/{postId}", benchParamHandler)
	r.Get("/api/{version}/users/{userId}", benchParamHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterRegexpRoutes(b *testing.B) {
	r := router.New[*router.Context]()

	// Register regexp routes
	r.Get("/users/{id:[0-9]+}", benchParamHandler)
	r.Get("/posts/{slug:[a-z-]+}", benchParamHandler)
	r.Get("/files/{filename:[a-zA-Z0-9._-]+}", benchParamHandler)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterWildcardRoutes(b *testing.B) {
	r := router.New[*router.Context]()

	// Register wildcard routes
	r.Get("/static/*", benchParamHandler)
	r.Get("/files/{dir}/*", benchParamHandler)

	req := httptest.NewRequest(http.MethodGet, "/static/css/main.css", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterMiddleware(b *testing.B) {
	r := router.New[*router.Context]()

	// Add middleware
	middleware1 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			return next(ctx)
		}
	}

	middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			return next(ctx)
		}
	}

	middleware3 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			return next(ctx)
		}
	}

	r.Use(middleware1, middleware2, middleware3)
	r.Get("/test", benchHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterParameterExtraction(b *testing.B) {
	r := router.New[*router.Context]()

	paramHandler := func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Extract multiple parameters
			_ = ctx.Param("version")
			_ = ctx.Param("userId")
			_ = ctx.Param("resourceId")
			_ = ctx.Param("action")

			w.WriteHeader(http.StatusOK)
			return nil
		}
	}

	r.Get("/api/{version}/users/{userId}/resources/{resourceId}/actions/{action}", paramHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/resources/456/actions/edit", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterNotFound(b *testing.B) {
	r := router.New[*router.Context]()

	// Register some routes
	r.Get("/users", benchHandler)
	r.Get("/posts", benchHandler)
	r.Get("/comments", benchHandler)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterMethodNotAllowed(b *testing.B) {
	r := router.New[*router.Context]()

	// Register only GET handler
	r.Get("/test", benchHandler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterLargeRouteTable(b *testing.B) {
	r := router.New[*router.Context]()

	// Create large route table
	for i := 0; i < 1000; i++ {
		path := "/api/v1/endpoint" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
		r.Get(path, benchHandler)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/endpointaa", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterDeepNesting(b *testing.B) {
	r := router.New[*router.Context]()

	// Create deeply nested routes
	r.Get("/level1/level2/level3/level4/level5/level6/level7/level8/level9/level10", benchHandler)

	req := httptest.NewRequest(http.MethodGet, "/level1/level2/level3/level4/level5/level6/level7/level8/level9/level10", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterMountedSubrouter(b *testing.B) {
	mainRouter := router.New[*router.Context]()
	subRouter := router.New[*router.Context]()

	// Add routes to subrouter
	subRouter.Get("/users", benchHandler)
	subRouter.Get("/posts", benchHandler)
	subRouter.Get("/users/{id}", benchParamHandler)

	// Mount subrouter
	mainRouter.Mount("/api", subRouter)

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mainRouter.ServeHTTP(w, req)
	}
}

func BenchmarkRouterRoutePriority(b *testing.B) {
	r := router.New[*router.Context]()

	// Register routes in priority order (static > regexp > param > wildcard)
	r.Get("/users/admin", benchHandler)       // Static (highest priority)
	r.Get("/users/{id:[0-9]+}", benchHandler) // Regexp
	r.Get("/users/{id}", benchHandler)        // Parameter
	r.Get("/users/*", benchHandler)           // Wildcard (lowest priority)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

// Benchmark different HTTP methods
func BenchmarkRouterHTTPMethods(b *testing.B) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		b.Run(method, func(b *testing.B) {
			r := router.New[*router.Context]()

			switch method {
			case http.MethodGet:
				r.Get("/test", benchHandler)
			case http.MethodPost:
				r.Post("/test", benchHandler)
			case http.MethodPut:
				r.Put("/test", benchHandler)
			case http.MethodDelete:
				r.Delete("/test", benchHandler)
			case http.MethodPatch:
				r.Patch("/test", benchHandler)
			case http.MethodHead:
				r.Head("/test", benchHandler)
			case http.MethodOptions:
				r.Options("/test", benchHandler)
			}

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w.Body.Reset()
				r.ServeHTTP(w, req)
			}
		})
	}
}

// Benchmark context parameter access
func BenchmarkRouterParamAccess(b *testing.B) {
	r := router.New[*router.Context]()

	paramAccessHandler := func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Access parameters multiple times
			for i := 0; i < 10; i++ {
				_ = ctx.Param("id")
				_ = ctx.Param("name")
				_ = ctx.Param("category")
			}
			w.WriteHeader(http.StatusOK)
			return nil
		}
	}

	r.Get("/items/{id}/categories/{category}/names/{name}", paramAccessHandler)

	req := httptest.NewRequest(http.MethodGet, "/items/123/categories/electronics/names/laptop", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

// Benchmark complex regexp patterns
func BenchmarkRouterComplexRegexp(b *testing.B) {
	r := router.New[*router.Context]()

	// Complex regexp patterns
	r.Get("/users/{id:[0-9]{1,10}}", benchParamHandler)
	r.Get("/posts/{slug:[a-z0-9-]{3,50}}", benchParamHandler)
	r.Get("/files/{filename:[a-zA-Z0-9._-]{1,255}}", benchParamHandler)

	req := httptest.NewRequest(http.MethodGet, "/posts/hello-world-123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

// Benchmark error handling
func BenchmarkRouterErrorHandling(b *testing.B) {
	r := router.New[*router.Context]()

	errorHandler := func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("benchmark error")
		}
	}

	r.Get("/error", errorHandler)

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

// Benchmark router creation
func BenchmarkRouterCreation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := router.New[*router.Context]()
		_ = r
	}
}

// Benchmark route registration
func BenchmarkRouterRouteRegistration(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := router.New[*router.Context]()

		r.Get("/users", benchHandler)
		r.Post("/users", benchHandler)
		r.Get("/users/{id}", benchParamHandler)
		r.Put("/users/{id}", benchHandler)
		r.Delete("/users/{id}", benchHandler)
		r.Get("/users/{id}/posts", benchHandler)
		r.Post("/users/{id}/posts", benchHandler)
		r.Get("/users/{id}/posts/{postId}", benchParamHandler)
		r.Put("/users/{id}/posts/{postId}", benchHandler)
		r.Delete("/users/{id}/posts/{postId}", benchHandler)
	}
}

// Benchmark memory allocation patterns
func BenchmarkRouterMemoryPattern(b *testing.B) {
	r := router.New[*router.Context]()
	r.Get("/users/{id}", benchParamHandler)

	requests := make([]*http.Request, 100)
	recorders := make([]*httptest.ResponseRecorder, 100)

	for i := 0; i < 100; i++ {
		requests[i] = httptest.NewRequest(http.MethodGet, "/users/123", nil)
		recorders[i] = httptest.NewRecorder()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := i % 100
		recorders[idx].Body.Reset()
		r.ServeHTTP(recorders[idx], requests[idx])
	}
}
