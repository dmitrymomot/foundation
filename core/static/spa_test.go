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

func TestSPA(t *testing.T) {
	t.Parallel()

	// Create SPA directory structure
	tmpDir := t.TempDir()

	// Create index.html
	indexContent := `<!DOCTYPE html><html><head><title>SPA</title></head><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	// Create static assets
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("// SPA app code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "styles.css"), []byte("body { font-family: sans-serif; }"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "favicon.ico"), []byte("favicon data"), 0644))

	// Create assets subdirectory
	assetsDir := filepath.Join(tmpDir, "assets")
	require.NoError(t, os.Mkdir(assetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "logo.png"), []byte("PNG logo"), 0644))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
		checkFallback  bool
	}{
		{
			name:           "serve_index_html_explicitly",
			urlPath:        "/index.html",
			expectedStatus: http.StatusMovedPermanently, // http.ServeFile redirects index.html to /
			expectedBody:   "",
			checkFallback:  false,
		},
		{
			name:           "serve_static_js_file",
			urlPath:        "/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "// SPA app code",
			checkFallback:  false,
		},
		{
			name:           "serve_static_css_file",
			urlPath:        "/styles.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { font-family: sans-serif; }",
			checkFallback:  false,
		},
		{
			name:           "serve_favicon",
			urlPath:        "/favicon.ico",
			expectedStatus: http.StatusOK,
			expectedBody:   "favicon data",
			checkFallback:  false,
		},
		{
			name:           "serve_asset_from_subdirectory",
			urlPath:        "/assets/logo.png",
			expectedStatus: http.StatusOK,
			expectedBody:   "PNG logo",
			checkFallback:  false,
		},
		{
			name:           "fallback_to_index_for_client_route",
			urlPath:        "/dashboard",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			checkFallback:  true,
		},
		{
			name:           "fallback_to_index_for_nested_client_route",
			urlPath:        "/users/123/profile",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			checkFallback:  true,
		},
		{
			name:           "fallback_to_index_for_root",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			checkFallback:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := static.SPA[*testContext](tmpDir)
			req := httptest.NewRequest("GET", tt.urlPath, nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())

			// Check content type for HTML responses
			if tt.checkFallback {
				contentType := w.Header().Get("Content-Type")
				assert.Contains(t, contentType, "text/html")
			}
		})
	}
}

func TestSPAWithCustomIndex(t *testing.T) {
	t.Parallel()

	// Create SPA directory with custom index file
	tmpDir := t.TempDir()

	customIndexContent := `<!DOCTYPE html><html><head><title>Custom SPA</title></head><body><div id="custom-app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.html"), []byte(customIndexContent), 0644))

	handler := static.SPA[*testContext](tmpDir, static.WithSPAIndex("main.html"))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_custom_index_explicitly",
			urlPath:        "/main.html",
			expectedStatus: http.StatusOK,
			expectedBody:   customIndexContent,
		},
		{
			name:           "fallback_to_custom_index",
			urlPath:        "/dashboard",
			expectedStatus: http.StatusOK,
			expectedBody:   customIndexContent,
		},
		{
			name:           "fallback_to_custom_index_for_root",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   customIndexContent,
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

func TestSPAWithNotFoundPage(t *testing.T) {
	t.Parallel()

	// Create SPA directory with custom 404 page
	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	notFoundContent := `<!DOCTYPE html><html><body><h1>Custom 404 Page</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "404.html"), []byte(notFoundContent), 0644))

	handler := static.SPA[*testContext](tmpDir, static.WithNotFoundPage("404.html"))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_index_normally",
			urlPath:        "/index.html",
			expectedStatus: http.StatusMovedPermanently, // http.ServeFile redirects index.html to /
			expectedBody:   "",
		},
		{
			name:           "serve_custom_404_explicitly",
			urlPath:        "/404.html",
			expectedStatus: http.StatusOK,
			expectedBody:   notFoundContent,
		},
		{
			name:           "serve_custom_404_for_missing_route_when_404_page_configured",
			urlPath:        "/dashboard",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
		},
		{
			name:           "serve_custom_404_for_missing_file",
			urlPath:        "/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
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

func TestSPAWithExcludePaths(t *testing.T) {
	t.Parallel()

	// Create SPA directory
	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	// Test default exclude paths
	handler := static.SPA[*testContext](tmpDir)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
		description    string
	}{
		{
			name:           "exclude_api_path",
			urlPath:        "/api/users",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			description:    "API paths should be excluded from SPA fallback",
		},
		{
			name:           "exclude_ws_path",
			urlPath:        "/ws/chat",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			description:    "WebSocket paths should be excluded from SPA fallback",
		},
		{
			name:           "allow_app_route",
			urlPath:        "/dashboard",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			description:    "App routes should fall back to index",
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
			assert.Equal(t, tt.expectedBody, w.Body.String(), tt.description)
		})
	}
}

func TestSPAWithCustomExcludePaths(t *testing.T) {
	t.Parallel()

	// Create SPA directory
	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	handler := static.SPA[*testContext](tmpDir, static.WithExcludePaths("/api", "/admin", "/health"))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "exclude_custom_api_path",
			urlPath:        "/api/v1/users",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "exclude_custom_admin_path",
			urlPath:        "/admin/dashboard",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "exclude_custom_health_path",
			urlPath:        "/health",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "allow_websocket_path_when_not_excluded",
			urlPath:        "/ws/chat",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "allow_app_route",
			urlPath:        "/dashboard",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
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

func TestSPAWithStripPrefix(t *testing.T) {
	t.Parallel()

	// Create SPA directory
	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("// app"), 0644))

	handler := static.SPA[*testContext](tmpDir, static.WithSPAStripPrefix("/app"))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_static_file_with_prefix",
			urlPath:        "/app/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "// app",
		},
		{
			name:           "fallback_to_index_with_prefix",
			urlPath:        "/app/dashboard",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "serve_index_with_prefix",
			urlPath:        "/app/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "no_prefix_falls_back_to_index",
			urlPath:        "/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent, // Without prefix, SPA serves index.html for non-excluded paths
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

func TestSPAWithDirectoryIndex(t *testing.T) {
	t.Parallel()

	// Create directory structure with index files
	tmpDir := t.TempDir()

	rootIndexContent := `<!DOCTYPE html><html><body>Root SPA</body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(rootIndexContent), 0644))

	// Create subdirectory with its own index.html
	subDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.Mkdir(subDir, 0755))
	subIndexContent := `<!DOCTYPE html><html><body>Docs Index</body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "index.html"), []byte(subIndexContent), 0644))

	handler := static.SPA[*testContext](tmpDir)

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
			expectedBody:   rootIndexContent,
		},
		{
			name:           "serve_subdirectory_index",
			urlPath:        "/docs/",
			expectedStatus: http.StatusOK,
			expectedBody:   subIndexContent,
		},
		{
			name:           "serve_subdirectory_index_explicit",
			urlPath:        "/docs/index.html",
			expectedStatus: http.StatusMovedPermanently, // http.ServeFile redirects /docs/index.html to /docs/
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

func TestSPAStartupValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a file (not a directory)
	testFile := filepath.Join(tmpDir, "not-a-dir.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	t.Run("panic_on_nonexistent_directory", func(t *testing.T) {
		t.Parallel()

		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")

		assert.Panics(t, func() {
			static.SPA[*testContext](nonExistentDir)
		})
	})

	t.Run("panic_on_file_instead_of_directory", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			static.SPA[*testContext](testFile)
		})
	})

	t.Run("panic_on_missing_index_file", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid")
		require.NoError(t, os.Mkdir(validDir, 0755))

		assert.Panics(t, func() {
			static.SPA[*testContext](validDir)
		})
	})

	t.Run("panic_on_missing_custom_index_file", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid2")
		require.NoError(t, os.Mkdir(validDir, 0755))

		assert.Panics(t, func() {
			static.SPA[*testContext](validDir, static.WithSPAIndex("app.html"))
		})
	})

	t.Run("panic_on_missing_404_file", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid3")
		require.NoError(t, os.Mkdir(validDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(validDir, "index.html"), []byte("index"), 0644))

		assert.Panics(t, func() {
			static.SPA[*testContext](validDir, static.WithNotFoundPage("missing.html"))
		})
	})

	t.Run("valid_setup_works", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid4")
		require.NoError(t, os.Mkdir(validDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(validDir, "index.html"), []byte("index"), 0644))

		assert.NotPanics(t, func() {
			static.SPA[*testContext](validDir)
		})
	})

	t.Run("clean_path_handling", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid5")
		require.NoError(t, os.Mkdir(validDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(validDir, "index.html"), []byte("index"), 0644))

		// Path with redundant elements should work (cleaned internally)
		messyPath := filepath.Join(tmpDir, ".", "valid5")

		assert.NotPanics(t, func() {
			static.SPA[*testContext](messyPath)
		})
	})

	t.Run("panic_on_permission_error", func(t *testing.T) {
		t.Parallel()

		// Skip permission tests as they don't work reliably across all environments
		t.Skip("Permission tests are environment-dependent and not reliable")

		restrictedDir := filepath.Join(tmpDir, "restricted")
		require.NoError(t, os.Mkdir(restrictedDir, 0000))
		defer os.Chmod(restrictedDir, 0755) // Clean up

		assert.Panics(t, func() {
			static.SPA[*testContext](restrictedDir)
		})
	})
}

func TestSPAMethodsAllowed(t *testing.T) {
	t.Parallel()

	// Create SPA directory
	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("// app"), 0644))

	handler := static.SPA[*testContext](tmpDir)
	methods := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("method_"+method+"_for_static_file", func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/app.js", nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)

			// http.ServeFile serves content for all methods, but only GET and HEAD return body
			assert.Equal(t, http.StatusOK, w.Code)
			if method == "GET" {
				assert.Equal(t, "// app", w.Body.String())
			} else if method == "HEAD" {
				assert.Empty(t, w.Body.String())
			}
		})

		t.Run("method_"+method+"_for_spa_route", func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(method, "/dashboard", nil)
			w := httptest.NewRecorder()
			ctx := newTestContext(req, w)

			response := handler(ctx)
			err := response(w, req)

			assert.NoError(t, err)

			// SPA fallback also uses http.ServeFile for index.html
			assert.Equal(t, http.StatusOK, w.Code)
			if method == "GET" {
				assert.Equal(t, indexContent, w.Body.String())
			} else if method == "HEAD" {
				assert.Empty(t, w.Body.String())
			}
		})
	}
}

func TestSPACombinedOptions(t *testing.T) {
	t.Parallel()

	// Create SPA directory
	tmpDir := t.TempDir()

	customIndexContent := `<!DOCTYPE html><html><body>Custom SPA</body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.html"), []byte(customIndexContent), 0644))

	notFoundContent := `<!DOCTYPE html><html><body>Custom 404</body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "error.html"), []byte(notFoundContent), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bundle.js"), []byte("// bundle"), 0644))

	handler := static.SPA[*testContext](tmpDir,
		static.WithSPAIndex("app.html"),
		static.WithNotFoundPage("error.html"),
		static.WithExcludePaths("/api", "/health"),
		static.WithSPAStripPrefix("/webapp"),
	)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_static_file_with_all_options",
			urlPath:        "/webapp/bundle.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "// bundle",
		},
		{
			name:           "serve_custom_404_for_route_when_404_page_configured",
			urlPath:        "/webapp/dashboard",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent, // With custom 404 page, routes get 404 instead of index fallback
		},
		{
			name:           "exclude_api_with_prefix",
			urlPath:        "/webapp/api/users",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "serve_custom_404_with_prefix",
			urlPath:        "/webapp/missing.txt",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
		},
		{
			name:           "no_prefix_serves_existing_file",
			urlPath:        "/bundle.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "// bundle", // File exists in root, so it's served directly
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

func TestSPAContentTypeHeaders(t *testing.T) {
	t.Parallel()

	// Create SPA directory with various file types
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html>SPA</html>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("// js"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "styles.css"), []byte("body{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "data.json"), []byte(`{"test":true}`), 0644))

	handler := static.SPA[*testContext](tmpDir)

	tests := []struct {
		name                string
		urlPath             string
		expectedContentType string
	}{
		{
			name:                "html_content_type_for_root",
			urlPath:             "/", // Use root path instead since index.html redirects
			expectedContentType: "text/html",
		},
		{
			name:                "js_content_type",
			urlPath:             "/app.js",
			expectedContentType: "text/javascript",
		},
		{
			name:                "css_content_type",
			urlPath:             "/styles.css",
			expectedContentType: "text/css",
		},
		{
			name:                "json_content_type",
			urlPath:             "/data.json",
			expectedContentType: "application/json",
		},
		{
			name:                "html_content_type_for_fallback",
			urlPath:             "/dashboard",
			expectedContentType: "text/html",
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
