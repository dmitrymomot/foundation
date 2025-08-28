package response

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
)

// executeTemplate runs the template with the appropriate method.
func executeTemplate(tmpl *template.Template, name string, data any, w io.Writer) error {
	if tmpl == nil {
		return fmt.Errorf("template is nil")
	}

	if name != "" {
		// Use ExecuteTemplate for named templates
		return tmpl.ExecuteTemplate(w, name, data)
	}
	// Use Execute for single templates
	return tmpl.Execute(w, data)
}

// Template creates an HTML response using html/template with 200 OK status.
// The template is buffered before writing (safer, prevents partial output on error).
func Template(tmpl *template.Template, data any) handler.Response {
	if tmpl == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Buffer the output first (safer - can catch template errors)
		var buf bytes.Buffer
		err := executeTemplate(tmpl, "", data, &buf)
		if err != nil {
			// Don't write anything if template fails
			return err
		}
		// Write status and content only after successful render
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write(buf.Bytes())
		return writeErr
	}
}

// TemplateWithStatus creates an HTML response using html/template with custom status code.
// The template is buffered before writing.
func TemplateWithStatus(tmpl *template.Template, data any, status int) handler.Response {
	if tmpl == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Determine status code
		resolvedStatus := status
		if resolvedStatus == 0 {
			resolvedStatus = http.StatusOK
		}

		// Buffer the output first (safer - can catch template errors)
		var buf bytes.Buffer
		err := executeTemplate(tmpl, "", data, &buf)
		if err != nil {
			// Don't write anything if template fails
			return err
		}
		// Write status and content only after successful render
		w.WriteHeader(resolvedStatus)
		_, writeErr := w.Write(buf.Bytes())
		return writeErr
	}
}

// TemplateName renders a named template from a template collection (e.g., from ParseFiles or ParseGlob).
// This is useful when you have multiple templates defined in files.
func TemplateName(tmpl *template.Template, name string, data any) handler.Response {
	if tmpl == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Buffer the output first (safer - can catch template errors)
		var buf bytes.Buffer
		err := executeTemplate(tmpl, name, data, &buf)
		if err != nil {
			// Don't write anything if template fails
			return err
		}
		// Write status and content only after successful render
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write(buf.Bytes())
		return writeErr
	}
}

// TemplateNameWithStatus renders a named template with a custom status code.
func TemplateNameWithStatus(tmpl *template.Template, name string, data any, status int) handler.Response {
	if tmpl == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Determine status code
		resolvedStatus := status
		if resolvedStatus == 0 {
			resolvedStatus = http.StatusOK
		}

		// Buffer the output first (safer - can catch template errors)
		var buf bytes.Buffer
		err := executeTemplate(tmpl, name, data, &buf)
		if err != nil {
			// Don't write anything if template fails
			return err
		}
		// Write status and content only after successful render
		w.WriteHeader(resolvedStatus)
		_, writeErr := w.Write(buf.Bytes())
		return writeErr
	}
}

// TemplateStream creates an HTML response that streams directly to the client.
// This is more memory efficient for large templates but cannot recover from template errors
// after headers are sent. Use this only when you're confident the template will succeed.
func TemplateStream(tmpl *template.Template, data any) handler.Response {
	if tmpl == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Stream directly to response (more efficient but can't recover from errors)
		w.WriteHeader(http.StatusOK)
		return executeTemplate(tmpl, "", data, w)
	}
}

// TemplateStreamWithStatus creates a streaming HTML response with custom status code.
func TemplateStreamWithStatus(tmpl *template.Template, data any, status int) handler.Response {
	if tmpl == nil {
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request) error {
		// Set HTML content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Determine status code
		resolvedStatus := status
		if resolvedStatus == 0 {
			resolvedStatus = http.StatusOK
		}

		// Stream directly to response (more efficient but can't recover from errors)
		w.WriteHeader(resolvedStatus)
		return executeTemplate(tmpl, "", data, w)
	}
}
