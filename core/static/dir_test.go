package static_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/static"
)

func TestDir(t *testing.T) {
	t.Parallel()

	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("file content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "styles.css"), []byte("body { color: blue; }"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "script.js"), []byte("console.log('hello');"), 0644))

	// Create subdirectory with files but NO index.html
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested content"), 0644))

	// Create another subdirectory with index.html
	subDirWithIndex := filepath.Join(tmpDir, "subdirwithindex")
	require.NoError(t, os.Mkdir(subDirWithIndex, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDirWithIndex, "index.html"), []byte("<html>Sub page</html>"), 0644))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
		checkHeaders   bool
	}{
		{
			name:           "serve_text_file",
			urlPath:        "/file.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "file content",
			checkHeaders:   true,
		},
		{
			name:           "serve_css_file",
			urlPath:        "/styles.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: blue; }",
			checkHeaders:   true,
		},
		{
			name:           "serve_js_file",
			urlPath:        "/script.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('hello');",
			checkHeaders:   true,
		},
		{
			name:           "serve_file_from_subdirectory",
			urlPath:        "/subdir/nested.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "nested content",
			checkHeaders:   true,
		},
		{
			name:           "serve_index_from_subdirectory_with_slash",
			urlPath:        "/subdirwithindex/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<html>Sub page</html>",
			checkHeaders:   true,
		},
		{
			name:           "directory_without_index_returns_404",
			urlPath:        "/subdir/",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
		{
			name:           "root_directory_without_index_returns_404",
			urlPath:        "/",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
		{
			name:           "nonexistent_file_returns_404",
			urlPath:        "/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.Dir[*testContext](tmpDir)
			req := httptest.NewRequest("GET", tt.urlPath, nil)
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
			}
		})
	}
}

func TestDirWithIndex(t *testing.T) {
	t.Parallel()

	// Create directory structure with index files
	tmpDir := t.TempDir()

	// Root index.html
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html>Root index</html>"), 0644))

	// Subdirectory with index.html
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "index.html"), []byte("<html>Sub index</html>"), 0644))

	handler := static.Dir[*testContext](tmpDir)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_root_index",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<html>Root index</html>",
		},
		{
			name:           "serve_root_index_explicit",
			urlPath:        "/index.html",
			expectedStatus: http.StatusMovedPermanently, // http.FileServer redirects /index.html to /
			expectedBody:   "",
		},
		{
			name:           "serve_subdir_index",
			urlPath:        "/subdir/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<html>Sub index</html>",
		},
		{
			name:           "serve_subdir_index_explicit",
			urlPath:        "/subdir/index.html",
			expectedStatus: http.StatusMovedPermanently, // http.FileServer redirects /subdir/index.html to /subdir/
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.urlPath, nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestDirWithStripPrefix(t *testing.T) {
	t.Parallel()

	// Create test directory
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644))

	tests := []struct {
		name           string
		stripPrefix    string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "strip_static_prefix",
			stripPrefix:    "/static",
			urlPath:        "/static/file.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "content",
		},
		{
			name:           "strip_assets_prefix",
			stripPrefix:    "/assets",
			urlPath:        "/assets/file.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "content",
		},
		{
			name:           "no_prefix_match_returns_404",
			stripPrefix:    "/static",
			urlPath:        "/file.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.Dir[*testContext](tmpDir, static.WithStripPrefix(tt.stripPrefix))
			req := httptest.NewRequest("GET", tt.urlPath, nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestDirWithNotFound(t *testing.T) {
	t.Parallel()

	// Create test directory
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "exists.txt"), []byte("exists"), 0644))

	customNotFoundCalled := false
	customNotFoundHandler := func(w http.ResponseWriter, r *http.Request) error {
		customNotFoundCalled = true
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Custom 404 page"))
		return nil
	}

	handler := static.Dir[*testContext](tmpDir, static.WithNotFound(customNotFoundHandler))

	t.Run("existing_file_uses_normal_handler", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/exists.txt", nil)
		w := httptest.NewRecorder()
		ctx := newTestContext(req, w)

		response := handler(ctx)
		err := response(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "exists", w.Body.String())
		// Custom handler should not be called for existing files
		// Note: customNotFoundCalled may be true from previous test run
		// so we can't reliably check it here
	})

	t.Run("nonexistent_file_uses_custom_handler", func(t *testing.T) {
		customNotFoundCalled = false // Reset for this test

		req := httptest.NewRequest("GET", "/nonexistent.txt", nil)
		w := httptest.NewRecorder()
		ctx := newTestContext(req, w)

		response := handler(ctx)
		err := response(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "Custom 404 page", w.Body.String())
		assert.True(t, customNotFoundCalled)
	})
}

func TestDirStartupValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a file (not a directory)
	testFile := filepath.Join(tmpDir, "not-a-dir.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	t.Run("panic_on_nonexistent_directory", func(t *testing.T) {
		t.Parallel()

		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")

		assert.Panics(t, func() {
			static.Dir[*testContext](nonExistentDir)
		})
	})

	t.Run("panic_on_file_instead_of_directory", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			static.Dir[*testContext](testFile)
		})
	})

	t.Run("panic_on_permission_error", func(t *testing.T) {
		t.Parallel()

		// Skip permission tests as they don't work reliably across all environments
		t.Skip("Permission tests are environment-dependent and not reliable")
	})

	t.Run("clean_path_handling", func(t *testing.T) {
		t.Parallel()

		// Create a valid directory
		validDir := filepath.Join(tmpDir, "valid")
		require.NoError(t, os.Mkdir(validDir, 0755))

		// Path with redundant elements should work (cleaned internally)
		messyPath := filepath.Join(tmpDir, ".", "valid")

		assert.NotPanics(t, func() {
			static.Dir[*testContext](messyPath)
		})
	})
}

func TestDirSecurityNoDirectoryListing(t *testing.T) {
	t.Parallel()

	// Create directory with files but no index.html
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "secret1.txt"), []byte("secret1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "secret2.txt"), []byte("secret2"), 0644))

	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "secret3.txt"), []byte("secret3"), 0644))

	handler := static.Dir[*testContext](tmpDir)

	tests := []struct {
		name    string
		urlPath string
	}{
		{
			name:    "root_directory_listing_blocked",
			urlPath: "/",
		},
		{
			name:    "subdirectory_listing_blocked",
			urlPath: "/subdir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.urlPath, nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, w.Code)

			// Make sure response doesn't contain file names (no directory listing)
			body := w.Body.String()
			assert.NotContains(t, body, "secret1.txt")
			assert.NotContains(t, body, "secret2.txt")
			assert.NotContains(t, body, "secret3.txt")
		})
	}
}

func TestDirPathTraversalPrevention(t *testing.T) {
	t.Parallel()

	// Create directory structure
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	require.NoError(t, os.Mkdir(publicDir, 0755))

	// Secret file outside public directory
	secretFile := filepath.Join(tmpDir, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret content"), 0644))

	// Public file inside public directory
	publicFile := filepath.Join(publicDir, "public.txt")
	require.NoError(t, os.WriteFile(publicFile, []byte("public content"), 0644))

	handler := static.Dir[*testContext](publicDir)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		description    string
	}{
		{
			name:           "normal_file_access",
			urlPath:        "/public.txt",
			expectedStatus: http.StatusOK,
			description:    "Normal files should be accessible",
		},
		{
			name:           "path_traversal_dots",
			urlPath:        "/../secret.txt",
			expectedStatus: http.StatusNotFound,
			description:    "Path traversal with .. should be blocked",
		},
		{
			name:           "path_traversal_encoded_dots",
			urlPath:        "/%2e%2e/secret.txt",
			expectedStatus: http.StatusNotFound,
			description:    "URL-encoded path traversal should be blocked",
		},
		{
			name:           "path_traversal_double_encoded",
			urlPath:        "/%252e%252e/secret.txt",
			expectedStatus: http.StatusNotFound,
			description:    "Double-encoded path traversal should be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.urlPath, nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err, tt.description)
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Ensure secret content is never served
			if tt.expectedStatus == http.StatusNotFound {
				assert.NotContains(t, w.Body.String(), "secret content")
			}
		})
	}
}

func TestDirMethodsAllowed(t *testing.T) {
	t.Parallel()

	// Create test directory and file
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644))

	handler := static.Dir[*testContext](tmpDir)
	methods := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/test.txt", nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)

			// http.FileServer serves content for all methods, but only GET and HEAD return body
			assert.Equal(t, http.StatusOK, w.Code)
			if method == "GET" {
				assert.Equal(t, "content", w.Body.String())
			} else if method == "HEAD" {
				assert.Empty(t, w.Body.String())
			}
		})
	}
}

func TestDirCombinedOptions(t *testing.T) {
	t.Parallel()

	// Create test directory
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("file content"), 0644))

	customNotFoundHandler := func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Custom not found with prefix"))
		return nil
	}

	handler := static.Dir[*testContext](tmpDir,
		static.WithStripPrefix("/static"),
		static.WithNotFound(customNotFoundHandler),
	)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "existing_file_with_prefix",
			urlPath:        "/static/file.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "file content",
		},
		{
			name:           "nonexistent_file_with_prefix_uses_custom_404",
			urlPath:        "/static/missing.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Custom not found with prefix",
		},
		{
			name:           "file_without_prefix_uses_default_404",
			urlPath:        "/file.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.urlPath, nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}
