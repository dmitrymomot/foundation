package static

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
)

// spaConfig holds configuration options for Single Page Application serving.
// This struct is used internally by SPA() to manage client-side routing behavior.
type spaConfig struct {
	root         string
	indexFile    string
	notFoundFile string
	excludePaths []string
	stripPrefix  string
}

// SPAOption is a functional option type for configuring SPA serving behavior.
// Use with SPA() to customize how single page applications are served.
type SPAOption func(*spaConfig)

// WithSPAIndex sets the index file for the SPA (default: "index.html").
// This file is served when no matching static file is found, enabling
// client-side routing for single page applications.
//
// The indexFile parameter should be relative to the SPA root directory.
func WithSPAIndex(indexFile string) SPAOption {
	return func(c *spaConfig) {
		c.indexFile = indexFile
	}
}

// WithNotFoundPage sets a custom 404 page for the SPA.
// This page is served with a 404 status code when files are not found,
// instead of falling back to the index file.
//
// This is useful for showing a proper 404 page for invalid routes
// that shouldn't be handled by client-side routing.
//
// The notFoundFile parameter should be relative to the SPA root directory.
func WithNotFoundPage(notFoundFile string) SPAOption {
	return func(c *spaConfig) {
		c.notFoundFile = notFoundFile
	}
}

// WithExcludePaths sets paths that should be excluded from SPA handling.
// These paths will return 404 instead of falling back to index.html.
//
// This is essential for preventing API routes, WebSocket endpoints, or other
// server-side routes from accidentally serving the SPA index file.
//
// Default excluded paths are "/api" and "/ws".
//
// Example:
//
//	static.WithExcludePaths("/api", "/ws", "/health", "/metrics")
func WithExcludePaths(paths ...string) SPAOption {
	return func(c *spaConfig) {
		c.excludePaths = paths
	}
}

// WithSPAStripPrefix removes the given prefix from the URL path before serving files.
// This is useful when the SPA is mounted under a specific route prefix.
//
// For example, if the SPA is served at "/app/*" but files are in "./dist/",
// use WithSPAStripPrefix("/app") to properly map URLs to filesystem paths.
func WithSPAStripPrefix(prefix string) SPAOption {
	return func(c *spaConfig) {
		c.stripPrefix = prefix
	}
}

// SPA creates a handler for serving Single Page Applications.
//
// This handler implements the standard SPA serving pattern:
// 1. If a static file exists at the requested path, serve it
// 2. If a directory exists with index.html, serve the index.html
// 3. If the path is excluded (e.g., API routes), return 404
// 4. Otherwise, fall back to the main index file for client-side routing
//
// This pattern enables client-side routing frameworks (React Router, Vue Router, etc.)
// to handle navigation while still serving static assets normally.
//
// Features:
// - Automatic static file serving with proper content types
// - Client-side routing fallback to index.html
// - Configurable exclude paths for API routes
// - Custom 404 page support
// - HTTP range requests and caching headers
//
// Parameters:
//   - root: The root directory containing the SPA build files (must exist at startup)
//   - opts: Optional configuration functions for customizing behavior
//
// Panics at startup if:
//   - The root directory doesn't exist
//   - The index file doesn't exist
//   - The custom 404 file (if specified) doesn't exist
//   - Any specified files are not accessible
//
// Example for React/Vue/Angular applications:
//
//	// Basic SPA serving
//	spaHandler := static.SPA[MyContext]("./dist")
//
//	// Advanced configuration
//	spaHandler := static.SPA[MyContext](
//		"./build",
//		static.WithSPAIndex("app.html"),
//		static.WithNotFoundPage("404.html"),
//		static.WithExcludePaths("/api", "/ws", "/health"),
//		static.WithSPAStripPrefix("/app"),
//	)
func SPA[C handler.Context](root string, opts ...SPAOption) handler.HandlerFunc[C] {
	config := &spaConfig{
		root:         filepath.Clean(root),
		indexFile:    "index.html",
		notFoundFile: "",
		excludePaths: []string{"/api", "/ws"},
		stripPrefix:  "",
	}

	for _, opt := range opts {
		opt(config)
	}

	// Validate root directory exists
	rootInfo, err := os.Stat(config.root)
	if err != nil {
		if os.IsNotExist(err) {
			panic("static.SPA: root directory does not exist: " + config.root)
		}
		panic("static.SPA: error accessing root directory: " + err.Error())
	}

	if !rootInfo.IsDir() {
		panic("static.SPA: root path is not a directory: " + config.root)
	}

	// Prepare and validate index file path
	indexPath := filepath.Join(config.root, config.indexFile)
	if _, err := os.Stat(indexPath); err != nil {
		if os.IsNotExist(err) {
			panic("static.SPA: index file does not exist: " + indexPath)
		}
		panic("static.SPA: error accessing index file: " + err.Error())
	}

	// Prepare and validate 404 file path if specified
	var notFoundPath string
	if config.notFoundFile != "" {
		notFoundPath = filepath.Join(config.root, config.notFoundFile)
		if _, err := os.Stat(notFoundPath); err != nil {
			if os.IsNotExist(err) {
				panic("static.SPA: 404 file does not exist: " + notFoundPath)
			}
			panic("static.SPA: error accessing 404 file: " + err.Error())
		}
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Clean the URL path
			urlPath := path.Clean(r.URL.Path)

			// Strip prefix if configured
			if config.stripPrefix != "" {
				urlPath = strings.TrimPrefix(urlPath, config.stripPrefix)
			}

			// Check if path should be excluded from SPA handling
			for _, exclude := range config.excludePaths {
				if strings.HasPrefix(urlPath, exclude) {
					http.NotFound(w, r)
					return nil
				}
			}

			// Construct file path
			filePath := filepath.Join(config.root, urlPath)

			// Check if file exists
			info, err := os.Stat(filePath)
			if err == nil && !info.IsDir() {
				// File exists, serve it
				http.ServeFile(w, r, filePath)
				return nil
			}

			// Check if it's a directory with index.html
			if err == nil && info.IsDir() {
				indexInDir := filepath.Join(filePath, "index.html")
				if _, err := os.Stat(indexInDir); err == nil {
					http.ServeFile(w, r, indexInDir)
					return nil
				}
			}

			// File doesn't exist, check for custom 404 page
			if notFoundPath != "" {
				if _, err := os.Stat(notFoundPath); err == nil {
					w.WriteHeader(http.StatusNotFound)
					http.ServeFile(w, r, notFoundPath)
					return nil
				}
			}

			// Fall back to index.html for client-side routing
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return nil
			}

			// Index file doesn't exist either
			http.NotFound(w, r)
			return nil
		}
	}
}
