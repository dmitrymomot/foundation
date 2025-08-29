package static

import (
	"io/fs"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
)

// fsConfig holds configuration options for fs.FS serving.
// This struct is used internally by FS() to manage embedded filesystem serving.
type fsConfig struct {
	fs          fs.FS
	stripPrefix string
	subPath     string
}

// FSOption is a functional option type for configuring fs.FS serving behavior.
// Use with FS() to customize how embedded files are served.
type FSOption func(*fsConfig)

// WithFSStripPrefix removes the given prefix from the URL path before serving files.
// This is useful when embedding files under a specific route prefix.
//
// For example, if embedded files are mounted at "/assets/" but stored in "static/",
// use WithFSStripPrefix("/assets") so "/assets/style.css" serves "static/style.css".
func WithFSStripPrefix(prefix string) FSOption {
	return func(c *fsConfig) {
		c.stripPrefix = prefix
	}
}

// WithSubFS serves files from a subdirectory within the fs.FS.
// This is useful when the embedded filesystem contains multiple directories
// and you want to serve only a specific subdirectory.
//
// For example, if your embed.FS contains "assets/css/" and "assets/js/",
// use WithSubFS("assets/css") to serve only the CSS files.
//
// The path parameter should use forward slashes regardless of OS.
func WithSubFS(path string) FSOption {
	return func(c *fsConfig) {
		c.subPath = path
	}
}

// FS creates a handler that serves files from an fs.FS (including embed.FS).
//
// This handler is designed for serving embedded static assets that are compiled
// into the binary using Go's embed directive. It provides the same features as Dir()
// but works with any filesystem that implements fs.FS.
//
// Features:
// - Directory listing disabled for security
// - Serves index.html when present in directories
// - Supports HTTP range requests and caching headers
// - Works with embed.FS, os.DirFS, and any fs.FS implementation
//
// Parameters:
//   - fsys: The filesystem to serve files from (embed.FS, os.DirFS, etc.)
//   - opts: Optional configuration functions (WithFSStripPrefix, WithSubFS)
//
// Panics at startup if:
//   - The sub-path specified in WithSubFS is invalid
//   - The filesystem root is not accessible
//
// Example with embed.FS:
//
//	//go:embed assets/*
//	var assetsFS embed.FS
//
//	// Serve all embedded files
//	handler := static.FS[MyContext](assetsFS)
//
//	// Serve only from "dist" subdirectory with prefix stripping
//	handler := static.FS[MyContext](
//		assetsFS,
//		static.WithSubFS("dist"),
//		static.WithFSStripPrefix("/static"),
//	)
func FS[C handler.Context](fsys fs.FS, opts ...FSOption) handler.HandlerFunc[C] {
	config := &fsConfig{
		fs:          fsys,
		stripPrefix: "",
	}

	for _, opt := range opts {
		opt(config)
	}

	// Validate and use sub filesystem if path is specified
	if config.subPath != "" {
		sub, err := fs.Sub(fsys, config.subPath)
		if err != nil {
			panic("static.FS: invalid sub-path '" + config.subPath + "': " + err.Error())
		}
		config.fs = sub
	}

	// Validate that the filesystem is accessible by trying to open root
	if _, err := config.fs.Open("."); err != nil {
		panic("static.FS: filesystem is not accessible: " + err.Error())
	}

	fileServer := http.FileServer(neuteredFileSystem{fs: http.FS(config.fs)})

	if config.stripPrefix != "" {
		fileServer = http.StripPrefix(config.stripPrefix, fileServer)
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			fileServer.ServeHTTP(w, r)
			return nil
		}
	}
}
