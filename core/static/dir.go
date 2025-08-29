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

// dirConfig holds configuration for directory serving
type dirConfig struct {
	root            string
	stripPrefix     string
	notFoundHandler func(w http.ResponseWriter, r *http.Request) error
}

// DirOption configures directory serving behavior
type DirOption func(*dirConfig)

// WithStripPrefix removes the given prefix from the URL path before serving files.
// This is useful when mounting static files under a specific route prefix.
func WithStripPrefix(prefix string) DirOption {
	return func(c *dirConfig) {
		c.stripPrefix = prefix
	}
}

// WithNotFound sets a custom handler for when files are not found.
// This allows custom 404 pages or fallback behavior.
func WithNotFound(handler func(w http.ResponseWriter, r *http.Request) error) DirOption {
	return func(c *dirConfig) {
		c.notFoundHandler = handler
	}
}

// Dir creates a handler that serves files from a directory.
// Directory listing is disabled by default for security.
// Panics at startup if the directory doesn't exist.
// Use DirOption functions to customize behavior.
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

// neuteredFileSystem wraps http.FileSystem to disable directory listing
type neuteredFileSystem struct {
	http.FileSystem
}

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
