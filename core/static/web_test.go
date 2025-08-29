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

// TestSPA tests the basic SPA handler functionality
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
		{
			name:           "spa_does_not_serve_subdirectory_index",
			urlPath:        "/assets/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent, // Should fallback to root index, not serve directory
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

// TestSPAWithCustomIndex tests SPA with custom index file
func TestSPAWithCustomIndex(t *testing.T) {
	t.Parallel()

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

// TestSPAWithExcludePaths tests SPA with excluded paths
func TestSPAWithExcludePaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	// Test default exclude paths
	handler := static.SPA[*testContext](tmpDir)

	tests := []struct {
		name           string
		urlPath        string
		expectedError  bool
		expectedStatus int
		expectedBody   string
		description    string
	}{
		{
			name:           "exclude_api_path",
			urlPath:        "/api/users",
			expectedError:  true, // Should return response.ErrNotFound
			expectedStatus: http.StatusNotFound,
			description:    "API paths should be excluded from SPA fallback",
		},
		{
			name:           "exclude_ws_path",
			urlPath:        "/ws/chat",
			expectedError:  true, // Should return response.ErrNotFound
			expectedStatus: http.StatusNotFound,
			description:    "WebSocket paths should be excluded from SPA fallback",
		},
		{
			name:           "allow_app_route",
			urlPath:        "/dashboard",
			expectedError:  false,
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			description:    "App routes should fall back to index",
		},
		{
			name:           "allow_api_docs",
			urlPath:        "/api-docs",
			expectedError:  false,
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			description:    "Path segment matching: /api-docs should not be excluded by /api",
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

			if tt.expectedError {
				assert.Error(t, err, tt.description)
				assert.Equal(t, "Not Found", err.Error())
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
				assert.Equal(t, tt.expectedBody, w.Body.String(), tt.description)
			}
		})
	}
}

