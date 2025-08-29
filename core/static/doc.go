// Package static provides HTTP handlers for serving static files and Single Page Applications.
// It offers a simple, secure API for serving files, directories, embedded filesystems,
// and SPAs with client-side routing support.
//
// # Features
//
//   - Single file serving with automatic content type detection
//   - Directory serving with security by default (no directory listing)
//   - Support for embedded filesystems (embed.FS)
//   - SPA serving with fallback to index.html for client-side routing
//   - Path sanitization to prevent directory traversal attacks
//   - Customizable behavior through functional options
//
// # Basic Usage
//
// All functions return handler.HandlerFunc[C] which can be used with the router:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/static"
//		"github.com/dmitrymomot/foundation/core/router"
//	)
//
//	func setupRoutes(r *router.Mux[*router.Context]) {
//		// Serve a single file
//		r.Get("/favicon.ico", static.File[*router.Context]("./static/favicon.ico"))
//
//		// Serve files from a directory
//		r.Get("/assets/*", static.Dir[*router.Context]("./public/assets"))
//
//		// Serve SPA with client-side routing
//		r.Get("/*", static.SPA[*router.Context]("./dist"))
//	}
//
// # Serving Single Files
//
// Use File to serve individual static files:
//
//	// Serve favicon
//	r.Get("/favicon.ico", static.File[*router.Context]("./static/favicon.ico"))
//
//	// Serve robots.txt
//	r.Get("/robots.txt", static.File[*router.Context]("./static/robots.txt"))
//
//	// Serve sitemap
//	r.Get("/sitemap.xml", static.File[*router.Context]("./static/sitemap.xml"))
//
// # Serving Directories
//
// Use Dir to serve files from a directory without directory listing:
//
//	// Basic directory serving
//	r.Get("/static/*", static.Dir[*router.Context]("./public"))
//
//	// With prefix stripping
//	r.Get("/assets/*", static.Dir[*router.Context](
//		"./public/assets",
//		static.WithStripPrefix("/assets"),
//	))
//
//	// With custom 404 handler
//	r.Get("/files/*", static.Dir[*router.Context](
//		"./files",
//		static.WithNotFound(func(w http.ResponseWriter, r *http.Request) error {
//			w.WriteHeader(http.StatusNotFound)
//			_, err := w.Write([]byte("File not found"))
//			return err
//		}),
//	))
//
// # Serving Embedded Filesystems
//
// Use FS to serve files from an embedded filesystem:
//
//	import "embed"
//
//	//go:embed dist/*
//	var distFS embed.FS
//
//	// Serve from embedded filesystem
//	r.Get("/static/*", static.FS[*router.Context](distFS))
//
//	// Serve from subdirectory in embedded FS
//	r.Get("/assets/*", static.FS[*router.Context](
//		distFS,
//		static.WithSubFS("dist/assets"),
//		static.WithFSStripPrefix("/assets"),
//	))
//
// # Serving Single Page Applications
//
// Use SPA for applications with client-side routing:
//
//	// Basic SPA serving
//	r.Get("/*", static.SPA[*router.Context]("./dist"))
//
//	// With custom index and 404 pages
//	r.Get("/*", static.SPA[*router.Context](
//		"./dist",
//		static.WithSPAIndex("app.html"),
//		static.WithNotFoundPage("404.html"),
//	))
//
//	// Exclude API routes from SPA handling
//	r.Get("/*", static.SPA[*router.Context](
//		"./dist",
//		static.WithExcludePaths("/api", "/ws", "/health"),
//	))
//
//	// Mount SPA under a prefix
//	r.Get("/app/*", static.SPA[*router.Context](
//		"./dist",
//		static.WithSPAStripPrefix("/app"),
//	))
//
// # Security Considerations
//
// The package implements several security measures:
//
//   - Path cleaning to prevent directory traversal attacks
//   - Directory listing disabled by default
//   - Automatic file existence checks
//   - Safe handling of symbolic links
//
// # Performance
//
// The package uses standard library functions for optimal performance:
//
//   - http.ServeFile for single files (supports range requests and caching)
//   - http.FileServer for directory serving (efficient static file serving)
//   - Minimal overhead for SPA routing logic
//
// # Complete Example
//
// Here's a complete example of a web application with static assets and SPA:
//
//	package main
//
//	import (
//		"embed"
//		"net/http"
//
//		"github.com/dmitrymomot/foundation/core/router"
//		"github.com/dmitrymomot/foundation/core/static"
//	)
//
//	//go:embed dist/*
//	var distFS embed.FS
//
//	func main() {
//		r := router.New[*router.Context]()
//
//		// API routes (excluded from SPA handling)
//		r.Get("/api/health", healthHandler)
//		r.Post("/api/users", createUserHandler)
//
//		// Static assets with caching
//		r.Get("/assets/*", static.FS[*router.Context](
//			distFS,
//			static.WithSubFS("dist/assets"),
//			static.WithFSStripPrefix("/assets"),
//		))
//
//		// Favicon
//		r.Get("/favicon.ico", static.File[*router.Context]("./dist/favicon.ico"))
//
//		// SPA for all other routes
//		r.Get("/*", static.SPA[*router.Context](
//			"./dist",
//			static.WithExcludePaths("/api", "/assets"),
//			static.WithNotFoundPage("404.html"),
//		))
//
//		http.ListenAndServe(":8080", r)
//	}
package static
