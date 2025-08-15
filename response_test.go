package gokit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
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

			response := gokit.String(tt.content)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

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

			response := gokit.StringWithStatus(tt.content, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

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

			response := gokit.HTML(tt.content)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

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

			response := gokit.HTMLWithStatus(tt.content, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

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

			response := gokit.Bytes(tt.content, tt.contentType)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

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

			response := gokit.BytesWithStatus(tt.content, tt.contentType, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, tt.contentType, w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expected, w.Body.String())
		})
	}
}

func TestNoContent(t *testing.T) {
	t.Parallel()

	response := gokit.NoContent()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)

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

			response := gokit.Status(tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

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
	response := gokit.String("test content")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Set custom header before rendering
	w.Header().Set("X-Custom-Header", "custom-value")
	w.Header().Set("Cache-Control", "no-cache")

	err := response.Render(w, req)

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
	response := gokit.String("test content")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Set different content type before rendering
	w.Header().Set("Content-Type", "application/json")

	err := response.Render(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	// Should override to text/plain
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "test content", w.Body.String())
}

func TestResponseEmptyContentType(t *testing.T) {
	t.Parallel()

	// Test Bytes with empty content type
	response := gokit.Bytes([]byte("test content"), "")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)

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

	response := gokit.Bytes(largeContent, "application/octet-stream")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, len(largeContent), len(w.Body.Bytes()))
	assert.Equal(t, largeContent, w.Body.Bytes())
}

func TestJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{
			name:     "simple_map",
			value:    map[string]string{"message": "Hello, World!"},
			expected: `{"message":"Hello, World!"}`,
		},
		{
			name: "simple_struct",
			value: struct {
				Name string `json:"name"`
			}{Name: "John Doe"},
			expected: `{"name":"John Doe"}`,
		},
		{
			name:     "nested_map",
			value:    map[string]any{"user": map[string]string{"name": "Alice", "email": "alice@example.com"}},
			expected: `{"user":{"email":"alice@example.com","name":"Alice"}}`,
		},
		{
			name:     "array_of_numbers",
			value:    []int{1, 2, 3, 4, 5},
			expected: `[1,2,3,4,5]`,
		},
		{
			name:     "array_of_strings",
			value:    []string{"apple", "banana", "cherry"},
			expected: `["apple","banana","cherry"]`,
		},
		{
			name:     "empty_map",
			value:    map[string]string{},
			expected: `{}`,
		},
		{
			name:     "empty_array",
			value:    []string{},
			expected: `[]`,
		},
		{
			name:     "nil_value",
			value:    nil,
			expected: `null`,
		},
		{
			name:     "boolean_values",
			value:    map[string]bool{"active": true, "deleted": false},
			expected: `{"active":true,"deleted":false}`,
		},
		{
			name:     "mixed_types",
			value:    map[string]any{"name": "test", "count": 42, "active": true, "tags": []string{"a", "b"}},
			expected: `{"active":true,"count":42,"name":"test","tags":["a","b"]}`,
		},
		{
			name:     "unicode_content",
			value:    map[string]string{"greeting": "Hello, 世界! 🌍"},
			expected: `{"greeting":"Hello, 世界! 🌍"}`,
		},
		{
			name: "complex_struct",
			value: struct {
				ID       int            `json:"id"`
				Name     string         `json:"name"`
				Email    string         `json:"email"`
				Tags     []string       `json:"tags"`
				Metadata map[string]any `json:"metadata"`
			}{
				ID:    123,
				Name:  "Test User",
				Email: "test@example.com",
				Tags:  []string{"admin", "active"},
				Metadata: map[string]any{
					"last_login": "2024-01-01",
					"attempts":   3,
				},
			},
			expected: `{"id":123,"name":"Test User","email":"test@example.com","tags":["admin","active"],"metadata":{"attempts":3,"last_login":"2024-01-01"}}`,
		},
		{
			name:     "string_value",
			value:    "just a string",
			expected: `"just a string"`,
		},
		{
			name:     "number_value",
			value:    42,
			expected: `42`,
		},
		{
			name:     "float_value",
			value:    3.14159,
			expected: `3.14159`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.JSON(tt.value)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			// Parse both expected and actual JSON to compare them properly
			// This handles differences in map key ordering
			var expectedJSON, actualJSON any
			err = json.Unmarshal([]byte(tt.expected), &expectedJSON)
			assert.NoError(t, err)
			err = json.Unmarshal(w.Body.Bytes(), &actualJSON)
			assert.NoError(t, err)
			assert.Equal(t, expectedJSON, actualJSON)
		})
	}
}

func TestJSONWithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      any
		statusCode int
		expected   string
	}{
		{
			name:       "created_status",
			value:      map[string]any{"id": 123, "status": "created"},
			statusCode: http.StatusCreated,
			expected:   `{"id":123,"status":"created"}`,
		},
		{
			name:       "accepted_status",
			value:      map[string]string{"message": "Request accepted"},
			statusCode: http.StatusAccepted,
			expected:   `{"message":"Request accepted"}`,
		},
		{
			name:       "bad_request_status",
			value:      map[string]string{"error": "Invalid input"},
			statusCode: http.StatusBadRequest,
			expected:   `{"error":"Invalid input"}`,
		},
		{
			name:       "not_found_status",
			value:      map[string]string{"error": "Resource not found"},
			statusCode: http.StatusNotFound,
			expected:   `{"error":"Resource not found"}`,
		},
		{
			name:       "internal_error_status",
			value:      map[string]string{"error": "Internal server error"},
			statusCode: http.StatusInternalServerError,
			expected:   `{"error":"Internal server error"}`,
		},
		{
			name:       "custom_status_code",
			value:      map[string]string{"message": "Custom response"},
			statusCode: 299,
			expected:   `{"message":"Custom response"}`,
		},
		{
			name:       "zero_status_defaults_to_ok",
			value:      map[string]string{"message": "Default status"},
			statusCode: 0,
			expected:   `{"message":"Default status"}`,
		},
		{
			name:       "no_content_with_data",
			value:      map[string]string{"data": "should still be sent"},
			statusCode: http.StatusNoContent,
			expected:   `{"data":"should still be sent"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.JSONWithStatus(tt.value, tt.statusCode)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)

			expectedStatus := tt.statusCode
			if expectedStatus == 0 {
				expectedStatus = http.StatusOK
			}

			assert.NoError(t, err)
			assert.Equal(t, expectedStatus, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			// Parse both expected and actual JSON to compare them properly
			var expectedJSON, actualJSON any
			err = json.Unmarshal([]byte(tt.expected), &expectedJSON)
			assert.NoError(t, err)
			err = json.Unmarshal(w.Body.Bytes(), &actualJSON)
			assert.NoError(t, err)
			assert.Equal(t, expectedJSON, actualJSON)
		})
	}
}

// TestJSONMarshalError removed - JSON marshaling errors are now returned from Render() method
// instead of panicking, which is a cleaner and more performant approach

func TestJSONWithCustomHeaders(t *testing.T) {
	t.Parallel()

	// Test that existing headers are not overwritten by JSON response
	response := gokit.JSON(map[string]string{"message": "test"})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Set custom headers before rendering
	w.Header().Set("X-Custom-Header", "custom-value")
	w.Header().Set("Cache-Control", "no-cache")

	err := response.Render(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))

	var result map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "test", result["message"])
}

func TestJSONLargeData(t *testing.T) {
	t.Parallel()

	// Create a large JSON structure
	largeData := make([]map[string]any, 1000)
	for i := range largeData {
		largeData[i] = map[string]any{
			"id":          i,
			"name":        "Item " + string(rune(i)),
			"description": "This is a longer description for item number " + string(rune(i)),
			"tags":        []string{"tag1", "tag2", "tag3"},
			"metadata": map[string]any{
				"created": "2024-01-01",
				"updated": "2024-01-02",
				"version": i,
			},
		}
	}

	response := gokit.JSON(largeData)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	// Verify the JSON is valid and can be unmarshaled
	var result []map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1000)
	assert.Equal(t, float64(0), result[0]["id"])
	assert.Equal(t, float64(999), result[999]["id"])
}
