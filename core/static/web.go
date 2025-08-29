package static

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
)

// spaConfig holds configuration options for Single Page Application serving.
type spaConfig struct {
	root         string
	indexFile    string
	excludePaths []string
}

// SPAOption is a functional option type for configuring SPA serving behavior.
type SPAOption func(*spaConfig)

// WithSPAIndex sets the index file for the SPA (default: "index.html").
func WithSPAIndex(indexFile string) SPAOption {
	return func(c *spaConfig) {
		c.indexFile = indexFile
	}
}

// WithExcludePaths sets paths that should be excluded from SPA handling.
// These paths will return 404 instead of falling back to index.html.
// Default excluded paths are "/api" and "/ws".
func WithSPAExcludePaths(paths ...string) SPAOption {
	return func(c *spaConfig) {
		c.excludePaths = paths
	}
}

// SPA creates a handler for serving Single Page Applications with client-side routing.
//
// This handler is designed for modern JavaScript frameworks (React, Vue, Angular) that
// handle routing on the client side. It serves static assets normally and falls back
// to the index file for all other routes, enabling client-side routing.
//
// Mounting and Path Resolution:
// When mounting this handler at a router path, the router strips its mount prefix
// before passing requests to the handler. The handler then resolves files relative
// to the root directory:
//
//	mux.Handle("/", static.SPA[MyContext]("./dist"))
//	// GET /app.js → ./dist/app.js
//	// GET /dashboard → ./dist/index.html (fallback)
//
//	mux.Handle("/app/", static.SPA[MyContext]("./build"))
//	// GET /app/main.js → ./build/main.js (router strips /app)
//	// GET /app/users → ./build/index.html (fallback)
//
// Behavior:
// - Serves static files if they exist
// - Falls back to index.html for all non-file routes (client-side routing)
// - Returns 404 for excluded paths (API routes, WebSocket endpoints)
// - Does NOT serve subdirectory index files
//
// Parameters:
//   - root: The root directory containing the SPA build files
//   - opts: Optional configuration functions
//
// Panics at startup if the root directory or index file doesn't exist.
func SPA[C handler.Context](root string, opts ...SPAOption) handler.HandlerFunc[C] {
	config := &spaConfig{
		root:         filepath.Clean(root),
		indexFile:    "index.html",
		excludePaths: []string{"/api", "/ws"},
	}

	for _, opt := range opts {
		opt(config)
	}

	// Validate root directory exists
	if err := validateStartup(config.root, true); err != nil {
		panic("static.SPA: " + err.Error())
	}

	// Validate index file exists
	indexPath := filepath.Join(config.root, config.indexFile)
	if err := validateStartup(indexPath, false); err != nil {
		panic("static.SPA: " + err.Error())
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Clean the URL path
			urlPath := path.Clean(r.URL.Path)

			// Check if path should be excluded from SPA handling
			for _, exclude := range config.excludePaths {
				// Use exact path segment matching
				if urlPath == exclude || strings.HasPrefix(urlPath, exclude+"/") {
					return response.ErrNotFound
				}
			}

			// Construct file path
			filePath := filepath.Join(config.root, urlPath)

			// Validate that the path is within the root directory
			if err := validatePathSecurity(config.root, filePath); err != nil {
				// Serve index for invalid paths (client-side routing)
				http.ServeFile(w, r, indexPath)
				return nil
			}

			// Check if file exists (not directory)
			info, err := os.Stat(filePath)
			if err == nil && !info.IsDir() {
				// File exists, serve it
				http.ServeFile(w, r, filePath)
				return nil
			}

			// All other paths fall back to index.html for client-side routing
			http.ServeFile(w, r, indexPath)
			return nil
		}
	}
}

// websiteConfig holds configuration options for static website serving.
type websiteConfig struct {
	root         string
	indexFile    string
	notFoundFile string
	excludePaths []string
}

// WebsiteOption is a functional option type for configuring static website serving.
type WebsiteOption func(*websiteConfig)

// WithIndexFile sets the index file for directories (default: "index.html").
func WithIndexFile(indexFile string) WebsiteOption {
	return func(c *websiteConfig) {
		c.indexFile = indexFile
	}
}

// WithNotFoundFile sets a custom 404 page for the website.
func WithNotFoundFile(notFoundFile string) WebsiteOption {
	return func(c *websiteConfig) {
		c.notFoundFile = notFoundFile
	}
}

// WithWebsiteExcludePaths sets paths that should be excluded from website handling.
func WithWebsiteExcludePaths(paths ...string) WebsiteOption {
	return func(c *websiteConfig) {
		c.excludePaths = paths
	}
}

