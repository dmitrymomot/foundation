// Package qrcode provides QR code generation utilities with base64 encoding support.
//
// This package generates PNG QR codes with configurable sizes and medium error correction
// level, making them suitable for web applications and mobile device scanning. It supports
// both raw PNG byte output and base64-encoded data URIs for direct HTML embedding.
//
// # Features
//
// - PNG format output with configurable dimensions
// - Medium error correction (balances data capacity with error recovery)
// - Base64 encoding with data URI format for web use
// - Input validation and error handling
// - Default sizing optimized for web and mobile scanning
//
// # Usage
//
// Generate raw PNG bytes:
//
//	content := "https://example.com"
//	size := 256 // pixels
//
//	pngBytes, err := qrcode.Generate(content, size)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Write to file
//	err = os.WriteFile("qrcode.png", pngBytes, 0644)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Generate base64 data URI for HTML:
//
//	dataURI, err := qrcode.GenerateBase64Image("https://example.com", 256)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use in HTML template
//	fmt.Printf(`<img src="%s" alt="QR Code">`, dataURI)
//
// HTTP handler example:
//
//	func qrHandler(w http.ResponseWriter, r *http.Request) {
//		url := r.URL.Query().Get("url")
//		if url == "" {
//			http.Error(w, "URL parameter required", http.StatusBadRequest)
//			return
//		}
//
//		pngBytes, err := qrcode.Generate(url, 256)
//		if err != nil {
//			http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
//			return
//		}
//
//		w.Header().Set("Content-Type", "image/png")
//		w.Header().Set("Cache-Control", "max-age=3600") // Cache for 1 hour
//		w.Write(pngBytes)
//	}
//
// # Error Correction Level
//
// The package uses Medium error correction level, which:
//   - Can recover from ~15% data corruption
//   - Balances data capacity with error resilience
//   - Suitable for most web and mobile scanning scenarios
//   - Handles typical printing and display quality variations
//
// # Size Recommendations
//
// Recommended sizes for different use cases:
//   - 128px: Small web icons, minimal data
//   - 256px: Standard web use (default), good mobile scanning
//   - 512px: High-quality printing, complex data
//   - 1024px: Large displays, maximum scan distance
//
// Mobile scanning considerations:
//   - Minimum 21x21 modules (varies by content length)
//   - 256px provides good balance for most smartphones
//   - Higher sizes improve scanning from greater distances
//
// # Performance Characteristics
//
// - Generation time: ~1-5ms for typical web content
// - Memory usage: ~O(size²) for image buffer
// - Base64 encoding adds ~33% to data size
// - PNG compression reduces file size significantly
//
// QR code complexity scales with content:
//   - URLs: Fast generation, moderate size
//   - Plain text: Fastest generation, smallest size
//   - JSON data: Slower generation, larger size
//   - Binary data: Slowest generation, largest size
//
// # Content Guidelines
//
// Optimal content for QR codes:
//   - URLs (most common use case)
//   - Contact information (vCard format)
//   - WiFi credentials
//   - Plain text messages (under 1KB)
//
// Content limitations:
//   - Maximum ~3KB with Medium error correction
//   - Avoid binary data when possible
//   - Use URL shorteners for long URLs
//   - Consider QR code scanning app compatibility
//
// # Integration Examples
//
// Template integration:
//
//	type PageData struct {
//		URL    string
//		QRCode string
//	}
//
//	func renderPage(w http.ResponseWriter, data PageData) {
//		qrCode, err := qrcode.GenerateBase64Image(data.URL, 256)
//		if err != nil {
//			// Handle error or use placeholder
//			qrCode = "data:image/svg+xml;base64,..." // placeholder
//		}
//		data.QRCode = qrCode
//
//		tmpl.Execute(w, data)
//	}
//
// API endpoint:
//
//	type QRRequest struct {
//		Content string `json:"content"`
//		Size    int    `json:"size,omitempty"`
//	}
//
//	func apiQRHandler(w http.ResponseWriter, r *http.Request) {
//		var req QRRequest
//		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//			http.Error(w, "Invalid JSON", http.StatusBadRequest)
//			return
//		}
//
//		dataURI, err := qrcode.GenerateBase64Image(req.Content, req.Size)
//		if err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//			return
//		}
//
//		json.NewEncoder(w).Encode(map[string]string{"qr_code": dataURI})
//	}
package qrcode
