// Package s3 provides Amazon S3 and S3-compatible storage implementation.
//
// This package implements the storage.Storage interface using the AWS S3 SDK v2
// with support for Amazon S3, MinIO, DigitalOcean Spaces, Wasabi, and other
// S3-compatible services.
//
// Basic usage:
//
//	import (
//		"context"
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
//			AccessKeyID: "AKIA...", // Optional - uses IAM roles if empty
//			SecretKey:   "...",     // Optional - uses IAM roles if empty
//		}
//
//		// Create storage
//		storage, err := s3.New(ctx, cfg)
//		if err != nil {
//			panic(err)
//		}
//
//		// Use in HTTP handler
//		http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
//			_, header, err := r.FormFile("file")
//			if err != nil {
//				http.Error(w, "Bad request", 400)
//				return
//			}
//
//			file, err := storage.Save(ctx, header, "uploads/")
//			if err != nil {
//				http.Error(w, "Save failed", 500)
//				return
//			}
//
//			// Get public URL
//			url := storage.URL(file.RelativePath)
//			w.Header().Set("Location", url)
//		})
//	}
//
// # S3-Compatible Services
//
// MinIO configuration:
//
//	cfg := s3.S3Config{
//		Bucket:         "my-bucket",
//		Region:         "us-east-1", // Required
//		AccessKeyID:    "minioadmin",
//		SecretKey:      "minioadmin",
//		Endpoint:       "http://localhost:9000",
//		ForcePathStyle: true, // Required for MinIO
//	}
//
// DigitalOcean Spaces with CDN:
//
//	cfg := s3.S3Config{
//		Bucket:   "my-space",
//		Region:   "nyc3",
//		Endpoint: "https://nyc3.digitaloceanspaces.com",
//		BaseURL:  "https://my-space.nyc3.cdn.digitaloceanspaces.com",
//	}
//
// # Configuration Options
//
// Advanced configuration with functional options:
//
//	// Custom HTTP client
//	httpClient := &http.Client{Timeout: 30 * time.Second}
//	storage, err := s3.New(ctx, cfg, s3.WithHTTPClient(httpClient))
//
//	// Upload timeout
//	storage, err := s3.New(ctx, cfg, s3.WithS3UploadTimeout(5*time.Minute))
//
//	// Custom S3 client for testing
//	mockClient := &MockS3Client{}
//	storage, err := s3.New(ctx, cfg, s3.WithS3Client(mockClient))
package s3
