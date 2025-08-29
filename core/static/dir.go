package static

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
)

// dirConfig holds configuration options for directory serving.
// This struct is used internally by Dir() to manage serving behavior.
type dirConfig[C handler.Context] struct {
	root         string
	stripPrefix  string
	errorHandler func(ctx C, err error) handler.Response
}

// DirOption is a functional option type for configuring directory serving behavior.
// Use with Dir() to customize how static files are served from a directory.
type DirOption[C handler.Context] func(*dirConfig[C])

// WithStripPrefix removes the given prefix from the URL path before serving files.
// This is useful when mounting static files under a specific route prefix.
//
// For example, if files are mounted at "/static/" but stored in "./assets/",
// use WithStripPrefix("/static") so "/static/css/style.css" serves "./assets/css/style.css".
//
// The prefix parameter should include leading slash if needed for proper path matching.
func WithStripPrefix[C handler.Context](prefix string) DirOption[C] {
	return func(c *dirConfig[C]) {
		c.stripPrefix = prefix
	}
}

// WithErrorHandler sets a custom handler for file serving errors.
// This allows custom error pages or fallback behavior for various error conditions
// including file not found, permission denied, and other filesystem errors.
//
// The handler function receives the context and the error that occurred,
// and should return a Response that handles the error appropriately.
// The default error handler uses http.Error to return standard HTTP error responses.
//
// Example:
//
//	func customErrorHandler(ctx *MyContext, err error) handler.Response {
//		return func(w http.ResponseWriter, r *http.Request) error {
//			if os.IsNotExist(err) {
//				w.WriteHeader(http.StatusNotFound)
//				w.Write([]byte("<h1>File Not Found</h1>"))
//			} else {
//				http.Error(w, err.Error(), http.StatusInternalServerError)
//			}
//			return nil
//		}
//	}
func WithErrorHandler[C handler.Context](handler func(ctx C, err error) handler.Response) DirOption[C] {
	return func(c *dirConfig[C]) {
		c.errorHandler = handler
	}
}

// defaultErrorHandler provides standard HTTP error responses for file serving errors.
// It maps common filesystem errors to appropriate HTTP status codes:
// - os.IsNotExist -> 404 Not Found
// - os.IsPermission -> 403 Forbidden
// - Other errors -> 500 Internal Server Error
func defaultErrorHandler[C handler.Context](ctx C, err error) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		if os.IsNotExist(err) {
			http.Error(w, "404 page not found", http.StatusNotFound)
		} else if os.IsPermission(err) {
			http.Error(w, "403 forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
		}
		return nil
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
// - Uses a default error handler that maps filesystem errors to HTTP status codes
//
// Parameters:
//   - root: The root directory path to serve files from (must exist at startup)
//   - opts: Optional configuration functions (WithStripPrefix, WithErrorHandler)
//
// Panics at startup if:
//   - The directory doesn't exist
//   - The path is not a directory
//   - The directory is not accessible
//
// Example:
//
//	// Basic directory serving with default error handling
//	handler := static.Dir[MyContext]("./public")
//
//	// With custom options
//	handler := static.Dir[MyContext](
//		"./assets",
//		static.WithStripPrefix("/static"),
//		static.WithErrorHandler(customErrorHandler),
//	)
func Dir[C handler.Context](root string, opts ...DirOption[C]) handler.HandlerFunc[C] {
	config := &dirConfig[C]{
		root:         filepath.Clean(root),
		stripPrefix:  "",
		errorHandler: defaultErrorHandler[C],
	}

	for _, opt := range opts {
		opt(config)
	}

	// Validate directory exists at startup
	info, err := os.Stat(config.root)
	if err != nil {
		if os.IsNotExist(err) {
			panic("static.Dir: directory does not exist: " + config.root)
		}
		panic("static.Dir: error accessing directory: " + err.Error())
	}

	if !info.IsDir() {
		panic("static.Dir: path is not a directory: " + config.root)
	}

	fileServer := http.FileServer(neuteredFileSystem{http.Dir(config.root)})

	if config.stripPrefix != "" {
		fileServer = http.StripPrefix(config.stripPrefix, fileServer)
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Clean the URL path to prevent directory traversal
			cleanPath := path.Clean(r.URL.Path)

			// Check if file exists and use error handler for any errors
			fullPath := filepath.Join(config.root, strings.TrimPrefix(cleanPath, config.stripPrefix))

			// Additional security: Validate that the path is within the root directory
			cleanFullPath := filepath.Clean(fullPath)
			cleanRoot := filepath.Clean(config.root)
			if !strings.HasPrefix(cleanFullPath, cleanRoot+string(filepath.Separator)) && cleanFullPath != cleanRoot {
				return config.errorHandler(ctx, fmt.Errorf("invalid file path: %s", fullPath))
			}

			if _, err := os.Stat(fullPath); err != nil {
				response := config.errorHandler(ctx, err)
				return response(w, r)
			}

			fileServer.ServeHTTP(w, r)
			return nil
		}
	}
}

// neuteredFileSystem wraps http.FileSystem to disable directory listing for security.
// It only allows directory access if an index.html file is present.
type neuteredFileSystem struct {
	http.FileSystem
}

// Open implements http.FileSystem.Open with directory listing disabled.
// Directories are only accessible if they contain an index.html file.
func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.FileSystem.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	if s.IsDir() {
		// Check if index.html exists in directory
		index := filepath.Join(path, "index.html")
		if _, err := nfs.FileSystem.Open(index); err != nil {
			_ = f.Close()
			return nil, fs.ErrNotExist
		}
	}

	return f, nil
}
