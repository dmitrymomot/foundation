package static

import (
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
type dirConfig struct {
	root            string
	stripPrefix     string
	notFoundHandler func(w http.ResponseWriter, r *http.Request) error
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

// WithNotFound sets a custom handler for when files are not found.
// This allows custom 404 pages or fallback behavior instead of the default HTTP 404.
//
// The handler function receives the original http.ResponseWriter and *http.Request,
// and should return an error if the response writing fails. The handler is responsible
// for setting appropriate status codes and response content.
//
// Example:
//
//	func custom404(w http.ResponseWriter, r *http.Request) error {
//		w.WriteHeader(http.StatusNotFound)
//		w.Write([]byte("<h1>Page Not Found</h1>"))
//		return nil
//	}
func WithNotFound(handler func(w http.ResponseWriter, r *http.Request) error) DirOption {
	return func(c *dirConfig) {
		c.notFoundHandler = handler
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
//
// Parameters:
//   - root: The root directory path to serve files from (must exist at startup)
//   - opts: Optional configuration functions (WithStripPrefix, WithNotFound)
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
//	// With custom options
//	handler := static.Dir[MyContext](
//		"./assets",
//		static.WithStripPrefix("/static"),
//		static.WithNotFound(custom404Handler),
//	)
func Dir[C handler.Context](root string, opts ...DirOption) handler.HandlerFunc[C] {
	config := &dirConfig{
		root:        filepath.Clean(root),
		stripPrefix: "",
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

			// Check if custom 404 handler is set
			if config.notFoundHandler != nil {
				fullPath := filepath.Join(config.root, strings.TrimPrefix(cleanPath, config.stripPrefix))
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					return config.notFoundHandler(w, r)
				}
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
		f.Close()
		return nil, err
	}

	if s.IsDir() {
		// Check if index.html exists in directory
		index := filepath.Join(path, "index.html")
		if _, err := nfs.FileSystem.Open(index); err != nil {
			f.Close()
			return nil, fs.ErrNotExist
		}
	}

	return f, nil
}
