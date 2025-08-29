// Package static provides handlers for serving static files, directories, and Single Page Applications (SPAs).
//
// This package offers four main types of static content serving with type-safe handlers using Go generics:
//
// 1. Single file serving with File()
// 2. Directory serving with Dir()
// 3. Embedded filesystem serving with FS()
// 4. Single Page Application serving with SPA()
//
// All handlers integrate with the foundation/core/handler framework and support custom context types.
// Directory listing is disabled by default for security, but index.html files are served when present.
//
// # Basic File Serving
//
// Serve a single static file:
//
//	import (
//		"net/http"
//		"os"
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/static"
//	)
//
//	// Serve a single file (panics at startup if file doesn't exist)
//	fileHandler := static.File[handler.BaseContext]("/path/to/style.css")
//
// # Directory Serving
//
// Serve files from a directory with customizable options:
//
//	// Basic directory serving
//	assetsHandler := static.Dir[handler.BaseContext]("./assets")
//
//	// Advanced directory serving with prefix stripping and custom error handler
//	customHandler := static.Dir[handler.BaseContext](
//		"./public",
//		static.WithStripPrefix[handler.BaseContext]("/static"),
//		static.WithErrorHandler[handler.BaseContext](func(ctx handler.BaseContext, err error) handler.Response {
//			return func(w http.ResponseWriter, r *http.Request) error {
//				if os.IsNotExist(err) {
//					w.WriteHeader(http.StatusNotFound)
//					w.Write([]byte("<h1>Custom 404 Page</h1>"))
//				} else if os.IsPermission(err) {
//					w.WriteHeader(http.StatusForbidden)
//					w.Write([]byte("<h1>Access Denied</h1>"))
//				} else {
//					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
//				}
//				return nil
//			}
//		}),
//	)
//
// # Embedded Filesystem Serving
//
// Serve files from embedded filesystems using embed.FS:
//
//	import "embed"
//
//	//go:embed assets/*
//	var assetsFS embed.FS
//
//	// Serve embedded files
//	embedHandler := static.FS[handler.BaseContext](assetsFS)
//
//	// Serve from subdirectory within embedded filesystem
//	subHandler := static.FS[handler.BaseContext](
//		assetsFS,
//		static.WithSubFS("dist"),
//		static.WithFSStripPrefix("/assets"),
//	)
//
// # Single Page Application Serving
//
// Serve SPAs with client-side routing support. Files are served if they exist,
// otherwise requests fall back to index.html for client-side routing:
//
//	// Basic SPA serving
//	spaHandler := static.SPA[handler.BaseContext]("./dist")
//
//	// Advanced SPA with custom configuration
//	advancedSPA := static.SPA[handler.BaseContext](
//		"./build",
//		static.WithSPAIndex("app.html"),
//		static.WithNotFoundPage("404.html"),
//		static.WithExcludePaths("/api", "/ws", "/health"),
//		static.WithSPAStripPrefix("/app"),
//	)
//
// # Integration with HTTP Router
//
// All handlers return handler.HandlerFunc[C] which can be used with any HTTP router:
//
//	import (
//		"net/http"
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/static"
//	)
//
//	func setupRoutes() *http.ServeMux {
//		mux := http.NewServeMux()
//
//		// Single file
//		mux.Handle("GET /favicon.ico", handler.Adapt(static.File[handler.BaseContext]("./favicon.ico")))
//
//		// Static assets with default error handling
//		mux.Handle("GET /assets/", handler.Adapt(static.Dir[handler.BaseContext](
//			"./public",
//			static.WithStripPrefix[handler.BaseContext]("/assets"),
//		)))
//
//		// SPA fallback (should be last)
//		mux.Handle("/", handler.Adapt(static.SPA[handler.BaseContext]("./dist")))
//
//		return mux
//	}
//
// # Security Features
//
// - Directory listing is disabled by default for security
// - Path traversal protection through path cleaning
// - Startup validation ensures files/directories exist
// - Custom error handlers prevent information disclosure
// - Default error handler maps filesystem errors to appropriate HTTP status codes
//
// # Error Handling
//
// All functions panic at startup if:
// - Required files or directories don't exist
// - Paths are invalid or inaccessible
// - Filesystem operations fail during initialization
//
// This fail-fast approach ensures configuration errors are caught early
// rather than at runtime.
//
// The Dir() handler includes a default error handler that maps filesystem errors
// to HTTP status codes (404 for not found, 403 for permission denied, 500 for others).
// You can override this with WithErrorHandler() for custom error handling.
package static
