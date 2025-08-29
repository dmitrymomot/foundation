package static_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/foundation/core/static"
)

// testFS would contain embedded files for testing
// var testFS embed.FS - commented out since testdata directory doesn't exist

func TestFS(t *testing.T) {
	t.Parallel()

	// Create in-memory filesystem for testing
	fsys := fstest.MapFS{
		"index.html":      {Data: []byte("<html>Index page</html>"), Mode: 0644},
		"styles.css":      {Data: []byte("body { color: red; }"), Mode: 0644},
		"script.js":       {Data: []byte("console.log('test');"), Mode: 0644},
		"data.json":       {Data: []byte(`{"key": "value"}`), Mode: 0644},
		"images/logo.png": {Data: []byte("PNG data"), Mode: 0644},
		"docs/readme.txt": {Data: []byte("Documentation"), Mode: 0644},
	}

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
		checkHeaders   bool
	}{
		{
			name:           "serve_html_file",
			urlPath:        "/index.html",
			expectedStatus: http.StatusMovedPermanently, // http.FileServer redirects index.html to /
			expectedBody:   "",
			checkHeaders:   false,
		},
		{
			name:           "serve_css_file",
			urlPath:        "/styles.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: red; }",
			checkHeaders:   true,
		},
		{
			name:           "serve_js_file",
			urlPath:        "/script.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('test');",
			checkHeaders:   true,
		},
		{
			name:           "serve_json_file",
			urlPath:        "/data.json",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"key": "value"}`,
			checkHeaders:   true,
		},
		{
			name:           "serve_file_from_subdirectory",
			urlPath:        "/images/logo.png",
			expectedStatus: http.StatusOK,
			expectedBody:   "PNG data",
			checkHeaders:   true,
		},
		{
			name:           "serve_nested_file",
			urlPath:        "/docs/readme.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "Documentation",
			checkHeaders:   true,
		},
		{
			name:           "nonexistent_file_returns_404",
			urlPath:        "/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
		{
			name:           "directory_without_index_returns_404",
			urlPath:        "/images/",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
		{
			name:           "root_directory_serves_index_when_available",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<html>Index page</html>",
			checkHeaders:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.FS[*testContext](fsys)
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

func TestFSWithIndex(t *testing.T) {
	t.Parallel()

	// Create filesystem with index files
	fsys := fstest.MapFS{
		"index.html":        {Data: []byte("<html>Root index</html>"), Mode: 0644},
		"subdir/index.html": {Data: []byte("<html>Sub index</html>"), Mode: 0644},
		"other/file.txt":    {Data: []byte("Other file"), Mode: 0644},
	}

	handler := static.FS[*testContext](fsys)

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
			expectedStatus: http.StatusMovedPermanently, // http.FileServer redirects index.html to /
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
			expectedStatus: http.StatusMovedPermanently, // http.FileServer redirects subdir/index.html to subdir/
			expectedBody:   "",
		},
		{
			name:           "directory_without_index_returns_404",
			urlPath:        "/other/",
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

func TestFSWithStripPrefix(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"app.js":    {Data: []byte("app content"), Mode: 0644},
		"style.css": {Data: []byte("css content"), Mode: 0644},
	}

	tests := []struct {
		name           string
		stripPrefix    string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "strip_assets_prefix",
			stripPrefix:    "/assets",
			urlPath:        "/assets/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "app content",
		},
		{
			name:           "strip_static_prefix",
			stripPrefix:    "/static",
			urlPath:        "/static/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "css content",
		},
		{
			name:           "no_prefix_match_returns_404",
			stripPrefix:    "/assets",
			urlPath:        "/app.js",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.FS[*testContext](fsys, static.WithFSStripPrefix(tt.stripPrefix))
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

func TestFSWithSubFS(t *testing.T) {
	t.Parallel()

	// Create nested filesystem structure
	fsys := fstest.MapFS{
		"root.txt":            {Data: []byte("root file"), Mode: 0644},
		"assets/app.js":       {Data: []byte("app script"), Mode: 0644},
		"assets/style.css":    {Data: []byte("app styles"), Mode: 0644},
		"assets/images/1.png": {Data: []byte("image 1"), Mode: 0644},
		"docs/readme.md":      {Data: []byte("# Readme"), Mode: 0644},
		"docs/guide.txt":      {Data: []byte("Guide content"), Mode: 0644},
	}

	tests := []struct {
		name           string
		subPath        string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_from_assets_subdir",
			subPath:        "assets",
			urlPath:        "/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "app script",
		},
		{
			name:           "serve_css_from_assets_subdir",
			subPath:        "assets",
			urlPath:        "/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "app styles",
		},
		{
			name:           "serve_nested_file_from_assets",
			subPath:        "assets",
			urlPath:        "/images/1.png",
			expectedStatus: http.StatusOK,
			expectedBody:   "image 1",
		},
		{
			name:           "serve_from_docs_subdir",
			subPath:        "docs",
			urlPath:        "/readme.md",
			expectedStatus: http.StatusOK,
			expectedBody:   "# Readme",
		},
		{
			name:           "root_file_not_accessible_from_subdir",
			subPath:        "assets",
			urlPath:        "/root.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "other_subdir_not_accessible",
			subPath:        "assets",
			urlPath:        "/docs/readme.md",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.FS[*testContext](fsys, static.WithSubFS(tt.subPath))
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

func TestFSStartupValidation(t *testing.T) {
	t.Parallel()

	t.Run("panic_on_invalid_sub_path", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"file.txt": {Data: []byte("content"), Mode: 0644},
		}

		assert.Panics(t, func() {
			static.FS[*testContext](fsys, static.WithSubFS("nonexistent"))
		})
	})

	t.Run("panic_on_inaccessible_filesystem", func(t *testing.T) {
		t.Parallel()

		// Create a filesystem that fails on Open(".")
		invalidFS := &failingFS{}

		assert.Panics(t, func() {
			static.FS[*testContext](invalidFS)
		})
	})

	t.Run("valid_sub_path_works", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"subdir/file.txt": {Data: []byte("content"), Mode: 0644},
		}

		assert.NotPanics(t, func() {
			static.FS[*testContext](fsys, static.WithSubFS("subdir"))
		})
	})

	t.Run("empty_filesystem_works", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{}

		assert.NotPanics(t, func() {
			static.FS[*testContext](fsys)
		})
	})
}

// failingFS is a test filesystem that always fails
type failingFS struct{}

func (f *failingFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrInvalid
}

func TestFSSecurityNoDirectoryListing(t *testing.T) {
	t.Parallel()

	// Create filesystem with files but no index.html in directories
	fsys := fstest.MapFS{
		"secret1.txt":        {Data: []byte("secret1"), Mode: 0644},
		"secret2.txt":        {Data: []byte("secret2"), Mode: 0644},
		"subdir/secret3.txt": {Data: []byte("secret3"), Mode: 0644},
		"subdir/secret4.txt": {Data: []byte("secret4"), Mode: 0644},
	}

	handler := static.FS[*testContext](fsys)

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
			assert.NotContains(t, body, "secret4.txt")
		})
	}
}

func TestFSMethodsAllowed(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"test.txt": {Data: []byte("content"), Mode: 0644},
	}

	handler := static.FS[*testContext](fsys)
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

func TestFSCombinedOptions(t *testing.T) {
	t.Parallel()

	// Create nested filesystem
	fsys := fstest.MapFS{
		"root.txt":         {Data: []byte("root file"), Mode: 0644},
		"assets/app.js":    {Data: []byte("app script"), Mode: 0644},
		"assets/style.css": {Data: []byte("app styles"), Mode: 0644},
	}

	handler := static.FS[*testContext](fsys,
		static.WithSubFS("assets"),
		static.WithFSStripPrefix("/static"),
	)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_file_with_both_options",
			urlPath:        "/static/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "app script",
		},
		{
			name:           "serve_css_with_both_options",
			urlPath:        "/static/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "app styles",
		},
		{
			name:           "root_file_not_accessible_due_to_subfs",
			urlPath:        "/static/root.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "no_prefix_returns_404",
			urlPath:        "/app.js",
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

func TestFSWithEmbedFS(t *testing.T) {
	t.Parallel()

	// Skip this test as we don't have embedded testdata files
	// In a real project, you would have:
	// //go:embed testdata/*
	// var testFS embed.FS
	t.Skip("Skipping embed.FS test - no testdata directory embedded")
}

func TestFSEmptyFileHandling(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"empty.txt": {Data: []byte{}, Mode: 0644},
		"small.txt": {Data: []byte("x"), Mode: 0644},
	}

	handler := static.FS[*testContext](fsys)

	tests := []struct {
		name         string
		urlPath      string
		expectedBody string
	}{
		{
			name:         "serve_empty_file",
			urlPath:      "/empty.txt",
			expectedBody: "",
		},
		{
			name:         "serve_single_char_file",
			urlPath:      "/small.txt",
			expectedBody: "x",
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
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestFSContentTypeDetection(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"test.html": {Data: []byte("<html>test</html>"), Mode: 0644},
		"test.css":  {Data: []byte("body{}"), Mode: 0644},
		"test.js":   {Data: []byte("console.log()"), Mode: 0644},
		"test.json": {Data: []byte(`{"test": true}`), Mode: 0644},
		"test.xml":  {Data: []byte(`<?xml version="1.0"?><root/>`), Mode: 0644},
		"test.txt":  {Data: []byte("plain text"), Mode: 0644},
	}

	handler := static.FS[*testContext](fsys)

	tests := []struct {
		name                string
		urlPath             string
		expectedContentType string
	}{
		{
			name:                "html_content_type",
			urlPath:             "/test.html",
			expectedContentType: "text/html",
		},
		{
			name:                "css_content_type",
			urlPath:             "/test.css",
			expectedContentType: "text/css",
		},
		{
			name:                "js_content_type",
			urlPath:             "/test.js",
			expectedContentType: "text/javascript",
		},
		{
			name:                "json_content_type",
			urlPath:             "/test.json",
			expectedContentType: "application/json",
		},
		{
			name:                "text_content_type",
			urlPath:             "/test.txt",
			expectedContentType: "text/plain",
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
			assert.Equal(t, http.StatusOK, w.Code)

			contentType := w.Header().Get("Content-Type")
			assert.Contains(t, contentType, tt.expectedContentType)
		})
	}
}
