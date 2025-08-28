// Package qrcode provides QR code generation utilities for web applications.
//
// The package generates PNG format QR codes with medium error correction and
// supports both raw PNG bytes and base64 data URIs for HTML embedding.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/pkg/qrcode"
//
//	// Generate PNG bytes
//	pngBytes, err := qrcode.Generate("https://example.com", 256)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Generate base64 data URI for HTML
//	dataURI, err := qrcode.GenerateBase64Image("https://example.com", 256)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf(`<img src="%s" alt="QR Code">`, dataURI)
//
// Size parameter controls the output dimensions in pixels. Use 0 or negative values
// to get the default size of 256 pixels.
package qrcode
