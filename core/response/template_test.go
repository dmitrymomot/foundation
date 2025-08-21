package response_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/response"
)

func TestTemplate(t *testing.T) {
	t.Parallel()

	tmpl := template.Must(template.New("test").Parse(`<h1>Hello {{.Name}}</h1>`))

	tests := []struct {
		name            string
		template        *template.Template
		data            any
		expectedStatus  int
		expectedContent string
		expectedType    string
	}{
		{
			name:            "simple_template",
			template:        tmpl,
			data:            map[string]string{"Name": "World"},
			expectedStatus:  http.StatusOK,
			expectedContent: "<h1>Hello World</h1>",
			expectedType:    "text/html; charset=utf-8",
		},
		{
			name:            "template_with_struct",
			template:        tmpl,
			data:            struct{ Name string }{Name: "GoKit"},
			expectedStatus:  http.StatusOK,
			expectedContent: "<h1>Hello GoKit</h1>",
			expectedType:    "text/html; charset=utf-8",
		},
		{
			name:            "template_with_nil_data",
			template:        template.Must(template.New("static").Parse(`<p>Static content</p>`)),
			data:            nil,
			expectedStatus:  http.StatusOK,
			expectedContent: "<p>Static content</p>",
			expectedType:    "text/html; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := response.Template(tt.template, tt.data)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.String())
			assert.Equal(t, tt.expectedType, w.Header().Get("Content-Type"))
		})
	}
}

func TestTemplateWithStatus(t *testing.T) {
	t.Parallel()

	tmpl := template.Must(template.New("error").Parse(`<h1>{{.Code}} - {{.Message}}</h1>`))

	tests := []struct {
		name            string
		template        *template.Template
		data            any
		status          int
		expectedContent string
	}{
		{
			name:            "not_found",
			template:        tmpl,
			data:            map[string]any{"Code": 404, "Message": "Not Found"},
			status:          http.StatusNotFound,
			expectedContent: "<h1>404 - Not Found</h1>",
		},
		{
			name:            "internal_error",
			template:        tmpl,
			data:            map[string]any{"Code": 500, "Message": "Internal Server Error"},
			status:          http.StatusInternalServerError,
			expectedContent: "<h1>500 - Internal Server Error</h1>",
		},
		{
			name:            "created",
			template:        template.Must(template.New("created").Parse(`<p>Created successfully</p>`)),
			data:            nil,
			status:          http.StatusCreated,
			expectedContent: "<p>Created successfully</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := response.TemplateWithStatus(tt.template, tt.data, tt.status)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.String())
			assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		})
	}
}

func TestTemplateName(t *testing.T) {
	t.Parallel()

	// Create a template collection like ParseFiles would
	templates := template.New("root")
	template.Must(templates.New("index.html").Parse(`<h1>Index: {{.Title}}</h1>`))
	template.Must(templates.New("about.html").Parse(`<h2>About: {{.Content}}</h2>`))
	template.Must(templates.New("contact.html").Parse(`<h3>Contact: {{.Email}}</h3>`))

	tests := []struct {
		name            string
		templateName    string
		data            any
		expectedContent string
	}{
		{
			name:            "index_template",
			templateName:    "index.html",
			data:            map[string]string{"Title": "Welcome"},
			expectedContent: "<h1>Index: Welcome</h1>",
		},
		{
			name:            "about_template",
			templateName:    "about.html",
			data:            map[string]string{"Content": "Our Story"},
			expectedContent: "<h2>About: Our Story</h2>",
		},
		{
			name:            "contact_template",
			templateName:    "contact.html",
			data:            map[string]string{"Email": "test@example.com"},
			expectedContent: "<h3>Contact: test@example.com</h3>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := response.TemplateName(templates, tt.templateName, tt.data)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response(w, req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.String())
		})
	}
}

func TestTemplateNameWithStatus(t *testing.T) {
	t.Parallel()

	templates := template.New("root")
	template.Must(templates.New("404.html").Parse(`<h1>404 - Page Not Found</h1>`))
	template.Must(templates.New("500.html").Parse(`<h1>500 - Server Error</h1>`))

	response := response.TemplateNameWithStatus(templates, "404.html", nil, http.StatusNotFound)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "<h1>404 - Page Not Found</h1>", w.Body.String())
}

func TestTemplateStream(t *testing.T) {
	t.Parallel()

	tmpl := template.Must(template.New("stream").Parse(`<div>{{.Content}}</div>`))
	data := map[string]string{"Content": "Streamed content"}

	response := response.TemplateStream(tmpl, data)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "<div>Streamed content</div>", w.Body.String())
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestTemplateStreamWithStatus(t *testing.T) {
	t.Parallel()

	tmpl := template.Must(template.New("stream").Parse(`<h1>Accepted</h1>`))

	response := response.TemplateStreamWithStatus(tmpl, nil, http.StatusAccepted)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "<h1>Accepted</h1>", w.Body.String())
}