// Website creates a handler for serving static websites with proper SEO-friendly URLs.
//
// This handler is designed for static site generators (Astro, Hugo, Jekyll) that
// produce a directory structure of HTML files. It enforces canonical URLs to
// prevent duplicate content issues for SEO.
//
// Mounting and Path Resolution:
// When mounting this handler at a router path, the router strips its mount prefix
// before passing requests to the handler. The handler then resolves files relative
// to the root directory:
//
//	mux.Handle("/", static.Website[MyContext]("./public"))
//	// GET /about.html → ./public/about.html
//
//	mux.Handle("/blog/", static.Website[MyContext]("./dist"))
//	// GET /blog/post.html → ./dist/post.html (router strips /blog)
//
// To serve files from a subdirectory matching the mount path:
//
//	mux.Handle("/blog/", static.Website[MyContext]("./dist/blog"))
//	// GET /blog/post.html → ./dist/blog/post.html
//
// URL handling for SEO:
// - /path/index.html → 301 redirect to /path/
// - /path → 301 redirect to /path/
// - /path/ → serves ./path/index.html
// - /about.html → serves file directly
//
// Behavior:
// - Redirects index.html requests to directory URLs
// - Adds trailing slashes to directory requests
// - Serves index files for directory URLs
// - Returns 404 for missing files (with optional custom 404 page)
//
// Parameters:
//   - root: The root directory containing the website files
//   - opts: Optional configuration functions
//
// Panics at startup if the root directory doesn't exist.
func Website[C handler.Context](root string, opts ...WebsiteOption) handler.HandlerFunc[C] {
	config := &websiteConfig{
		root:         filepath.Clean(root),
		indexFile:    "index.html",
		notFoundFile: "",
		excludePaths: []string{},
	}

	for _, opt := range opts {
		opt(config)
	}

	// Validate root directory exists
	if err := validateStartup(config.root, true); err != nil {
		panic("static.Website: " + err.Error())
	}

	// Validate 404 file if specified
	var notFoundPath string
	if config.notFoundFile != "" {
		notFoundPath = filepath.Join(config.root, config.notFoundFile)
		if err := validateStartup(notFoundPath, false); err != nil {
			panic("static.Website: " + err.Error())
		}
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Clean the URL path
			urlPath := path.Clean(r.URL.Path)
			originalPath := r.URL.Path // Keep original to check for trailing slash

			// Check if path should be excluded
			for _, exclude := range config.excludePaths {
				if urlPath == exclude || strings.HasPrefix(urlPath, exclude+"/") {
					serveNotFound(w, r, notFoundPath)
					return nil
				}
			}

			// SEO: Redirect /path/index.html to /path/
			if strings.HasSuffix(urlPath, "/"+config.indexFile) {
				// Build redirect path
				redirectPath := strings.TrimSuffix(originalPath, config.indexFile)
				http.Redirect(w, r, redirectPath, http.StatusMovedPermanently)
				return nil
			}

			// Construct file path
			filePath := filepath.Join(config.root, urlPath)

			// Validate that the path is within the root directory
			if err := validatePathSecurity(config.root, filePath); err != nil {
				serveNotFound(w, r, notFoundPath)
				return nil
			}

			// Check if path exists
			info, err := os.Stat(filePath)
			if err != nil {
				// Path doesn't exist, serve 404
				serveNotFound(w, r, notFoundPath)
				return nil
			}

			// If it's a directory
			if info.IsDir() {
				// Check if URL has trailing slash
				if !strings.HasSuffix(originalPath, "/") {
					// Redirect to add trailing slash for directories
					redirectPath := originalPath + "/"
					http.Redirect(w, r, redirectPath, http.StatusMovedPermanently)
					return nil
				}

				// Try to serve index file from directory
				indexPath := filepath.Join(filePath, config.indexFile)
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(w, r, indexPath)
					return nil
				}

				// No index file in directory, serve 404
				serveNotFound(w, r, notFoundPath)
				return nil
			}

			// It's a file, serve it directly
			http.ServeFile(w, r, filePath)
			return nil
		}
	}
}

// serveNotFound serves a 404 response with optional custom 404 page
func serveNotFound(w http.ResponseWriter, r *http.Request, customNotFoundPath string) {
	if customNotFoundPath != "" {
		w.WriteHeader(http.StatusNotFound)
		http.ServeFile(w, r, customNotFoundPath)
	} else {
		http.NotFound(w, r)
	}
}
