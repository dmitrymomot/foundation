package gokit

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
)

// templateResponse handles html/template rendering.
type templateResponse struct {
	tmpl       *template.Template
	name       string // optional: for ExecuteTemplate
	data       any
	statusCode int
	buffered   bool // if true, buffer before writing (safer but uses more memory)
}

// Render implements the Response interface.
func (r *templateResponse) Render(w http.ResponseWriter, req *http.Request) error {
	// Set HTML content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Determine status code
	status := r.statusCode
	if status == 0 {
		status = http.StatusOK
	}

	if r.buffered {
		// Buffer the output first (safer - can catch template errors)
		var buf bytes.Buffer
		err := r.execute(&buf)
		if err != nil {
			// Don't write anything if template fails
			return err
		}
		// Write status and content only after successful render
		w.WriteHeader(status)
		_, writeErr := w.Write(buf.Bytes())
		return writeErr
	}

	// Stream directly to response (more efficient but can't recover from errors)
	w.WriteHeader(status)
	return r.execute(w)
}

// execute runs the template with the appropriate method.
func (r *templateResponse) execute(w io.Writer) error {
	if r.tmpl == nil {
		return ErrInternalServerError.WithMessage("template is nil")
	}

	if r.name != "" {
		// Use ExecuteTemplate for named templates
		return r.tmpl.ExecuteTemplate(w, r.name, r.data)
	}
	// Use Execute for single templates
	return r.tmpl.Execute(w, r.data)
}

// Template creates an HTML response using html/template with 200 OK status.
// The template is buffered before writing (safer, prevents partial output on error).
func Template(tmpl *template.Template, data any) Response {
	if tmpl == nil {
		return nil
	}
	return &templateResponse{
		tmpl:     tmpl,
		data:     data,
		buffered: true,
	}
}

// TemplateWithStatus creates an HTML response using html/template with custom status code.
// The template is buffered before writing.
func TemplateWithStatus(tmpl *template.Template, data any, status int) Response {
	if tmpl == nil {
		return nil
	}
	return &templateResponse{
		tmpl:       tmpl,
		data:       data,
		statusCode: status,
		buffered:   true,
	}
}

// TemplateName renders a named template from a template collection (e.g., from ParseFiles or ParseGlob).
// This is useful when you have multiple templates defined in files.
func TemplateName(tmpl *template.Template, name string, data any) Response {
	if tmpl == nil {
		return nil
	}
	return &templateResponse{
		tmpl:     tmpl,
		name:     name,
		data:     data,
		buffered: true,
	}
}

// TemplateNameWithStatus renders a named template with a custom status code.
func TemplateNameWithStatus(tmpl *template.Template, name string, data any, status int) Response {
	if tmpl == nil {
		return nil
	}
	return &templateResponse{
		tmpl:       tmpl,
		name:       name,
		data:       data,
		statusCode: status,
		buffered:   true,
	}
}

// TemplateStream creates an HTML response that streams directly to the client.
// This is more memory efficient for large templates but cannot recover from template errors
// after headers are sent. Use this only when you're confident the template will succeed.
func TemplateStream(tmpl *template.Template, data any) Response {
	if tmpl == nil {
		return nil
	}
	return &templateResponse{
		tmpl:     tmpl,
		data:     data,
		buffered: false,
	}
}

// TemplateStreamWithStatus creates a streaming HTML response with custom status code.
func TemplateStreamWithStatus(tmpl *template.Template, data any, status int) Response {
	if tmpl == nil {
		return nil
	}
	return &templateResponse{
		tmpl:       tmpl,
		data:       data,
		statusCode: status,
		buffered:   false,
	}
}
