// Package s3 provides production-ready Amazon S3 and S3-compatible storage integration for file management in SaaS applications.
//
// This package implements the core storage.Storage interface using the AWS S3 SDK with support for both
// Amazon S3 and S3-compatible services like MinIO, Wasabi, and DigitalOcean Spaces. It provides proper
// error classification, security validation, and flexible URL generation for reliable file operations.
//
// # Key Features
//
// The package provides comprehensive S3 storage operations:
//
//   - New: Creates an S3 storage client with flexible configuration options
//   - Save: Stores multipart files with MIME type detection and path validation
//   - Delete: Removes individual files with existence verification
//   - DeleteDir: Batch deletes entire directories (up to 1000 objects per request)
//   - Exists: Checks file existence without downloading content
//   - List: Returns directory contents with proper file/directory distinction
//   - URL: Generates public URLs for files with support for CDNs and custom endpoints
//
// All operations include comprehensive error classification and security validation to prevent
// path traversal attacks and ensure reliable operation in production environments.
//
// # Configuration
//
// Basic configuration is handled through the S3Config struct:
//
//	type S3Config struct {
//		Bucket         string // S3 bucket name
//		Region         string // AWS region (e.g., "us-east-1")
//		AccessKeyID    string // AWS access key (optional, uses IAM roles if empty)
//		SecretKey      string // AWS secret key (optional, uses IAM roles if empty)
//		Endpoint       string // Custom endpoint for S3-compatible services
//		BaseURL        string // Custom CDN or public URL base
//		ForcePathStyle bool   // Required for MinIO and some S3-compatible services
//	}
//
// The configuration supports both AWS S3 and S3-compatible services with automatic URL generation
// and flexible authentication options.
//
// # Usage Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"mime/multipart"
//		"net/http"
//
//		"github.com/dmitrymomot/foundation/integration/storage/s3"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		// AWS S3 configuration
//		cfg := s3.S3Config{
//			Bucket:      "my-app-uploads",
//			Region:      "us-east-1",
//			AccessKeyID: "AKIA...", // Optional: uses IAM roles if empty
//			SecretKey:   "...",     // Optional: uses IAM roles if empty
//		}
//
//		// Create S3 storage client
//		storage, err := s3.New(ctx, cfg)
//		if err != nil {
//			log.Fatal("Failed to create S3 storage:", err)
//		}
//
//		// Handle file upload in HTTP handler
//		http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
//			file, header, err := r.FormFile("upload")
//			if err != nil {
//				http.Error(w, "Failed to read file", http.StatusBadRequest)
//				return
//			}
//			defer file.Close()
//
//			// Save file to S3
//			savedFile, err := storage.Save(ctx, header, "uploads/user-files/")
//			if err != nil {
//				http.Error(w, "Failed to save file", http.StatusInternalServerError)
//				return
//			}
//
//			// Get public URL for the file
//			publicURL := storage.URL(savedFile.RelativePath)
//			log.Printf("File saved: %s", publicURL)
//		})
//	}
//
// # S3-Compatible Services
//
// The package supports various S3-compatible services with appropriate configuration:
//
//	// MinIO configuration
//	cfg := s3.S3Config{
//		Bucket:         "my-bucket",
//		Region:         "us-east-1", // Required even for MinIO
//		AccessKeyID:    "minioadmin",
//		SecretKey:      "minioadmin",
//		Endpoint:       "http://localhost:9000",
//		ForcePathStyle: true, // Required for MinIO
//	}
//
//	// DigitalOcean Spaces configuration
//	cfg := s3.S3Config{
//		Bucket:      "my-space",
//		Region:      "nyc3",
//		AccessKeyID: "your-spaces-key",
//		SecretKey:   "your-spaces-secret",
//		Endpoint:    "https://nyc3.digitaloceanspaces.com",
//		BaseURL:     "https://my-space.nyc3.cdn.digitaloceanspaces.com", // CDN URL
//	}
//
//	// Wasabi configuration
//	cfg := s3.S3Config{
//		Bucket:      "my-wasabi-bucket",
//		Region:      "us-east-1",
//		AccessKeyID: "your-wasabi-key",
//		SecretKey:   "your-wasabi-secret",
//		Endpoint:    "https://s3.wasabisys.com",
//	}
//
// # Advanced Configuration Options
//
// The package provides several advanced configuration options through functional options:
//
//	// Custom HTTP client for proxy or timeout configuration
//	httpClient := &http.Client{Timeout: 30 * time.Second}
//	storage, err := s3.New(ctx, cfg, s3.WithHTTPClient(httpClient))
//
//	// Upload timeout for large file operations
//	storage, err := s3.New(ctx, cfg, s3.WithS3UploadTimeout(5*time.Minute))
//
//	// Custom S3 client options
//	storage, err := s3.New(ctx, cfg, s3.WithS3ClientOption(func(o *s3aws.Options) {
//		o.UsePathStyle = true
//	}))
//
// These options allow fine-tuning for specific deployment requirements or performance needs.
//
// # File Operations
//
// The package provides comprehensive file management operations:
//
//	// Save a file from multipart upload
//	savedFile, err := storage.Save(ctx, fileHeader, "uploads/documents/")
//	if err != nil {
//		log.Printf("Save failed: %v", err)
//	}
//
//	// Check if a file exists
//	exists := storage.Exists(ctx, "uploads/documents/file.pdf")
//
//	// List directory contents
//	entries, err := storage.List(ctx, "uploads/documents/")
//	for _, entry := range entries {
//		if entry.IsDir {
//			log.Printf("Directory: %s", entry.Name)
//		} else {
//			log.Printf("File: %s (%d bytes)", entry.Name, entry.Size)
//		}
//	}
//
//	// Delete a single file
//	err = storage.Delete(ctx, "uploads/documents/old-file.pdf")
//
//	// Delete an entire directory
//	err = storage.DeleteDir(ctx, "uploads/temp/")
//
// # URL Generation
//
// The package provides flexible URL generation for different deployment scenarios:
//
//	// Basic S3 URL
//	url := storage.URL("uploads/image.jpg")
//	// Returns: https://bucket.s3.region.amazonaws.com/uploads/image.jpg
//
//	// With custom base URL (CDN)
//	cfg.BaseURL = "https://cdn.example.com"
//	url := storage.URL("uploads/image.jpg")
//	// Returns: https://cdn.example.com/uploads/image.jpg
//
//	// S3-compatible service with custom endpoint
//	cfg.Endpoint = "https://s3.example.com"
//	url := storage.URL("uploads/image.jpg")
//	// Returns: https://bucket.s3.example.com/uploads/image.jpg (or path-style if ForcePathStyle=true)
//
// # Error Handling
//
// The package provides comprehensive error classification for different failure scenarios:
//
//	if err := storage.Save(ctx, fileHeader, path); err != nil {
//		switch {
//		case errors.Is(err, storage.ErrFileNotFound):
//			log.Error("File not found")
//		case errors.Is(err, storage.ErrAccessDenied):
//			log.Error("Insufficient permissions")
//		case errors.Is(err, storage.ErrInvalidPath):
//			log.Error("Invalid file path")
//		case errors.Is(err, storage.ErrOperationTimeout):
//			log.Error("Operation timed out")
//		case errors.Is(err, storage.ErrServiceUnavailable):
//			log.Error("S3 service temporarily unavailable - retry recommended")
//		default:
//			log.Error("Unexpected storage error", "error", err)
//		}
//	}
//
// This error classification enables appropriate retry logic and user-facing error messages.
//
// # Security and Path Validation
//
// The package includes comprehensive security validation:
//
//   - Path traversal prevention: Automatically blocks ".." in file paths
//   - Filename sanitization: Uses storage.SanitizeFilename for safe file names
//   - MIME type detection: Automatically sets Content-Type headers for proper browser handling
//   - Input validation: Validates all inputs to prevent S3 key injection attacks
//
// All file operations include these security checks by default, ensuring safe operation
// even with untrusted user input.
//
// # Performance Considerations
//
// For high-performance applications:
//
//   - Use batch operations (DeleteDir) for multiple file operations
//   - Configure appropriate upload timeouts for large files
//   - Consider using S3 Transfer Acceleration for global deployments
//   - Implement proper caching strategies for frequently accessed files
//   - Use CDNs (configure BaseURL) for better global performance
//
// The package is optimized for typical SaaS file operations with reasonable defaults
// for most use cases.
//
// # Testing Support
//
// The package includes comprehensive testing support through dependency injection:
//
//	// Use custom S3 client for testing
//	mockClient := &MockS3Client{} // Implement S3Client interface
//	storage, err := s3.New(ctx, cfg, s3.WithS3Client(mockClient))
//
//	// Custom paginator for testing list operations
//	mockPaginator := &MockPaginator{} // Implement S3ListObjectsV2Paginator interface
//	storage, err := s3.New(ctx, cfg, s3.WithPaginatorFactory(func(client S3Client, params *s3aws.ListObjectsV2Input) S3ListObjectsV2Paginator {
//		return mockPaginator
//	}))
//
// This design enables comprehensive unit testing without requiring actual S3 infrastructure.
//
// # Production Deployment
//
// For production S3 deployments:
//
//   - Use IAM roles instead of hardcoded credentials when possible
//   - Configure proper bucket policies and CORS settings
//   - Enable S3 server-side encryption for sensitive data
//   - Set up appropriate lifecycle policies for cost optimization
//   - Monitor S3 costs and usage patterns
//   - Configure CloudFront or another CDN for better performance
//   - Implement proper backup and disaster recovery procedures
//
// The package provides the foundation for reliable file storage, but consider these additional
// infrastructure components for a complete production storage solution.
package s3
