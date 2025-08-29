package static

import (
	"net/http"
	"path/filepath"

	"github.com/dmitrymomot/foundation/core/handler"
)

// File creates a handler that serves a single static file.
//
// The handler provides efficient file serving with the following features:
// - Automatic content type detection based on file extension
// - HTTP range request support for partial content (useful for video/audio)
// - Proper HTTP caching headers (Last-Modified, ETag)
// - Efficient sendfile system calls when available
//
// Parameters:
//   - filePath: Absolute or relative path to the file to serve (must exist at startup)
//
// Panics at startup if:
//   - The file doesn't exist
//   - The path points to a directory instead of a file
//   - The file is not accessible due to permissions
//
// Example:
//
//	// Serve a favicon
//	faviconHandler := static.File[MyContext]("./public/favicon.ico")
//
//	// Serve a CSS file
//	styleHandler := static.File[MyContext]("/var/www/assets/style.css")
//
// The handler integrates with any HTTP router that accepts http.HandlerFunc:
//
//	mux.Handle("GET /favicon.ico", handler.Adapt(faviconHandler))
func File[C handler.Context](filePath string) handler.HandlerFunc[C] {
	// Validate at startup
	cleanPath := filepath.Clean(filePath)

	if err := validateStartup(cleanPath, false); err != nil {
		panic("static.File: " + err.Error())
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			http.ServeFile(w, r, cleanPath)
			return nil
		}
	}
}
