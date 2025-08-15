package gokit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
)

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
			expected:   ``, // 204 No Content must not have a body per HTTP spec
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
			// Handle empty body for 204 No Content
			if tt.expected == "" {
				assert.Empty(t, w.Body.String())
			} else {
				var expectedJSON, actualJSON any
				err = json.Unmarshal([]byte(tt.expected), &expectedJSON)
				assert.NoError(t, err)
				err = json.Unmarshal(w.Body.Bytes(), &actualJSON)
				assert.NoError(t, err)
				assert.Equal(t, expectedJSON, actualJSON)
			}
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
