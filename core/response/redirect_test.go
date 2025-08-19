package response_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
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

			response := response.Redirect(tt.url)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

			response := response.RedirectPermanent(tt.url)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

			response := response.RedirectSeeOther(tt.url)
			req := httptest.NewRequest("POST", "/form-submit", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

			response := response.RedirectTemporary(tt.url)
			req := httptest.NewRequest("POST", "/api/data", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

			response := response.RedirectPermanentPreserve(tt.url)
			req := httptest.NewRequest("POST", "/api/v1/data", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

			response := response.RedirectWithStatus(tt.url, tt.status)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

			var resp handler.Response
			switch tt.redirectType {
			case "found":
				resp = response.Redirect("/new-location")
			case "see_other":
				resp = response.RedirectSeeOther("/new-location")
			case "temporary":
				resp = response.RedirectTemporary("/new-location")
			case "permanent_preserve":
				resp = response.RedirectPermanentPreserve("/new-location")
			case "permanent":
				resp = response.RedirectPermanent("/new-location")
			}

			req := httptest.NewRequest(tt.method, "/old-location", nil)
			w := httptest.NewRecorder()

			err := resp(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "/new-location", w.Header().Get("Location"))
		})
	}
}

func TestRedirectWithExistingHeaders(t *testing.T) {
	t.Parallel()

	response := response.Redirect("/new-location")
	req := httptest.NewRequest("GET", "/old-location", nil)
	w := httptest.NewRecorder()

	// Set existing headers before redirect
	w.Header().Set("X-Custom-Header", "custom-value")
	w.Header().Set("Cache-Control", "no-cache")

	err := response(w, req)

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

			response := response.Redirect(tt.url)
			req := httptest.NewRequest("GET", "/old-location", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

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

		response := response.Redirect(longURL)
		req := httptest.NewRequest("GET", "/old", nil)
		w := httptest.NewRecorder()

		err := response(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, longURL, w.Header().Get("Location"))
	})

	t.Run("redirect_to_same_path", func(t *testing.T) {
		t.Parallel()

		response := response.Redirect("/same-path")
		req := httptest.NewRequest("GET", "/same-path", nil)
		w := httptest.NewRecorder()

		err := response(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/same-path", w.Header().Get("Location"))
	})

	t.Run("multiple_redirects_simulation", func(t *testing.T) {
		t.Parallel()

		// Simulate multiple redirects in sequence
		urls := []string{"/step1", "/step2", "/step3", "/final"}

		for i, url := range urls {
			response := response.Redirect(url)
			req := httptest.NewRequest("GET", "/start", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			assert.NoError(t, err, "Redirect step %d failed", i+1)
			assert.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, url, w.Header().Get("Location"))
		}
	})
}

func TestRedirectWithHTMX(t *testing.T) {
	t.Parallel()

	t.Run("htmx_redirect", func(t *testing.T) {
		t.Parallel()

		resp := response.Redirect("/dashboard")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code) // HTMX uses 200 OK
		assert.Equal(t, "/dashboard", w.Header().Get(response.HeaderHXLocation))
		assert.Empty(t, w.Header().Get("Location")) // No standard Location header
	})

	t.Run("htmx_redirect_permanent", func(t *testing.T) {
		t.Parallel()

		resp := response.RedirectPermanent("/new-location")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/new-location", w.Header().Get(response.HeaderHXLocation))
		assert.Empty(t, w.Header().Get("Location"))
	})

	t.Run("htmx_redirect_see_other", func(t *testing.T) {
		t.Parallel()

		resp := response.RedirectSeeOther("/success")
		req := httptest.NewRequest("POST", "/form", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/success", w.Header().Get(response.HeaderHXLocation))
		assert.Empty(t, w.Header().Get("Location"))
	})

	t.Run("htmx_redirect_temporary", func(t *testing.T) {
		t.Parallel()

		resp := response.RedirectTemporary("/maintenance")
		req := httptest.NewRequest("POST", "/api", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/maintenance", w.Header().Get(response.HeaderHXLocation))
		assert.Empty(t, w.Header().Get("Location"))
	})

	t.Run("htmx_redirect_permanent_preserve", func(t *testing.T) {
		t.Parallel()

		resp := response.RedirectPermanentPreserve("/api/v2")
		req := httptest.NewRequest("POST", "/api/v1", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/api/v2", w.Header().Get(response.HeaderHXLocation))
		assert.Empty(t, w.Header().Get("Location"))
	})

	t.Run("htmx_redirect_with_status", func(t *testing.T) {
		t.Parallel()

		resp := response.RedirectWithStatus("/new", http.StatusSeeOther)
		req := httptest.NewRequest("POST", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code) // HTMX always gets 200
		assert.Equal(t, "/new", w.Header().Get(response.HeaderHXLocation))
		assert.Empty(t, w.Header().Get("Location"))
	})

	t.Run("non_htmx_request_unchanged", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			redirectFn func(string) handler.Response
			url        string
			status     int
		}{
			{
				name:       "regular_redirect",
				redirectFn: response.Redirect,
				url:        "/dashboard",
				status:     http.StatusFound,
			},
			{
				name:       "regular_permanent",
				redirectFn: response.RedirectPermanent,
				url:        "/new",
				status:     http.StatusMovedPermanently,
			},
			{
				name:       "regular_see_other",
				redirectFn: response.RedirectSeeOther,
				url:        "/success",
				status:     http.StatusSeeOther,
			},
			{
				name:       "regular_temporary",
				redirectFn: response.RedirectTemporary,
				url:        "/temp",
				status:     http.StatusTemporaryRedirect,
			},
			{
				name:       "regular_permanent_preserve",
				redirectFn: response.RedirectPermanentPreserve,
				url:        "/v2",
				status:     http.StatusPermanentRedirect,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				resp := tt.redirectFn(tt.url)
				req := httptest.NewRequest("GET", "/old", nil)
				// No HX-Request header
				w := httptest.NewRecorder()

				err := resp(w, req)

				assert.NoError(t, err)
				assert.Equal(t, tt.status, w.Code)
				assert.Equal(t, tt.url, w.Header().Get("Location"))
				assert.Empty(t, w.Header().Get(response.HeaderHXLocation))
			})
		}
	})

	t.Run("htmx_request_false_value", func(t *testing.T) {
		t.Parallel()

		resp := response.Redirect("/dashboard")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "false") // Not "true"
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusFound, w.Code) // Regular redirect
		assert.Equal(t, "/dashboard", w.Header().Get("Location"))
		assert.Empty(t, w.Header().Get(response.HeaderHXLocation))
	})

	t.Run("htmx_boosted_request", func(t *testing.T) {
		t.Parallel()

		resp := response.Redirect("/dashboard")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		req.Header.Set(response.HeaderHXBoosted, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/dashboard", w.Header().Get(response.HeaderHXLocation))
	})

	t.Run("htmx_redirect_with_query_params", func(t *testing.T) {
		t.Parallel()

		resp := response.Redirect("/search?q=golang&page=2")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/search?q=golang&page=2", w.Header().Get(response.HeaderHXLocation))
	})

	t.Run("htmx_redirect_with_fragment", func(t *testing.T) {
		t.Parallel()

		resp := response.Redirect("/page#section")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/page#section", w.Header().Get(response.HeaderHXLocation))
	})

	t.Run("htmx_external_redirect", func(t *testing.T) {
		t.Parallel()

		resp := response.Redirect("https://example.com")
		req := httptest.NewRequest("GET", "/old", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		w := httptest.NewRecorder()

		err := resp(w, req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "https://example.com", w.Header().Get(response.HeaderHXLocation))
	})
}
