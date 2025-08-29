package static

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
)

// spaConfig holds configuration for SPA serving
type spaConfig struct {
	root         string
	indexFile    string
	notFoundFile string
	excludePaths []string
	stripPrefix  string
}

// SPAOption configures SPA serving behavior
type SPAOption func(*spaConfig)

// WithSPAIndex sets the index file for the SPA (default: "index.html").
func WithSPAIndex(indexFile string) SPAOption {
	return func(c *spaConfig) {
		c.indexFile = indexFile
	}
}

// WithNotFoundPage sets a custom 404 page for the SPA.
// This page is served with a 404 status when files are not found.
func WithNotFoundPage(notFoundFile string) SPAOption {
	return func(c *spaConfig) {
		c.notFoundFile = notFoundFile
	}
}

// WithExcludePaths sets paths that should be excluded from SPA handling.
// These paths will return 404 instead of falling back to index.html.
// Useful for API routes or WebSocket endpoints.
func WithExcludePaths(paths ...string) SPAOption {
	return func(c *spaConfig) {
		c.excludePaths = paths
	}
}

// WithSPAStripPrefix removes the given prefix from the URL path before serving files.
func WithSPAStripPrefix(prefix string) SPAOption {
	return func(c *spaConfig) {
		c.stripPrefix = prefix
	}
}

// SPA creates a handler for serving Single Page Applications.
// It serves static files if they exist, otherwise falls back to index.html
// for client-side routing. This allows SPA frameworks to handle their own routing.
// Panics at startup if the root directory or index file doesn't exist.
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
