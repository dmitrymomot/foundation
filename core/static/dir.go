package static

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
)

// dirConfig holds configuration options for directory serving.
// This struct is used internally by Dir() to manage serving behavior.
type dirConfig struct {
	root        string
	stripPrefix string
}

// DirOption is a functional option type for configuring directory serving behavior.
// Use with Dir() to customize how static files are served from a directory.
type DirOption func(*dirConfig)

// WithStripPrefix removes the given prefix from the URL path before serving files.
// This is useful when mounting static files under a specific route prefix.
//
// For example, if files are mounted at "/static/" but stored in "./assets/",
// use WithStripPrefix("/static") so "/static/css/style.css" serves "./assets/css/style.css".
//
// The prefix parameter should include leading slash if needed for proper path matching.
func WithStripPrefix(prefix string) DirOption {
	return func(c *dirConfig) {
		c.stripPrefix = prefix
	}
}

// Dir creates a handler that serves files from a directory.
//
// The handler serves static files from the specified root directory with the following behavior:
// - Directory listing is disabled by default for security
// - Serves index.html when present in requested directories
// - Supports HTTP range requests for partial content
// - Automatically detects content types
// - Cleans URL paths to prevent directory traversal attacks
// - Returns appropriate HTTP errors for filesystem errors
//
// Parameters:
//   - root: The root directory path to serve files from (must exist at startup)
//   - opts: Optional configuration functions (WithStripPrefix)
//
// Panics at startup if:
//   - The directory doesn't exist
//   - The path is not a directory
//   - The directory is not accessible
//
// Example:
//
//	// Basic directory serving
//	handler := static.Dir[MyContext]("./public")
//
//	// With strip prefix
//	handler := static.Dir[MyContext](
//		"./assets",
//		static.WithStripPrefix("/static"),
//	)
//
// statusCaptureWriter captures the status code written by the handler
type statusCaptureWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCaptureWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

func Dir[C handler.Context](root string, opts ...DirOption) handler.HandlerFunc[C] {
	config := &dirConfig{
		root:        filepath.Clean(root),
		stripPrefix: "",
	}

	for _, opt := range opts {
		opt(config)
	}

	// Validate directory exists at startup
	if err := validateStartup(config.root, true); err != nil {
		panic("static.Dir: " + err.Error())
	}

	fileServer := http.FileServer(neuteredFileSystem{fs: http.Dir(config.root)})

	if config.stripPrefix != "" {
		fileServer = http.StripPrefix(config.stripPrefix, fileServer)
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Clean the URL path to prevent directory traversal
			cleanPath := path.Clean(r.URL.Path)

			// If a prefix is configured, check if the path starts with it
			if config.stripPrefix != "" && !strings.HasPrefix(cleanPath, config.stripPrefix) {
				return response.ErrNotFound
			}

			// Validate path security
			fullPath := filepath.Join(config.root, strings.TrimPrefix(cleanPath, config.stripPrefix))
			if err := validatePathSecurity(config.root, fullPath); err != nil {
				return response.ErrBadRequest.WithMessage("Invalid path")
			}

			// Use a custom ResponseWriter to capture 404s from neuteredFileSystem
			captureWriter := &statusCaptureWriter{ResponseWriter: w, statusCode: http.StatusOK}
			fileServer.ServeHTTP(captureWriter, r)

			// If the fileServer returned 404, return an error
			if captureWriter.statusCode == http.StatusNotFound {
				return response.ErrNotFound
			}

			return nil
		}
	}
}
