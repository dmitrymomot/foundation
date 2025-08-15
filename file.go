package gokit

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// fileResponse implements Response for serving static files.
type fileResponse struct {
	path string
}

// Render serves the file using http.ServeFile for efficiency.
func (r fileResponse) Render(w http.ResponseWriter, req *http.Request) error {
	cleanPath := filepath.Clean(r.path) // prevent directory traversal

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, req)
			return nil
		}
		return err
	}

	// Don't serve directories
	if info.IsDir() {
		http.NotFound(w, req)
		return nil
	}

	// Use http.ServeFile for efficient file serving
	// It handles Range requests, If-Modified-Since, and content type detection
	http.ServeFile(w, req, cleanPath)
	return nil
}

// downloadResponse implements Response for forced file downloads.
type downloadResponse struct {
	path     string
	filename string
}

// Render serves the file as a download with Content-Disposition header.
func (r downloadResponse) Render(w http.ResponseWriter, req *http.Request) error {
	cleanPath := filepath.Clean(r.path) // prevent directory traversal

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, req)
			return nil
		}
		return err
	}

	// Don't serve directories
	if info.IsDir() {
		http.NotFound(w, req)
		return nil
	}

	// Determine filename for download
	downloadName := r.filename
	if downloadName == "" {
		downloadName = filepath.Base(cleanPath)
	}

	disposition := fmt.Sprintf(`attachment; filename="%s"`, downloadName)
	w.Header().Set("Content-Disposition", disposition)

	// Detect and set content type
	contentType := mime.TypeByExtension(filepath.Ext(cleanPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)

	// Serve the file
	http.ServeFile(w, req, cleanPath)
	return nil
}

// attachmentResponse implements Response for in-memory file downloads.
type attachmentResponse struct {
	data        []byte
	filename    string
	contentType string
}

// Render serves in-memory data as a downloadable attachment.
func (r attachmentResponse) Render(w http.ResponseWriter, req *http.Request) error {
	disposition := fmt.Sprintf(`attachment; filename="%s"`, r.filename)
	w.Header().Set("Content-Disposition", disposition)

	contentType := r.contentType
	if contentType == "" {
		// Try to detect from filename extension
		contentType = mime.TypeByExtension(filepath.Ext(r.filename))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(r.data)))

	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.data)
	return err
}

// File creates a response that serves a static file from the filesystem.
// It automatically detects the content type and supports range requests.
// Returns 404 if the file doesn't exist or is a directory.
func File(path string) Response {
	return fileResponse{path: path}
}

// Download creates a response that forces the browser to download the file
// instead of displaying it inline. If filename is empty, uses the base name
// of the file path.
func Download(path string, filename string) Response {
	return downloadResponse{
		path:     path,
		filename: filename,
	}
}

// Attachment creates a response for downloading in-memory data as a file.
// This is useful for dynamically generated content that needs to be downloaded.
// If contentType is empty, it will be detected from the filename extension,
// defaulting to "application/octet-stream" if detection fails.
func Attachment(data []byte, filename string, contentType string) Response {
	// Sanitize filename to prevent header injection
	filename = strings.ReplaceAll(filename, "\n", "")
	filename = strings.ReplaceAll(filename, "\r", "")
	filename = strings.ReplaceAll(filename, "\"", "'")

	return attachmentResponse{
		data:        data,
		filename:    filename,
		contentType: contentType,
	}
}

// FileReader creates a response that streams data from an io.Reader as a downloadable file.
// This is useful for large files or streams that shouldn't be loaded entirely into memory.
func FileReader(reader io.Reader, filename string, contentType string) Response {
	return &streamFileResponse{
		reader:      reader,
		filename:    filename,
		contentType: contentType,
	}
}

// streamFileResponse implements Response for streaming file downloads.
type streamFileResponse struct {
	reader      io.Reader
	filename    string
	contentType string
}

// Render streams data from the reader as a downloadable file.
func (r *streamFileResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Sanitize filename
	filename := strings.ReplaceAll(r.filename, "\n", "")
	filename = strings.ReplaceAll(filename, "\r", "")
	filename = strings.ReplaceAll(filename, "\"", "'")

	disposition := fmt.Sprintf(`attachment; filename="%s"`, filename)
	w.Header().Set("Content-Disposition", disposition)

	contentType := r.contentType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)

	// Stream the content
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, r.reader)
	return err
}