// TestSPAWithCustomExcludePaths tests SPA with custom exclude paths
func TestSPAWithCustomExcludePaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	indexContent := `<!DOCTYPE html><html><body><div id="app"></div></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	handler := static.SPA[*testContext](tmpDir, static.WithSPAExcludePaths("/api", "/admin", "/health"))

	tests := []struct {
		name          string
		urlPath       string
		expectedError bool
		expectedBody  string
	}{
		{
			name:          "exclude_custom_api_path",
			urlPath:       "/api/v1/users",
			expectedError: true,
		},
		{
			name:          "exclude_custom_admin_path",
			urlPath:       "/admin/dashboard",
			expectedError: true,
		},
		{
			name:          "exclude_custom_health_path",
			urlPath:       "/health",
			expectedError: true,
		},
		{
			name:          "allow_websocket_path_when_not_excluded",
			urlPath:       "/ws/chat",
			expectedError: false,
			expectedBody:  indexContent,
		},
		{
			name:          "allow_app_route",
			urlPath:       "/dashboard",
			expectedError: false,
			expectedBody:  indexContent,
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

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, "Not Found", err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

// TestSPAStartupValidation tests SPA startup validation
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

	t.Run("valid_setup_works", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid3")
		require.NoError(t, os.Mkdir(validDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(validDir, "index.html"), []byte("index"), 0644))

		assert.NotPanics(t, func() {
			static.SPA[*testContext](validDir)
		})
	})
}

// TestWebsite tests basic Website handler functionality
func TestWebsite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create website structure
	indexContent := `<!DOCTYPE html><html><body><h1>Home</h1></body></html>`
	aboutContent := `<!DOCTYPE html><html><body><h1>About</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "about.html"), []byte(aboutContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "styles.css"), []byte("body { margin: 0; }"), 0644))

	// Create blog directory with index
	blogDir := filepath.Join(tmpDir, "blog")
	require.NoError(t, os.Mkdir(blogDir, 0755))
	blogIndexContent := `<!DOCTYPE html><html><body><h1>Blog</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(blogDir, "index.html"), []byte(blogIndexContent), 0644))

	handler := static.Website[*testContext](tmpDir)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
		followRedirect bool
	}{
		{
			name:           "serve_root_index",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "serve_html_file_directly",
			urlPath:        "/about.html",
			expectedStatus: http.StatusOK,
			expectedBody:   aboutContent,
		},
		{
			name:           "serve_css_file",
			urlPath:        "/styles.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { margin: 0; }",
		},
		{
			name:           "redirect_directory_without_slash",
			urlPath:        "/blog",
			expectedStatus: http.StatusMovedPermanently,
			expectedBody:   "",
		},
		{
			name:           "serve_directory_with_slash",
			urlPath:        "/blog/",
			expectedStatus: http.StatusOK,
			expectedBody:   blogIndexContent,
		},
		{
			name:           "return_404_for_missing_file",
			urlPath:        "/missing.html",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
		},
		{
			name:           "return_404_for_missing_route",
			urlPath:        "/dashboard",
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
			if tt.expectedStatus == http.StatusMovedPermanently {
				location := w.Header().Get("Location")
				assert.Equal(t, tt.urlPath+"/", location)
			} else {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

// TestWebsiteIndexRedirects tests SEO-friendly index.html redirects
func TestWebsiteIndexRedirects(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create website structure
	indexContent := `<!DOCTYPE html><html><body><h1>Home</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	// Create blog directory with index
	blogDir := filepath.Join(tmpDir, "blog")
	require.NoError(t, os.Mkdir(blogDir, 0755))
	blogIndexContent := `<!DOCTYPE html><html><body><h1>Blog</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(blogDir, "index.html"), []byte(blogIndexContent), 0644))

	handler := static.Website[*testContext](tmpDir)

	tests := []struct {
		name             string
		urlPath          string
		expectedStatus   int
		expectedLocation string
		description      string
	}{
		{
			name:             "redirect_root_index_html",
			urlPath:          "/index.html",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/",
			description:      "SEO: /index.html should redirect to /",
		},
		{
			name:             "redirect_blog_index_html",
			urlPath:          "/blog/index.html",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/blog/",
			description:      "SEO: /blog/index.html should redirect to /blog/",
		},
		{
			name:             "redirect_blog_without_slash",
			urlPath:          "/blog",
			expectedStatus:   http.StatusMovedPermanently,
			expectedLocation: "/blog/",
			description:      "SEO: /blog should redirect to /blog/",
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
			assert.Equal(t, tt.expectedLocation, w.Header().Get("Location"), tt.description)
		})
	}
}

// TestWebsiteWithCustom404 tests Website handler with custom 404 page
func TestWebsiteWithCustom404(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><h1>Home</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	notFoundContent := `<!DOCTYPE html><html><body><h1>Custom 404 - Page Not Found</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "404.html"), []byte(notFoundContent), 0644))

	handler := static.Website[*testContext](tmpDir, static.WithNotFoundFile("404.html"))

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_index_normally",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
		},
		{
			name:           "serve_custom_404_for_missing_file",
			urlPath:        "/missing.html",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
		},
		{
			name:           "serve_custom_404_for_missing_route",
			urlPath:        "/dashboard",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
		},
		{
			name:           "serve_404_file_directly",
			urlPath:        "/404.html",
			expectedStatus: http.StatusOK,
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

// TestWebsiteWithExcludePaths tests Website handler with excluded paths
func TestWebsiteWithExcludePaths(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	indexContent := `<!DOCTYPE html><html><body><h1>Home</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644))

	notFoundContent := `<!DOCTYPE html><html><body><h1>Custom 404</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "404.html"), []byte(notFoundContent), 0644))

	handler := static.Website[*testContext](tmpDir,
		static.WithNotFoundFile("404.html"),
		static.WithWebsiteExcludePaths("/api", "/admin"),
	)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
		description    string
	}{
		{
			name:           "exclude_api_path_with_custom_404",
			urlPath:        "/api/users",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
			description:    "Excluded paths should serve custom 404",
		},
		{
			name:           "exclude_admin_path_with_custom_404",
			urlPath:        "/admin",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
			description:    "Excluded paths should serve custom 404",
		},
		{
			name:           "serve_index_normally",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			description:    "Non-excluded paths work normally",
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

// TestWebsiteStartupValidation tests Website startup validation
func TestWebsiteStartupValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("panic_on_nonexistent_directory", func(t *testing.T) {
		t.Parallel()

		nonExistentDir := filepath.Join(tmpDir, "does-not-exist")
		assert.Panics(t, func() {
			static.Website[*testContext](nonExistentDir)
		})
	})

	t.Run("panic_on_missing_404_file", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid")
		require.NoError(t, os.Mkdir(validDir, 0755))

		assert.Panics(t, func() {
			static.Website[*testContext](validDir, static.WithNotFoundFile("missing.html"))
		})
	})

	t.Run("valid_setup_without_index", func(t *testing.T) {
		t.Parallel()

		validDir := filepath.Join(tmpDir, "valid2")
		require.NoError(t, os.Mkdir(validDir, 0755))
		// Note: Website doesn't require index.html in root

		assert.NotPanics(t, func() {
			static.Website[*testContext](validDir)
		})
	})
}

// TestWebsiteCombinedOptions tests Website handler with all options combined
func TestWebsiteCombinedOptions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	homeContent := `<!DOCTYPE html><html><body><h1>Home</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "home.html"), []byte(homeContent), 0644))

	notFoundContent := `<!DOCTYPE html><html><body><h1>Error 404</h1></body></html>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "error.html"), []byte(notFoundContent), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte("body { color: red; }"), 0644))

	handler := static.Website[*testContext](tmpDir,
		static.WithIndexFile("home.html"),
		static.WithNotFoundFile("error.html"),
		static.WithWebsiteExcludePaths("/api", "/health"),
	)

	tests := []struct {
		name           string
		urlPath        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "serve_custom_index",
			urlPath:        "/",
			expectedStatus: http.StatusOK,
			expectedBody:   homeContent,
		},
		{
			name:           "redirect_custom_index_html",
			urlPath:        "/home.html",
			expectedStatus: http.StatusMovedPermanently,
		},
		{
			name:           "serve_css",
			urlPath:        "/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: red; }",
		},
		{
			name:           "exclude_api_with_custom_404",
			urlPath:        "/api/users",
			expectedStatus: http.StatusNotFound,
			expectedBody:   notFoundContent,
		},
		{
			name:           "serve_custom_404_for_missing",
			urlPath:        "/missing.html",
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
			if tt.expectedStatus == http.StatusMovedPermanently {
				assert.Equal(t, "/", w.Header().Get("Location"))
			} else {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}
