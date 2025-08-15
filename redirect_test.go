package gokit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
)

func TestRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		expectedURL string // What Go's http.Redirect actually sets
	}{
		{
			name:        "simple_redirect",
			url:         "/new-location",
			expectedURL: "/new-location",
		},
		{
			name:        "external_redirect",
			url:         "https://example.com",
			expectedURL: "https://example.com",
		},
		{
			name:        "relative_redirect",
			url:         "../parent",
			expectedURL: "/parent", // Go converts relative paths to absolute
		},
		{
			name:        "root_redirect",
			url:         "/",
			expectedURL: "/",
		},
		{
			name:        "query_params_redirect",
			url:         "/search?q=golang",
			expectedURL: "/search?q=golang",
		},
		{
			name:        "fragment_redirect",
			url:         "/page#section1",
			expectedURL: "/page#section1",
		},
		{
			name:        "empty_url",
			url:         "",
			expectedURL: "/", // Go converts empty URL to root
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.Redirect(tt.url)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, tt.expectedURL, w.Header().Get("Location"))
		})
	}
}

func TestRedirectPermanent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "permanent_redirect",
			url:  "/new-permanent-location",
		},
		{
			name: "permanent_external_redirect",
			url:  "https://newdomain.com",
		},
		{
			name: "permanent_root_redirect",
			url:  "/",
		},
		{
			name: "permanent_with_params",
			url:  "/api/v2/users?active=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.RedirectPermanent(tt.url)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusMovedPermanently, w.Code)
			assert.Equal(t, tt.url, w.Header().Get("Location"))
		})
	}
}

func TestRedirectSeeOther(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "see_other_redirect",
			url:  "/success",
		},
		{
			name: "post_redirect_get",
			url:  "/thank-you",
		},
		{
			name: "see_other_external",
			url:  "https://example.com/success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.RedirectSeeOther(tt.url)
			req := httptest.NewRequest("POST", "/form-submit", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusSeeOther, w.Code)
			assert.Equal(t, tt.url, w.Header().Get("Location"))
		})
	}
}

func TestRedirectTemporary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "temporary_redirect",
			url:  "/maintenance",
		},
		{
			name: "temporary_external",
			url:  "https://backup.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.RedirectTemporary(tt.url)
			req := httptest.NewRequest("POST", "/api/data", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
			assert.Equal(t, tt.url, w.Header().Get("Location"))
		})
	}
}

func TestRedirectPermanentPreserve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "permanent_preserve_redirect",
			url:  "/api/v2/data",
		},
		{
			name: "permanent_preserve_external",
			url:  "https://newapi.example.com/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.RedirectPermanentPreserve(tt.url)
			req := httptest.NewRequest("POST", "/api/v1/data", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusPermanentRedirect, w.Code)
			assert.Equal(t, tt.url, w.Header().Get("Location"))
		})
	}
}

func TestRedirectWithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		url            string
		status         int
		expectedStatus int
	}{
		{
			name:           "custom_301",
			url:            "/moved",
			status:         http.StatusMovedPermanently,
			expectedStatus: http.StatusMovedPermanently,
		},
		{
			name:           "custom_302",
			url:            "/found",
			status:         http.StatusFound,
			expectedStatus: http.StatusFound,
		},
		{
			name:           "custom_303",
			url:            "/see-other",
			status:         http.StatusSeeOther,
			expectedStatus: http.StatusSeeOther,
		},
		{
			name:           "custom_307",
			url:            "/temporary",
			status:         http.StatusTemporaryRedirect,
			expectedStatus: http.StatusTemporaryRedirect,
		},
		{
			name:           "custom_308",
			url:            "/permanent",
			status:         http.StatusPermanentRedirect,
			expectedStatus: http.StatusPermanentRedirect,
		},
		{
			name:           "invalid_status_below_300",
			url:            "/test",
			status:         200,
			expectedStatus: http.StatusFound, // Should default to 302
		},
		{
			name:           "invalid_status_above_399",
			url:            "/test",
			status:         400,
			expectedStatus: http.StatusFound, // Should default to 302
		},
		{
			name:           "zero_status",
			url:            "/test",
			status:         0,
			expectedStatus: http.StatusFound, // Should default to 302
		},
		{
			name:           "negative_status",
			url:            "/test",
			status:         -1,
			expectedStatus: http.StatusFound, // Should default to 302
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.RedirectWithStatus(tt.url, tt.status)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.url, w.Header().Get("Location"))
		})
	}
}

