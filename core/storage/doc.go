// Package storage provides local filesystem storage for handling multipart file uploads
// with security features including path traversal protection, MIME type validation,
// and file type checking. Designed for web applications that need secure file storage.
//
// # Basic Usage
//
// Create storage and save files from multipart uploads:
//
//	import (
//		"context"
//		"log"
//		"net/http"
//
//		"github.com/dmitrymomot/foundation/core/storage"
//	)
//
//	func uploadHandler(w http.ResponseWriter, r *http.Request) {
//		// Create local storage
//		store, err := storage.New("/var/uploads", "/files/")
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Parse multipart form
//		fh, err := r.FormFile("file")
//		if err != nil {
//			http.Error(w, "Failed to get file", http.StatusBadRequest)
//			return
//		}
//		defer fh.Close()
//
//		// Validate file (optional)
//		if err := storage.ValidateSize(fh, 5<<20); err != nil { // 5MB limit
//			http.Error(w, "File too large", http.StatusBadRequest)
//			return
//		}
//
//		// Save file
//		file, err := store.Save(r.Context(), fh, "uploads/document.pdf")
//		if err != nil {
//			log.Printf("Save error: %v", err)
//			http.Error(w, "Failed to save file", http.StatusInternalServerError)
//			return
//		}
//
//		// File saved successfully
//		log.Printf("Saved %s (%d bytes) to %s",
//			file.Filename, file.Size, file.RelativePath)
//	}
//
// # Storage Operations
//
// The Storage interface provides file management operations:
//
//	store, _ := storage.New("/uploads", "/files/")
//	ctx := context.Background()
//
//	// Check if file exists
//	if store.Exists(ctx, "documents/report.pdf") {
//		log.Println("File exists")
//	}
//
//	// List directory contents
//	entries, err := store.List(ctx, "documents/")
//	if err != nil {
//		log.Fatal(err)
//	}
//	for _, entry := range entries {
//		if entry.IsDir {
//			log.Printf("Directory: %s", entry.Name)
//		} else {
//			log.Printf("File: %s (%d bytes)", entry.Name, entry.Size)
//		}
//	}
//
//	// Delete a file
//	err = store.Delete(ctx, "documents/old.pdf")
//	if err != nil {
//		log.Printf("Delete failed: %v", err)
//	}
//
//	// Delete entire directory
//	err = store.DeleteDir(ctx, "temp/")
//	if err != nil {
//		log.Printf("DeleteDir failed: %v", err)
//	}
//
//	// Get public URL for file
//	url := store.URL("documents/report.pdf")
//	log.Printf("File URL: %s", url) // "/files/documents/report.pdf"
//
// # File Type Validation
//
// Built-in functions for validating uploaded files:
//
//	// Check file types
//	if storage.IsImage(fh) {
//		log.Println("Image file detected")
//	}
//	if storage.IsVideo(fh) {
//		log.Println("Video file detected")
//	}
//	if storage.IsPDF(fh) {
//		log.Println("PDF file detected")
//	}
//
//	// Validate MIME types (prevents spoofing)
//	err := storage.ValidateMIMEType(fh, "image/jpeg", "image/png")
//	if err != nil {
//		log.Printf("Invalid file type: %v", err)
//		return
//	}
//
//	// Get file extension and MIME type
//	ext := storage.GetExtension(fh)           // ".jpg"
//	mimeType, _ := storage.GetMIMEType(fh)    // "image/jpeg"
//
// # File Processing
//
// Additional utilities for file handling:
//
//	// Read entire file content (use cautiously with large files)
//	data, err := storage.ReadAll(fh)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Calculate file hash for integrity/deduplication
//	hash, err := storage.Hash(fh, nil) // Uses SHA256 by default
//	if err != nil {
//		log.Fatal(err)
//	}
//	log.Printf("File hash: %s", hash)
//
//	// Sanitize filename to prevent path traversal attacks
//	safe := storage.SanitizeFilename("../../../etc/passwd") // Returns "passwd"
//
// # Configuration Options
//
// Configure storage with options:
//
//	store, err := storage.New("/uploads", "/files/",
//		storage.WithLocalUploadTimeout(30*time.Second),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Error Handling
//
// Handle specific storage errors:
//
//	file, err := store.Save(ctx, fh, "documents/file.pdf")
//	if err != nil {
//		switch {
//		case errors.Is(err, storage.ErrFileNotFound):
//			log.Println("File not found")
//		case errors.Is(err, storage.ErrInvalidPath):
//			log.Println("Invalid file path (possible attack)")
//		case errors.Is(err, storage.ErrFileTooLarge):
//			log.Println("File exceeds size limit")
//		case errors.Is(err, storage.ErrMIMETypeNotAllowed):
//			log.Println("File type not allowed")
//		default:
//			log.Printf("Storage error: %v", err)
//		}
//	}
//
// # Security Features
//
// The package includes built-in security protections:
//
//   - Path traversal prevention (automatically blocks ../ attacks)
//   - MIME type detection based on file content (not just extensions)
//   - Filename sanitization to remove dangerous characters
//   - File size validation to prevent DoS attacks
//   - Context cancellation support for timeouts
//   - Restrictive file permissions (644 for files, 755 for directories)
package storage
