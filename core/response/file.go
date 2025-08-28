package response

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
)

// File creates a response that serves a static file from the filesystem.
// It automatically detects the content type and supports range requests.
// Returns 404 if the file doesn't exist or is a directory.
func File(path string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Prevent directory traversal attacks like ../../etc/passwd
		cleanPath := filepath.Clean(path)

		info, err := os.Stat(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return nil
			}
			return err
		}

		if info.IsDir() {
			http.NotFound(w, r)
			return nil
		}

		// http.ServeFile handles Range requests, If-Modified-Since, and content type detection
		http.ServeFile(w, r, cleanPath)
		return nil
	}
}

// Download creates a response that forces the browser to download the file
// instead of displaying it inline. If filename is empty, uses the base name
// of the file path.
func Download(path string, filename string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		cleanPath := filepath.Clean(path)

		info, err := os.Stat(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return nil
			}
			return err
		}

		if info.IsDir() {
			http.NotFound(w, r)
			return nil
		}

		downloadName := filename
		if downloadName == "" {
			downloadName = filepath.Base(cleanPath)
		}

		disposition := fmt.Sprintf(`attachment; filename="%s"`, downloadName)
		w.Header().Set("Content-Disposition", disposition)

		contentType := mime.TypeByExtension(filepath.Ext(cleanPath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)

		http.ServeFile(w, r, cleanPath)
		return nil
	}
}

// Attachment creates a response for downloading in-memory data as a file.
// This is useful for dynamically generated content that needs to be downloaded.
// If contentType is empty, it will be detected from the filename extension,
// defaulting to "application/octet-stream" if detection fails.
func Attachment(data []byte, filename string, contentType string) handler.Response {
	// Prevent HTTP header injection attacks through newlines and quotes
	sanitizedFilename := strings.ReplaceAll(filename, "\n", "")
	sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "\r", "")
	sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "\"", "'")

	return func(w http.ResponseWriter, r *http.Request) error {
		disposition := fmt.Sprintf(`attachment; filename="%s"`, sanitizedFilename)
		w.Header().Set("Content-Disposition", disposition)

		resolvedContentType := contentType
		if resolvedContentType == "" {
			resolvedContentType = mime.TypeByExtension(filepath.Ext(sanitizedFilename))
			if resolvedContentType == "" {
				resolvedContentType = "application/octet-stream"
			}
		}
		w.Header().Set("Content-Type", resolvedContentType)

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

		w.WriteHeader(http.StatusOK)
		_, err := w.Write(data)
		return err
	}
}

// FileReader creates a response that streams data from an io.Reader as a downloadable file.
// This is useful for large files or streams that shouldn't be loaded entirely into memory.
func FileReader(reader io.Reader, filename string, contentType string) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		sanitizedFilename := strings.ReplaceAll(filename, "\n", "")
		sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "\r", "")
		sanitizedFilename = strings.ReplaceAll(sanitizedFilename, "\"", "'")

		disposition := fmt.Sprintf(`attachment; filename="%s"`, sanitizedFilename)
		w.Header().Set("Content-Disposition", disposition)

		resolvedContentType := contentType
		if resolvedContentType == "" {
			resolvedContentType = mime.TypeByExtension(filepath.Ext(sanitizedFilename))
			if resolvedContentType == "" {
				resolvedContentType = "application/octet-stream"
			}
		}
		w.Header().Set("Content-Type", resolvedContentType)

		w.WriteHeader(http.StatusOK)
		_, err := io.Copy(w, reader)
		return err
	}
}

// CSV creates a response for downloading CSV data.
// The records should be a 2D slice where each inner slice is a row.
// The file will be served with content type "text/csv" and appropriate
// download headers.
func CSV(records [][]string, filename string) handler.Response {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write all records
	if err := w.WriteAll(records); err != nil {
		// Return error response if CSV writing fails
		return nil // Return nil on CSV generation failure
	}

	// Ensure .csv extension
	if !strings.HasSuffix(filename, ".csv") {
		filename = filename + ".csv"
	}

	return Attachment(buf.Bytes(), filename, "text/csv; charset=utf-8")
}

// CSVWithHeaders creates a response for downloading CSV data with custom headers.
// The first parameter is the header row, followed by data rows.
// This is a convenience wrapper around CSV for common use cases.
func CSVWithHeaders(headers []string, rows [][]string, filename string) handler.Response {
	records := append([][]string{headers}, rows...)
	return CSV(records, filename)
}
