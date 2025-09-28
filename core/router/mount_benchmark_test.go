package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
)

// BenchmarkMountedRouterWithDefaultFactory measures performance with default context factory
func BenchmarkMountedRouterWithDefaultFactory(b *testing.B) {
	mainRouter := router.New[*router.Context]()
	apiRouter := router.New[*router.Context]()
	webRouter := router.New[*router.Context]()

	// Add routes to subrouters
	apiRouter.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte(ctx.Param("id")))
			return nil
		}
	})

	webRouter.Get("/profile", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte("profile"))
			return nil
		}
	})

	// Mount subrouters
	mainRouter.Mount("/api", apiRouter)
	mainRouter.Mount("/web", webRouter)

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mainRouter.ServeHTTP(w, req)
	}
}

// BenchmarkMountedRouterWithCustomFactory measures performance with custom context factory
func BenchmarkMountedRouterWithCustomFactory(b *testing.B) {
	// Custom factory that does minimal work
	customFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *router.Context {
		// This simulates real work like session extraction
		_ = r.Header.Get("Authorization")
		return &router.Context{}
	}

	mainRouter := router.New[*router.Context](router.WithContextFactory(customFactory))
	apiRouter := router.New[*router.Context](router.WithContextFactory(customFactory))
	webRouter := router.New[*router.Context](router.WithContextFactory(customFactory))

	// Add routes to subrouters
	apiRouter.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte(ctx.Param("id")))
			return nil
		}
	})

	webRouter.Get("/profile", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte("profile"))
			return nil
		}
	})

	// Mount subrouters
	mainRouter.Mount("/api", apiRouter)
	mainRouter.Mount("/web", webRouter)

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mainRouter.ServeHTTP(w, req)
	}
}

// BenchmarkMountedRouterMixedFactories measures performance with different factories per router
func BenchmarkMountedRouterMixedFactories(b *testing.B) {
	// Different factories for different routers
	mainFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *router.Context {
		return &router.Context{}
	}

	apiFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *router.Context {
		_ = r.Header.Get("Authorization")
		return &router.Context{}
	}

	webFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *router.Context {
		_, _ = r.Cookie("session")
		return &router.Context{}
	}

	mainRouter := router.New[*router.Context](router.WithContextFactory(mainFactory))
	apiRouter := router.New[*router.Context](router.WithContextFactory(apiFactory))
	webRouter := router.New[*router.Context](router.WithContextFactory(webFactory))

	// Add routes to subrouters
	apiRouter.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte(ctx.Param("id")))
			return nil
		}
	})

	webRouter.Get("/profile", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte("profile"))
			return nil
		}
	})

	// Mount subrouters
	mainRouter.Mount("/api", apiRouter)
	mainRouter.Mount("/web", webRouter)

	// Test API route
	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		mainRouter.ServeHTTP(w, req)
	}
}

// BenchmarkDirectRouterNoMount measures performance without mounting (baseline)
func BenchmarkDirectRouterNoMount(b *testing.B) {
	r := router.New[*router.Context]()

	r.Get("/api/users/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Write([]byte(ctx.Param("id")))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkCreateContext measures the overhead of context creation
func BenchmarkCreateContext(b *testing.B) {
	r := router.New[*router.Context]()

	// Set up a simple route
	r.Get("/test/{id}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			_ = ctx.Param("id")
			return nil
		}
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test/123", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		r.ServeHTTP(w, req)
	}
}