func TestRedirectMethodPreservation(t *testing.T) {
	t.Parallel()

	// Test that different redirect types behave correctly with different HTTP methods
	tests := []struct {
		name           string
		redirectType   string
		method         string
		expectedStatus int
	}{
		{
			name:           "302_with_post",
			redirectType:   "found",
			method:         "POST",
			expectedStatus: http.StatusFound,
		},
		{
			name:           "303_with_post",
			redirectType:   "see_other",
			method:         "POST",
			expectedStatus: http.StatusSeeOther,
		},
		{
			name:           "307_with_post",
			redirectType:   "temporary",
			method:         "POST",
			expectedStatus: http.StatusTemporaryRedirect,
		},
		{
			name:           "308_with_post",
			redirectType:   "permanent_preserve",
			method:         "POST",
			expectedStatus: http.StatusPermanentRedirect,
		},
		{
			name:           "301_with_get",
			redirectType:   "permanent",
			method:         "GET",
			expectedStatus: http.StatusMovedPermanently,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var response gokit.Response
			switch tt.redirectType {
			case "found":
				response = gokit.Redirect("/new-location")
			case "see_other":
				response = gokit.RedirectSeeOther("/new-location")
			case "temporary":
				response = gokit.RedirectTemporary("/new-location")
			case "permanent_preserve":
				response = gokit.RedirectPermanentPreserve("/new-location")
			case "permanent":
				response = gokit.RedirectPermanent("/new-location")
			}

			req := httptest.NewRequest(tt.method, "/old-location", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "/new-location", w.Header().Get("Location"))
		})
	}
}

func TestRedirectWithExistingHeaders(t *testing.T) {
	t.Parallel()

	response := gokit.Redirect("/new-location")
	req := httptest.NewRequest("GET", "/old-location", nil)
	w := httptest.NewRecorder()

	// Set existing headers before redirect
	w.Header().Set("X-Custom-Header", "custom-value")
	w.Header().Set("Cache-Control", "no-cache")

	err := response.Render(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/new-location", w.Header().Get("Location"))
	// Custom headers should be preserved
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
}

func TestRedirectURLEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		expectedURL string // What Go's http.Redirect might produce
	}{
		{
			name:        "url_with_spaces",
			url:         "/path with spaces",
			expectedURL: "/path%20with%20spaces", // Go may encode spaces
		},
		{
			name:        "url_with_special_chars",
			url:         "/path?param=value with spaces&other=test",
			expectedURL: "/path?param=value%20with%20spaces&other=test", // Go may encode spaces in query
		},
		{
			name:        "url_with_unicode",
			url:         "/пуць/файл",
			expectedURL: "/%D0%BF%D1%83%D1%86%D1%8C/%D1%84%D0%B0%D0%B9%D0%BB", // Go encodes Unicode
		},
		{
			name:        "url_with_encoded_chars",
			url:         "/path?param=%20encoded%20spaces",
			expectedURL: "/path?param=%20encoded%20spaces", // Already encoded, should pass through
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.Redirect(tt.url)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusFound, w.Code)
			// Check if the URL matches expected encoded version
			location := w.Header().Get("Location")
			// Go's http.Redirect may encode the URL, so check both original and expected
			// For Unicode, also check lowercase hex encoding
			expectedLowercase := strings.ToLower(tt.expectedURL)
			if location != tt.url && location != tt.expectedURL && location != expectedLowercase {
				t.Errorf("Expected Location header to be %q, %q, or %q, got %q", tt.url, tt.expectedURL, expectedLowercase, location)
			}
		})
	}
}

func TestRedirectEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("very_long_url", func(t *testing.T) {
		t.Parallel()

		// Test with a very long URL
		longURL := "/very/long/path/" + string(make([]byte, 1000))
		for i := range longURL[17:] { // Skip the prefix part
			longURL = longURL[:17] + string(rune('a'+i%26)) + longURL[18:]
		}

		response := gokit.Redirect(longURL)
		req := httptest.NewRequest("GET", "/old", nil)
		w := httptest.NewRecorder()

		err := response.Render(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, longURL, w.Header().Get("Location"))
	})

	t.Run("redirect_to_same_path", func(t *testing.T) {
		t.Parallel()

		response := gokit.Redirect("/same-path")
		req := httptest.NewRequest("GET", "/same-path", nil)
		w := httptest.NewRecorder()

		err := response.Render(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/same-path", w.Header().Get("Location"))
	})

	t.Run("multiple_redirects_simulation", func(t *testing.T) {
		t.Parallel()

		// Simulate multiple redirects in sequence
		urls := []string{"/step1", "/step2", "/step3", "/final"}

		for i, url := range urls {
			response := gokit.Redirect(url)
			req := httptest.NewRequest("GET", "/start", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err, "Redirect step %d failed", i+1)
			assert.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, url, w.Header().Get("Location"))
		}
	})
}