func TestTemplate_NilTemplate(t *testing.T) {
	t.Parallel()

	// All functions should return nil for nil template
	assert.Nil(t, response.Template(nil, nil))
	assert.Nil(t, response.TemplateWithStatus(nil, nil, 200))
	assert.Nil(t, response.TemplateName(nil, "test", nil))
	assert.Nil(t, response.TemplateNameWithStatus(nil, "test", nil, 200))
	assert.Nil(t, response.TemplateStream(nil, nil))
	assert.Nil(t, response.TemplateStreamWithStatus(nil, nil, 200))
}

func TestTemplate_TemplateError(t *testing.T) {
	t.Parallel()

	// Template with undefined variable
	tmpl := template.Must(template.New("error").Parse(`<h1>{{.UndefinedField}}</h1>`))
	data := map[string]string{"Name": "Test"}

	response := response.Template(tmpl, data)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Should return empty string for missing field (Go template behavior)
	err := response(w, req)
	assert.NoError(t, err)
	assert.Equal(t, "<h1></h1>", w.Body.String())
}

func TestTemplate_InvalidTemplateName(t *testing.T) {
	t.Parallel()

	templates := template.New("root")
	template.Must(templates.New("valid.html").Parse(`<p>Valid</p>`))

	// Try to render non-existent template
	response := response.TemplateName(templates, "nonexistent.html", nil)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.html")
}

func TestTemplate_ComplexTemplate(t *testing.T) {
	t.Parallel()

	tmpl := template.Must(template.New("complex").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
</head>
<body>
    <h1>{{.Title}}</h1>
    {{if .Items}}
    <ul>
        {{range .Items}}
        <li>{{.}}</li>
        {{end}}
    </ul>
    {{else}}
    <p>No items</p>
    {{end}}
</body>
</html>`))

	data := map[string]any{
		"Title": "My Page",
		"Items": []string{"Item 1", "Item 2", "Item 3"},
	}

	response := response.Template(tmpl, data)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "<title>My Page</title>")
	assert.Contains(t, body, "<h1>My Page</h1>")
	assert.Contains(t, body, "<li>Item 1</li>")
	assert.Contains(t, body, "<li>Item 2</li>")
	assert.Contains(t, body, "<li>Item 3</li>")
}

func TestTemplate_BufferedVsStreaming(t *testing.T) {
	t.Parallel()

	// Test with a template that calls an undefined function
	// This will cause an error during execution
	tmpl := template.Must(template.New("fail").Parse(`{{call .Func}}`))

	t.Run("buffered_invalid_function", func(t *testing.T) {
		// Try to call undefined function
		response := response.Template(tmpl, map[string]any{"NotFunc": "value"})
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Func")
	})

	t.Run("streaming_invalid_function", func(t *testing.T) {
		// For streaming, the error happens after headers are sent
		response := response.TemplateStream(tmpl, map[string]any{"NotFunc": "value"})
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		err := response(w, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Func")
		// Headers would have been sent with streaming
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestTemplate_LargeContent(t *testing.T) {
	t.Parallel()

	// Create template with large content
	largeText := strings.Repeat("This is a test paragraph. ", 1000)
	tmpl := template.Must(template.New("large").Parse(`<div>{{.}}</div>`))

	response := response.Template(tmpl, largeText)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), largeText)
}

func TestTemplate_WithDecorators(t *testing.T) {
	t.Parallel()

	tmpl := template.Must(template.New("decorated").Parse(`<h1>Dashboard</h1>`))

	// Apply decorators
	response := response.WithCache(
		response.WithHeaders(
			response.Template(tmpl, nil),
			map[string]string{
				"X-Frame-Options": "DENY",
				"X-Custom-Header": "value",
			},
		),
		time.Hour,
	)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	require.NoError(t, err)

	// Check all headers are set
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "public, max-age=3600", w.Header().Get("Cache-Control"))

	// Check content
	assert.Equal(t, "<h1>Dashboard</h1>", w.Body.String())
}

func TestTemplate_HTMLEscaping(t *testing.T) {
	t.Parallel()

	// html/template automatically escapes HTML
	tmpl := template.Must(template.New("escape").Parse(`<div>{{.Content}}</div>`))
	data := map[string]string{
		"Content": `<script>alert('XSS')</script>`,
	}

	response := response.Template(tmpl, data)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response(w, req)
	assert.NoError(t, err)

	// Check that HTML is escaped
	assert.Equal(t, `<div>&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;</div>`, w.Body.String())
}
