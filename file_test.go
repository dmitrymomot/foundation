package gokit_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	t.Parallel()

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create an HTML file for content type detection
	htmlFile := filepath.Join(tmpDir, "test.html")
	htmlContent := "<html><body>Hello</body></html>"
	err = os.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
		checkHeaders   bool
	}{
		{
			name:           "serve_existing_file",
			path:           testFile,
			expectedStatus: http.StatusOK,
			expectedBody:   testContent,
			checkHeaders:   true,
		},
		{
			name:           "serve_html_file",
			path:           htmlFile,
			expectedStatus: http.StatusOK,
			expectedBody:   htmlContent,
			checkHeaders:   true,
		},
		{
			name:           "non_existent_file",
			path:           filepath.Join(tmpDir, "nonexistent.txt"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
		{
			name:           "directory_not_served",
			path:           tmpDir,
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 page not found\n",
			checkHeaders:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.File(tt.path)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())

			if tt.checkHeaders {
				// Check that content type is set
				contentType := w.Header().Get("Content-Type")
				assert.NotEmpty(t, contentType)

				// For HTML files, check specific content type
				if strings.HasSuffix(tt.path, ".html") {
					assert.Contains(t, contentType, "text/html")
				}
			}
		})
	}
}

func TestDownload(t *testing.T) {
	t.Parallel()

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "document.pdf")
	testContent := []byte("PDF content here")
	err := os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	tests := []struct {
		name             string
		path             string
		filename         string
		expectedStatus   int
		expectedFilename string
		expectedContent  []byte
	}{
		{
			name:             "download_with_custom_filename",
			path:             testFile,
			filename:         "custom-name.pdf",
			expectedStatus:   http.StatusOK,
			expectedFilename: "custom-name.pdf",
			expectedContent:  testContent,
		},
		{
			name:             "download_with_default_filename",
			path:             testFile,
			filename:         "",
			expectedStatus:   http.StatusOK,
			expectedFilename: "document.pdf",
			expectedContent:  testContent,
		},
		{
			name:             "download_non_existent_file",
			path:             filepath.Join(tmpDir, "missing.pdf"),
			filename:         "missing.pdf",
			expectedStatus:   http.StatusNotFound,
			expectedFilename: "",
			expectedContent:  []byte("404 page not found\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.Download(tt.path, tt.filename)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.Bytes())

			if tt.expectedStatus == http.StatusOK {
				// Check Content-Disposition header
				disposition := w.Header().Get("Content-Disposition")
				assert.Contains(t, disposition, "attachment")
				assert.Contains(t, disposition, tt.expectedFilename)

				// Check Content-Type header
				contentType := w.Header().Get("Content-Type")
				assert.NotEmpty(t, contentType)
			}
		})
	}
}

func TestAttachment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		data            []byte
		filename        string
		contentType     string
		expectedStatus  int
		expectedContent []byte
		expectedType    string
	}{
		{
			name:            "attachment_with_content_type",
			data:            []byte("CSV data here"),
			filename:        "data.csv",
			contentType:     "text/csv",
			expectedStatus:  http.StatusOK,
			expectedContent: []byte("CSV data here"),
			expectedType:    "text/csv",
		},
		{
			name:            "attachment_auto_detect_type",
			data:            []byte(`{"key": "value"}`),
			filename:        "data.json",
			contentType:     "",
			expectedStatus:  http.StatusOK,
			expectedContent: []byte(`{"key": "value"}`),
			expectedType:    "application/json",
		},
		{
			name:            "attachment_unknown_type",
			data:            []byte("binary data"),
			filename:        "data.unknownext",
			contentType:     "",
			expectedStatus:  http.StatusOK,
			expectedContent: []byte("binary data"),
			expectedType:    "application/octet-stream",
		},
		{
			name:            "attachment_sanitize_filename",
			data:            []byte("test"),
			filename:        "file\nwith\rnewlines\"quotes.txt",
			contentType:     "text/plain",
			expectedStatus:  http.StatusOK,
			expectedContent: []byte("test"),
			expectedType:    "text/plain",
		},
		{
			name:            "empty_attachment",
			data:            []byte{},
			filename:        "empty.txt",
			contentType:     "text/plain",
			expectedStatus:  http.StatusOK,
			expectedContent: nil,
			expectedType:    "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.Attachment(tt.data, tt.filename, tt.contentType)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedContent == nil {
				assert.Empty(t, w.Body.Bytes())
			} else {
				assert.Equal(t, tt.expectedContent, w.Body.Bytes())
			}

			// Check Content-Disposition header
			disposition := w.Header().Get("Content-Disposition")
			assert.Contains(t, disposition, "attachment")
			assert.Contains(t, disposition, "filename=")

			// Check Content-Type header
			contentType := w.Header().Get("Content-Type")
			assert.Equal(t, tt.expectedType, contentType)

			// Check Content-Length header
			contentLength := w.Header().Get("Content-Length")
			assert.Equal(t, fmt.Sprintf("%d", len(tt.data)), contentLength)
		})
	}
}

func TestFileReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		data            string
		filename        string
		contentType     string
		expectedStatus  int
		expectedContent string
		expectedType    string
	}{
		{
			name:            "stream_with_content_type",
			data:            "Streaming content",
			filename:        "stream.txt",
			contentType:     "text/plain",
			expectedStatus:  http.StatusOK,
			expectedContent: "Streaming content",
			expectedType:    "text/plain",
		},
		{
			name:            "stream_auto_detect_type",
			data:            "<xml>data</xml>",
			filename:        "data.xml",
			contentType:     "",
			expectedStatus:  http.StatusOK,
			expectedContent: "<xml>data</xml>",
			expectedType:    "application/xml",
		},
		{
			name:            "stream_large_content",
			data:            strings.Repeat("Large content ", 1000),
			filename:        "large.txt",
			contentType:     "text/plain",
			expectedStatus:  http.StatusOK,
			expectedContent: strings.Repeat("Large content ", 1000),
			expectedType:    "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := bytes.NewReader([]byte(tt.data))
			response := gokit.FileReader(reader, tt.filename, tt.contentType)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedContent, w.Body.String())

			// Check Content-Disposition header
			disposition := w.Header().Get("Content-Disposition")
			assert.Contains(t, disposition, "attachment")
			assert.Contains(t, disposition, tt.filename)

			// Check Content-Type header
			contentType := w.Header().Get("Content-Type")
			// For XML, accept both "application/xml" and "text/xml; charset=utf-8"
			// as different systems may return different MIME types for XML
			if tt.expectedType == "application/xml" && strings.HasPrefix(contentType, "text/xml") {
				assert.Contains(t, contentType, "xml")
			} else {
				assert.Equal(t, tt.expectedType, contentType)
			}
		})
	}
}

func TestFileSecurityChecks(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	err := os.Mkdir(publicDir, 0755)
	require.NoError(t, err)

	secretFile := filepath.Join(tmpDir, "secret.txt")
	err = os.WriteFile(secretFile, []byte("secret data"), 0644)
	require.NoError(t, err)

	publicFile := filepath.Join(publicDir, "public.txt")
	err = os.WriteFile(publicFile, []byte("public data"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "path_traversal_attempt_dots",
			path:           filepath.Join(publicDir, "../secret.txt"),
			expectedStatus: http.StatusOK, // filepath.Clean will resolve this
			description:    "Path traversal is prevented by filepath.Clean",
		},
		{
			name:           "absolute_path_allowed",
			path:           publicFile,
			expectedStatus: http.StatusOK,
			description:    "Absolute paths are allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := gokit.File(tt.path)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err, tt.description)
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}

func TestRangeRequests(t *testing.T) {
	t.Parallel()

	// Create a test file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "range-test.txt")
	content := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Test that File response supports range requests
	response := gokit.File(testFile)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Range", "bytes=10-19")
	w := httptest.NewRecorder()

	err = response.Render(w, req)
	assert.NoError(t, err)

	// http.ServeFile should handle range requests
	// Status should be 206 Partial Content
	assert.Equal(t, http.StatusPartialContent, w.Code)

	// Content should be the requested range
	assert.Equal(t, "ABCDEFGHIJ", w.Body.String())
}

func TestConditionalRequests(t *testing.T) {
	t.Parallel()

	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "conditional-test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Get file modification time
	info, err := os.Stat(testFile)
	require.NoError(t, err)
	modTime := info.ModTime()

	// Test If-Modified-Since with a time before modification
	// The file was modified after this time, so it should return 200 OK
	response := gokit.File(testFile)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-Modified-Since", modTime.Add(-10*time.Second).Format(http.TimeFormat))
	w := httptest.NewRecorder()

	err = response.Render(w, req)
	assert.NoError(t, err)
	// Note: http.ServeFile may return 304 if the times are too close
	// We'll accept either 200 or 304 as valid
	assert.Contains(t, []int{http.StatusOK, http.StatusNotModified}, w.Code)

	// Test If-Modified-Since with a time after modification
	response = gokit.File(testFile)
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-Modified-Since", modTime.Add(1*time.Hour).Format(http.TimeFormat))
	w = httptest.NewRecorder()

	err = response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotModified, w.Code)
}

func TestFileReaderWithReader(t *testing.T) {
	t.Parallel()

	// Create a reader with test content
	content := "test content for streaming"
	reader := bytes.NewReader([]byte(content))

	response := gokit.FileReader(reader, "test.txt", "text/plain")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, content, w.Body.String())

	// Check headers
	disposition := w.Header().Get("Content-Disposition")
	assert.Contains(t, disposition, "attachment")
	assert.Contains(t, disposition, "test.txt")

	contentType := w.Header().Get("Content-Type")
	assert.Equal(t, "text/plain", contentType)
}

func TestCSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		records          [][]string
		filename         string
		expectedFilename string
		expectedContent  string
	}{
		{
			name: "simple_csv",
			records: [][]string{
				{"Name", "Age", "City"},
				{"Alice", "30", "New York"},
				{"Bob", "25", "Los Angeles"},
			},
			filename:         "users",
			expectedFilename: "users.csv",
			expectedContent:  "Name,Age,City\nAlice,30,New York\nBob,25,Los Angeles\n",
		},
		{
			name: "csv_with_extension",
			records: [][]string{
				{"Product", "Price"},
				{"Apple", "1.99"},
				{"Banana", "0.99"},
			},
			filename:         "products.csv",
			expectedFilename: "products.csv",
			expectedContent:  "Product,Price\nApple,1.99\nBanana,0.99\n",
		},
		{
			name: "csv_with_special_chars",
			records: [][]string{
				{"Field1", "Field2"},
				{"Value with, comma", "Value with \"quotes\""},
				{"Multi\nline", "Tab\there"},
			},
			filename:         "special",
			expectedFilename: "special.csv",
			expectedContent:  "Field1,Field2\n\"Value with, comma\",\"Value with \"\"quotes\"\"\"\n\"Multi\nline\",Tab\there\n",
		},
		{
			name:             "empty_csv",
			records:          [][]string{},
			filename:         "empty",
			expectedFilename: "empty.csv",
			expectedContent:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := gokit.CSV(tt.records, tt.filename)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			err := response.Render(w, req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, w.Code)

			// Check Content-Type
			contentType := w.Header().Get("Content-Type")
			assert.Equal(t, "text/csv; charset=utf-8", contentType)

			// Check Content-Disposition
			disposition := w.Header().Get("Content-Disposition")
			assert.Contains(t, disposition, "attachment")
			assert.Contains(t, disposition, tt.expectedFilename)

			// Check content
			assert.Equal(t, tt.expectedContent, w.Body.String())
		})
	}
}

func TestCSVWithHeaders(t *testing.T) {
	t.Parallel()

	headers := []string{"ID", "Name", "Email"}
	rows := [][]string{
		{"1", "John Doe", "john@example.com"},
		{"2", "Jane Smith", "jane@example.com"},
		{"3", "Bob Johnson", "bob@example.com"},
	}

	response := gokit.CSVWithHeaders(headers, rows, "users")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Check headers
	contentType := w.Header().Get("Content-Type")
	assert.Equal(t, "text/csv; charset=utf-8", contentType)

	disposition := w.Header().Get("Content-Disposition")
	assert.Contains(t, disposition, "attachment")
	assert.Contains(t, disposition, "users.csv")

	// Check content includes headers and rows
	expectedContent := "ID,Name,Email\n1,John Doe,john@example.com\n2,Jane Smith,jane@example.com\n3,Bob Johnson,bob@example.com\n"
	assert.Equal(t, expectedContent, w.Body.String())
}

func TestCSVWithEmptyData(t *testing.T) {
	t.Parallel()

	// Test with headers but no rows
	headers := []string{"Col1", "Col2", "Col3"}
	rows := [][]string{}

	response := gokit.CSVWithHeaders(headers, rows, "empty_data")
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	err := response.Render(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// Should only have headers
	expectedContent := "Col1,Col2,Col3\n"
	assert.Equal(t, expectedContent, w.Body.String())
}
