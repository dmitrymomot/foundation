package response_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/foundation/core/response"
)

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple_string",
			content:  "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "empty_string",
			content:  "",
			expected: "",
		},
		{
			name:     "string_with_special_chars",
			content:  "Hello, 世界! 🌍",
			expected: "Hello, 世界! 🌍",
		},
		{
			name:     "multiline_string",
			content:  "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.String(tt.content)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestStringWithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		statusCode int
		expected   string
	}{
		{
			name:       "created_status",
			content:    "Resource created",
			statusCode: http.StatusCreated,
			expected:   "Resource created",
		},
		{
			name:       "accepted_status",
			content:    "Request accepted",
			statusCode: http.StatusAccepted,
			expected:   "Request accepted",
		},
		{
			name:       "bad_request_status",
			content:    "Invalid input",
			statusCode: http.StatusBadRequest,
			expected:   "Invalid input",
		},
		{
			name:       "custom_status_code",
			content:    "Custom response",
			statusCode: 299,
			expected:   "Custom response",
		},
		{
			name:       "zero_status_defaults_to_ok",
			content:    "Default status",
			statusCode: 0,
			expected:   "Default status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.StringWithStatus(tt.content, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			expectedStatus := tt.statusCode
			if expectedStatus == 0 {
				expectedStatus = http.StatusOK
			}

			assert.NoError(t, err)
			assert.Equal(t, expectedStatus, w.Code)
			assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple_html",
			content:  "<h1>Hello, World!</h1>",
			expected: "<h1>Hello, World!</h1>",
		},
		{
			name:     "empty_html",
			content:  "",
			expected: "",
		},
		{
			name:     "complex_html",
			content:  `<html><body><h1>Title</h1><p>Paragraph</p></body></html>`,
			expected: `<html><body><h1>Title</h1><p>Paragraph</p></body></html>`,
		},
		{
			name:     "html_with_attributes",
			content:  `<div class="container" id="main"><span style="color: red;">Text</span></div>`,
			expected: `<div class="container" id="main"><span style="color: red;">Text</span></div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.HTML(tt.content)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestHTMLWithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		statusCode int
		expected   string
	}{
		{
			name:       "success_page",
			content:    "<h1>Success!</h1>",
			statusCode: http.StatusOK,
			expected:   "<h1>Success!</h1>",
		},
		{
			name:       "not_found_page",
			content:    "<h1>Page Not Found</h1>",
			statusCode: http.StatusNotFound,
			expected:   "<h1>Page Not Found</h1>",
		},
		{
			name:       "error_page",
			content:    "<h1>Internal Server Error</h1>",
			statusCode: http.StatusInternalServerError,
			expected:   "<h1>Internal Server Error</h1>",
		},
		{
			name:       "custom_status",
			content:    "<h1>Custom Status</h1>",
			statusCode: 299,
			expected:   "<h1>Custom Status</h1>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.HTMLWithStatus(tt.content, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     []byte
		contentType string
		expected    string
	}{
		{
			name:        "json_bytes",
			content:     []byte(`{"message": "Hello, World!"}`),
			contentType: "application/json",
			expected:    `{"message": "Hello, World!"}`,
		},
		{
			name:        "xml_bytes",
			content:     []byte(`<message>Hello, World!</message>`),
			contentType: "application/xml",
			expected:    `<message>Hello, World!</message>`,
		},
		{
			name:        "plain_text_bytes",
			content:     []byte("Plain text content"),
			contentType: "text/plain",
			expected:    "Plain text content",
		},
		{
			name:        "binary_data",
			content:     []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}, // "Hello" in bytes
			contentType: "application/octet-stream",
			expected:    "Hello",
		},
		{
			name:        "empty_bytes",
			content:     []byte{},
			contentType: "application/json",
			expected:    "",
		},
		{
			name:        "nil_bytes",
			content:     nil,
			contentType: "application/json",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.Bytes(tt.content, tt.contentType)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.contentType, w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestBytesWithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     []byte
		contentType string
		statusCode  int
		expected    string
	}{
		{
			name:        "created_json",
			content:     []byte(`{"id": 123, "status": "created"}`),
			contentType: "application/json",
			statusCode:  http.StatusCreated,
			expected:    `{"id": 123, "status": "created"}`,
		},
		{
			name:        "bad_request_json",
			content:     []byte(`{"error": "Invalid input"}`),
			contentType: "application/json",
			statusCode:  http.StatusBadRequest,
			expected:    `{"error": "Invalid input"}`,
		},
		{
			name:        "custom_content_type",
			content:     []byte("custom data"),
			contentType: "application/custom",
			statusCode:  http.StatusAccepted,
			expected:    "custom data",
		},
		{
			name:        "image_data",
			content:     []byte("fake image data"),
			contentType: "image/png",
			statusCode:  http.StatusOK,
			expected:    "fake image data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.BytesWithStatus(tt.content, tt.contentType, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, tt.contentType, w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestNoContent(t *testing.T) {
	t.Parallel()

	response := response.NoContent()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
	assert.Empty(t, w.Header().Get("Content-Type"))
}

func TestStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{
			name:       "ok_status",
			statusCode: http.StatusOK,
		},
		{
			name:       "created_status",
			statusCode: http.StatusCreated,
		},
		{
			name:       "not_found_status",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "internal_error_status",
			statusCode: http.StatusInternalServerError,
		},
		{
			name:       "custom_status",
			statusCode: 299,
		},
		{
			name:       "zero_status", // Zero status defaults to 200 OK
			statusCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := response.Status(tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)

			expectedStatus := tt.statusCode
			if expectedStatus == 0 {
				expectedStatus = http.StatusOK // baseResponse defaults zero status to 200
			}

			assert.NoError(t, err)
			assert.Equal(t, expectedStatus, w.Code)
			assert.Empty(t, w.Body.String())
			assert.Empty(t, w.Header().Get("Content-Type"))
		})
	}
}

func TestResponseWithCustomHeaders(t *testing.T) {
	t.Parallel()

	// Test that existing headers are not overwritten by response helpers
	response := response.String("test content")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Set custom header before rendering
	w.Header().Set("X-Custom-Header", "custom-value")
	w.Header().Set("Cache-Control", "no-cache")

	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "test content", w.Body.String())
}

func TestResponseContentTypeOverride(t *testing.T) {
	t.Parallel()

	// Test that response helpers set content-type even if already set
	response := response.String("test content")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Set different content type before rendering
	w.Header().Set("Content-Type", "application/json")

	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	// Should override to text/plain
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "test content", w.Body.String())
}

func TestResponseEmptyContentType(t *testing.T) {
	t.Parallel()

	// Test Bytes with empty content type
	response := response.Bytes([]byte("test content"), "")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Content-Type"))
	assert.Equal(t, "test content", w.Body.String())
}

func TestResponseLargeContent(t *testing.T) {
	t.Parallel()

	// Test with large content to ensure Write handles it properly
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	response := response.Bytes(largeContent, "application/octet-stream")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, len(largeContent), len(w.Body.Bytes()))
	assert.Equal(t, largeContent, w.Body.Bytes())
}
