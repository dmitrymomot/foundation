package gokit_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type templComponent interface {
	Render(ctx context.Context, w io.Writer) error
}

type templComponentFunc (func(ctx context.Context, w io.Writer) error)

func (fn templComponentFunc) Render(ctx context.Context, w io.Writer) error {
	return fn(ctx, w)
}

// mockComponent creates a simple test component
func mockComponent(content string) templComponent {
	return templComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(content))
		return err
	})
}

// errorComponent creates a component that returns an error
func errorComponent(errMsg string) templComponent {
	return templComponentFunc(func(ctx context.Context, w io.Writer) error {
		return errors.New(errMsg)
	})
}

// contextAwareComponent creates a component that reads from context
func contextAwareComponent(key string) templComponent {
	return templComponentFunc(func(ctx context.Context, w io.Writer) error {
		value := ctx.Value(key)
		if value != nil {
			_, err := w.Write([]byte(value.(string)))
			return err
		}
		_, err := w.Write([]byte("no value"))
		return err
	})
}

func TestTempl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		component       templComponent
		expectedStatus  int
		expectedContent string
		expectedType    string
	}{
		{
			name:            "simple_html",
			component:       mockComponent("<h1>Hello World</h1>"),
			expectedStatus:  http.StatusOK,
			expectedContent: "<h1>Hello World</h1>",
			expectedType:    "text/html; charset=utf-8",
		},
		{
			name:            "complex_html",
			component:       mockComponent(`<div class="container"><p>Test</p></div>`),
			expectedStatus:  http.StatusOK,
			expectedContent: `<div class="container"><p>Test</p></div>`,
			expectedType:    "text/html; charset=utf-8",
		},
		{
			name:            "empty_content",
			component:       mockComponent(""),
			expectedStatus:  http.StatusOK,
			expectedContent: "",
			expectedType:    "text/html; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := gokit.Templ(tt.component)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.String())
			assert.Equal(t, tt.expectedType, w.Header().Get("Content-Type"))
		})
	}
}

func TestTemplWithStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		component       templComponent
		status          int
		expectedContent string
	}{
		{
			name:            "not_found",
			component:       mockComponent("<h1>404 - Not Found</h1>"),
			status:          http.StatusNotFound,
			expectedContent: "<h1>404 - Not Found</h1>",
		},
		{
			name:            "internal_error",
			component:       mockComponent("<h1>500 - Internal Server Error</h1>"),
			status:          http.StatusInternalServerError,
			expectedContent: "<h1>500 - Internal Server Error</h1>",
		},
		{
			name:            "created",
			component:       mockComponent("<p>Resource created</p>"),
			status:          http.StatusCreated,
			expectedContent: "<p>Resource created</p>",
		},
		{
			name:            "no_content_with_body",
			component:       mockComponent("shouldn't see this"),
			status:          http.StatusNoContent,
			expectedContent: "shouldn't see this", // templ will still render, HTTP spec violation is on the user
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := gokit.TemplWithStatus(tt.component, tt.status)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.String())
			assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		})
	}
}

func TestTempl_NilComponent(t *testing.T) {
	t.Parallel()

	// Test Templ with nil
	response := gokit.Templ(nil)
	assert.Nil(t, response)

	// Test TemplWithStatus with nil
	response = gokit.TemplWithStatus(nil, http.StatusOK)
	assert.Nil(t, response)
}

func TestTempl_ComponentError(t *testing.T) {
	t.Parallel()

	component := errorComponent("render failed")
	response := gokit.Templ(component)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.Error(t, err)

	// Check that it's an Error type with the original error in details
	var appErr gokit.Error
	if assert.ErrorAs(t, err, &appErr) {
		assert.Equal(t, "Internal Server Error", appErr.Message)
		assert.Equal(t, "internal_server_error", appErr.Code)
		assert.Equal(t, "render failed", appErr.Details["cause"])
	}

	// Headers should still be set
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTempl_ContextPropagation(t *testing.T) {
	t.Parallel()

	// Create a component that reads from context
	component := contextAwareComponent("user_id")
	response := gokit.Templ(component)

	// Create request with context value
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), "user_id", "12345")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, "12345", w.Body.String())
}

func TestTempl_LargeContent(t *testing.T) {
	t.Parallel()

	// Create large HTML content
	largeContent := strings.Repeat("<div>This is a test content block.</div>", 1000)
	component := mockComponent(largeContent)
	response := gokit.Templ(component)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, largeContent, w.Body.String())
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestTempl_WithDecorators(t *testing.T) {
	t.Parallel()

	component := mockComponent("<h1>Dashboard</h1>")

	// Apply decorators
	response := gokit.WithCache(
		gokit.WithHeaders(
			gokit.Templ(component),
			map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
			},
		),
		time.Hour,
	)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	require.NoError(t, err)

	// Check all headers are set
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "public, max-age=3600", w.Header().Get("Cache-Control"))

	// Check content
	assert.Equal(t, "<h1>Dashboard</h1>", w.Body.String())
}

// TestTempl_RealWorldExample demonstrates a more realistic templ component usage
func TestTempl_RealWorldExample(t *testing.T) {
	t.Parallel()

	// Simulate a page component with dynamic data
	pageComponent := templComponentFunc(func(ctx context.Context, w io.Writer) error {
		html := `<!DOCTYPE html>
<html>
<head>
    <title>Welcome</title>
</head>
<body>
    <h1>Welcome to GoKit</h1>
    <p>This is rendered with templ!</p>
</body>
</html>`
		_, err := w.Write([]byte(html))
		return err
	})

	response := gokit.Templ(pageComponent)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Welcome to GoKit")
	assert.Contains(t, w.Body.String(), "This is rendered with templ!")
}
