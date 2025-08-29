package static

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// validatePathSecurity ensures the requested path is within the root directory.
// It prevents directory traversal attacks by cleaning and validating paths.
func validatePathSecurity(root, requestPath string) error {
	cleanPath := filepath.Clean(requestPath)
	cleanRoot := filepath.Clean(root)

	// Check if path is within root directory
	if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) && cleanPath != cleanRoot {
		return fmt.Errorf("invalid path: outside root directory")
	}

	return nil
}

// validateStartup checks that a file or directory exists and is accessible at startup.
// This is used to fail-fast during initialization rather than at runtime.
func validateStartup(path string, mustBeDir bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if mustBeDir {
				return fmt.Errorf("directory does not exist: %s", path)
			}
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("error accessing path: %w", err)
	}

	if mustBeDir && !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	if !mustBeDir && info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	return nil
}

// neuteredFileSystem wraps http.FileSystem to disable directory listing for security.
// It only allows directory access if an index.html file is present.
type neuteredFileSystem struct {
	fs http.FileSystem
}

// Open implements http.FileSystem.Open with directory listing disabled.
// Directories are only accessible if they contain an index.html file.
func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
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
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if path == "/" || path == "" {
			index = "/index.html"
		}

		if _, err := nfs.fs.Open(index); err != nil {
			_ = f.Close()
			return nil, fs.ErrNotExist
		}
	}

	return f, nil
}
