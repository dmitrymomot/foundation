package static

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/dmitrymomot/foundation/core/handler"
)

// File creates a handler that serves a single static file.
// It automatically detects content type and supports range requests.
// Panics at startup if the file doesn't exist or is a directory.
func File[C handler.Context](filePath string) handler.HandlerFunc[C] {
	// Validate at startup
	cleanPath := filepath.Clean(filePath)

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			panic("static.File: file does not exist: " + cleanPath)
		}
		panic("static.File: error accessing file: " + err.Error())
	}

	if info.IsDir() {
		panic("static.File: path is a directory, not a file: " + cleanPath)
	}

	return func(ctx C) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			http.ServeFile(w, r, cleanPath)
			return nil
		}
	}
}
