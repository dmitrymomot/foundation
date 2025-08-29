package static_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/static"
)

// testContext provides a minimal context implementation for testing
type testContext struct {
	context.Context
	req *http.Request
	w   http.ResponseWriter
}

func (c *testContext) Request() *http.Request              { return c.req }
func (c *testContext) ResponseWriter() http.ResponseWriter { return c.w }
func (c *testContext) Param(key string) string             { return "" }
func (c *testContext) SetValue(key, val any)               {}

func newTestContext(req *http.Request, w http.ResponseWriter) *testContext {
	return &testContext{
		Context: context.Background(),
		req:     req,
		w:       w,
	}
}

func TestFile(t *testing.T) {
	t.Parallel()

	// Create temporary test files
	tmpDir := t.TempDir()

	// Regular text file
	textFile := filepath.Join(tmpDir, "test.txt")
	textContent := "Hello, World!"
	require.NoError(t, os.WriteFile(textFile, []byte(textContent), 0644))

	// HTML file for content type detection
	htmlFile := filepath.Join(tmpDir, "index.html")
	htmlContent := "<html><body>Test</body></html>"
	require.NoError(t, os.WriteFile(htmlFile, []byte(htmlContent), 0644))

	// CSS file
	cssFile := filepath.Join(tmpDir, "styles.css")
	cssContent := "body { color: red; }"
	require.NoError(t, os.WriteFile(cssFile, []byte(cssContent), 0644))

	// Binary file
	binFile := filepath.Join(tmpDir, "binary.dat")
	binContent := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	require.NoError(t, os.WriteFile(binFile, binContent, 0644))

	tests := []struct {
		name           string
		filePath       string
		expectedStatus int
		expectedBody   string
		checkHeaders   bool
	}{
		{
			name:           "serve_text_file",
			filePath:       textFile,
			expectedStatus: http.StatusOK,
			expectedBody:   textContent,
			checkHeaders:   true,
		},
		{
			name:           "serve_html_file",
			filePath:       htmlFile,
			expectedStatus: http.StatusOK,
			expectedBody:   htmlContent,
			checkHeaders:   true,
		},
		{
			name:           "serve_css_file",
			filePath:       cssFile,
			expectedStatus: http.StatusOK,
			expectedBody:   cssContent,
			checkHeaders:   true,
		},
		{
			name:           "serve_binary_file",
			filePath:       binFile,
			expectedStatus: http.StatusOK,
			expectedBody:   string(binContent),
			checkHeaders:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.File[*testContext](tt.filePath)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())

			if tt.checkHeaders {
				// Check content type is set
				contentType := w.Header().Get("Content-Type")
				assert.NotEmpty(t, contentType)

				// Check specific content types
				switch {
				case strings.HasSuffix(tt.filePath, ".html"):
					assert.Contains(t, contentType, "text/html")
				case strings.HasSuffix(tt.filePath, ".css"):
					assert.Contains(t, contentType, "text/css")
				case strings.HasSuffix(tt.filePath, ".txt"):
					assert.Contains(t, contentType, "text/plain")
				}
			}
		})
	}
}

func TestFileRangeRequests(t *testing.T) {
	t.Parallel()

	// Create test file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "range-test.txt")
	content := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	handler := static.File[*testContext](testFile)

	// Test range request
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Range", "bytes=10-19")
	w := httptest.NewRecorder()
	ctx := newTestContext(req, w)

	response := handler(ctx)
	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "ABCDEFGHIJ", w.Body.String())

	// Check Accept-Ranges header
	acceptRanges := w.Header().Get("Accept-Ranges")
	assert.Equal(t, "bytes", acceptRanges)
}

func TestFileConditionalRequests(t *testing.T) {
	t.Parallel()

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "conditional-test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	// Get file info for modification time
	info, err := os.Stat(testFile)
	require.NoError(t, err)
	modTime := info.ModTime()

	handler := static.File[*testContext](testFile)

	t.Run("if_modified_since_after_modification", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("If-Modified-Since", modTime.Add(-1).Format(http.TimeFormat))
		w := httptest.NewRecorder()
		ctx := newTestContext(req, w)

		response := handler(ctx)
		err := response(w, req)

		assert.NoError(t, err)
		assert.Contains(t, []int{http.StatusOK, http.StatusNotModified}, w.Code)
	})
}

func TestFileStartupValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("panic_on_nonexistent_file", func(t *testing.T) {
		t.Parallel()

		nonExistentFile := filepath.Join(tmpDir, "does-not-exist.txt")

		assert.Panics(t, func() {
			static.File[*testContext](nonExistentFile)
		})
	})

	t.Run("panic_on_directory", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			static.File[*testContext](tmpDir)
		})
	})

	t.Run("panic_on_permission_error", func(t *testing.T) {
		t.Parallel()

		// Skip permission tests as they don't work reliably across all environments
		t.Skip("Permission tests are environment-dependent and not reliable")

		restrictedDir := filepath.Join(tmpDir, "restricted")
		require.NoError(t, os.Mkdir(restrictedDir, 0000))
		defer os.Chmod(restrictedDir, 0755) // Clean up

		restrictedFile := filepath.Join(restrictedDir, "file.txt")

		assert.Panics(t, func() {
			static.File[*testContext](restrictedFile)
		})
	})

	t.Run("clean_path_handling", func(t *testing.T) {
		t.Parallel()

		// Create a file
		testFile := filepath.Join(tmpDir, "clean-test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

		// Path with redundant elements should work (cleaned internally)
		messyPath := filepath.Join(tmpDir, ".", "clean-test.txt")

		assert.NotPanics(t, func() {
			static.File[*testContext](messyPath)
		})
	})
}

func TestFileSecurityValidation(t *testing.T) {
	t.Parallel()

	// Create directory structure
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	require.NoError(t, os.Mkdir(publicDir, 0755))

	secretFile := filepath.Join(tmpDir, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret content"), 0644))

	publicFile := filepath.Join(publicDir, "public.txt")
	require.NoError(t, os.WriteFile(publicFile, []byte("public content"), 0644))

	t.Run("path_traversal_cleaned_at_startup", func(t *testing.T) {
		t.Parallel()

		// Attempting to serve a file via path traversal
		// The path gets cleaned at startup, so it should work if the file exists
		traversalPath := filepath.Join(publicDir, "../secret.txt")

		// This should not panic because filepath.Clean resolves it to the actual file
		assert.NotPanics(t, func() {
			handler := static.File[*testContext](traversalPath)

			// Test that it serves the correct file
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "secret content", w.Body.String())
		})
	})
}

func TestFileMethodsAllowed(t *testing.T) {
	t.Parallel()

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "methods-test.txt")
	content := "test content"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	handler := static.File[*testContext](testFile)

	methods := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)

			// http.ServeFile serves content for all methods, but only GET and HEAD return body
			assert.Equal(t, http.StatusOK, w.Code)
			if method == "GET" {
				assert.Equal(t, content, w.Body.String())
			} else if method == "HEAD" {
				assert.Empty(t, w.Body.String())
			}
		})
	}
}

func TestFileEmptyFile(t *testing.T) {
	t.Parallel()

	// Create empty file
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	require.NoError(t, os.WriteFile(emptyFile, []byte{}, 0644))

	handler := static.File[*testContext](emptyFile)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	ctx := newTestContext(req, w)

	response := handler(ctx)
	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
	assert.Equal(t, "0", w.Header().Get("Content-Length"))
}
