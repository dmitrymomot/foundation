package static

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
)

// fsConfig holds configuration for fs.FS serving
type fsConfig struct {
	fs          fs.FS
	stripPrefix string
	subPath     string
}

// FSOption configures fs.FS serving behavior
type FSOption func(*fsConfig)

// WithFSStripPrefix removes the given prefix from the URL path before serving files.
func WithFSStripPrefix(prefix string) FSOption {
	return func(c *fsConfig) {
		c.stripPrefix = prefix
	}
}

// WithSubFS serves files from a subdirectory within the fs.FS.
// This is useful when the embedded filesystem contains multiple directories.
func WithSubFS(path string) FSOption {
	return func(c *fsConfig) {
		c.subPath = path
	}
}

// FS creates a handler that serves files from an fs.FS (including embed.FS).
// This is useful for embedding static assets in the binary.
// Panics at startup if the sub-path is invalid.
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

	fileServer := http.FileServer(neuteredFS{http.FS(config.fs)})

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

// neuteredFS wraps http.FileSystem for embed.FS to disable directory listing
type neuteredFS struct {
	http.FileSystem
}

func (nfs neuteredFS) Open(path string) (http.File, error) {
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
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := nfs.FileSystem.Open(index); err != nil {
			f.Close()
			return nil, fs.ErrNotExist
		}
	}

	return f, nil
}
