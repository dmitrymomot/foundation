// Package storage provides file storage abstraction with support for local filesystem
// and cloud storage backends. It offers a unified interface for file operations
// including upload, download, deletion, and metadata management across different
// storage providers.
//
// # Features
//
//   - Unified interface for different storage backends
//   - Local filesystem storage for development
//   - Cloud storage support (extensible)
//   - File metadata management
//   - Path-based file organization
//   - Error handling with detailed error types
//   - Stream-based operations for large files
//   - Configurable storage options
//
// # Basic Usage
//
// Use the storage interface for file operations:
//
//	import "github.com/dmitrymomot/gokit/core/storage"
//
//	// Create local storage
//	store := storage.NewLocalStorage("./uploads")
//
//	// Upload file
//	file, err := os.Open("document.pdf")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer file.Close()
//
//	err = store.Put("documents/doc1.pdf", file)
//	if err != nil {
//		log.Fatal("Upload failed:", err)
//	}
//
//	// Download file
//	reader, err := store.Get("documents/doc1.pdf")
//	if err != nil {
//		log.Fatal("Download failed:", err)
//	}
//	defer reader.Close()
//
//	// Copy to destination
//	output, err := os.Create("downloaded.pdf")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer output.Close()
//
//	_, err = io.Copy(output, reader)
//	if err != nil {
//		log.Fatal("Copy failed:", err)
//	}
//
// # Local Storage
//
// Configure local filesystem storage:
//
//	// Basic local storage
//	store := storage.NewLocalStorage("/var/uploads")
//
//	// Local storage with options
//	store := storage.NewLocalStorage("/var/uploads",
//		storage.WithPermissions(0755),
//		storage.WithCreateDirs(true),
//	)
//
// # File Operations
//
// Perform various file operations:
//
//	// Check if file exists
//	exists, err := store.Exists("path/to/file.txt")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if exists {
//		// Get file info
//		info, err := store.Stat("path/to/file.txt")
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Printf("File size: %d bytes\n", info.Size)
//		fmt.Printf("Modified: %v\n", info.ModTime)
//	}
//
//	// Delete file
//	err = store.Delete("path/to/file.txt")
//	if err != nil {
//		log.Fatal("Delete failed:", err)
//	}
//
//	// List files in directory
//	files, err := store.List("documents/")
//	if err != nil {
//		log.Fatal("List failed:", err)
//	}
//
//	for _, file := range files {
//		fmt.Printf("File: %s, Size: %d\n", file.Name, file.Size)
//	}
//
// # File Upload Handler
//
// Create HTTP handler for file uploads:
//
//	func uploadHandler(store storage.Storage) http.HandlerFunc {
//		return func(w http.ResponseWriter, r *http.Request) {
//			file, header, err := r.FormFile("file")
//			if err != nil {
//				http.Error(w, "Failed to get file", http.StatusBadRequest)
//				return
//			}
//			defer file.Close()
//
//			// Validate file type
//			contentType := header.Header.Get("Content-Type")
//			if !isAllowedContentType(contentType) {
//				http.Error(w, "File type not allowed", http.StatusBadRequest)
//				return
//			}
//
//			// Generate unique filename
//			filename := generateUniqueFilename(header.Filename)
//			path := fmt.Sprintf("uploads/%s", filename)
//
//			// Store file
//			err = store.Put(path, file)
//			if err != nil {
//				log.Printf("Storage error: %v", err)
//				http.Error(w, "Failed to store file", http.StatusInternalServerError)
//				return
//			}
//
//			// Return success response
//			response := map[string]string{
//				"message": "File uploaded successfully",
//				"path":    path,
//			}
//			w.Header().Set("Content-Type", "application/json")
//			json.NewEncoder(w).Encode(response)
//		}
//	}
//
// # File Download Handler
//
// Create HTTP handler for file downloads:
//
//	func downloadHandler(store storage.Storage) http.HandlerFunc {
//		return func(w http.ResponseWriter, r *http.Request) {
//			path := r.URL.Query().Get("path")
//			if path == "" {
//				http.Error(w, "Path required", http.StatusBadRequest)
//				return
//			}
//
//			// Check if file exists
//			exists, err := store.Exists(path)
//			if err != nil {
//				log.Printf("Storage error: %v", err)
//				http.Error(w, "Storage error", http.StatusInternalServerError)
//				return
//			}
//
//			if !exists {
//				http.Error(w, "File not found", http.StatusNotFound)
//				return
//			}
//
//			// Get file info
//			info, err := store.Stat(path)
//			if err != nil {
//				log.Printf("Stat error: %v", err)
//				http.Error(w, "File info error", http.StatusInternalServerError)
//				return
//			}
//
//			// Get file reader
//			reader, err := store.Get(path)
//			if err != nil {
//				log.Printf("Get error: %v", err)
//				http.Error(w, "Failed to get file", http.StatusInternalServerError)
//				return
//			}
//			defer reader.Close()
//
//			// Set headers
//			filename := filepath.Base(path)
//			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
//			w.Header().Set("Content-Type", getContentType(filename))
//			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
//
//			// Stream file
//			_, err = io.Copy(w, reader)
//			if err != nil {
//				log.Printf("Stream error: %v", err)
//			}
//		}
//	}
//
// # Error Handling
//
// Handle storage-specific errors:
//
//	err := store.Put("path/to/file", reader)
//	if err != nil {
//		switch {
//		case errors.Is(err, storage.ErrFileNotFound):
//			// Handle file not found
//		case errors.Is(err, storage.ErrPermissionDenied):
//			// Handle permission error
//		case errors.Is(err, storage.ErrInsufficientSpace):
//			// Handle disk space error
//		default:
//			// Handle other errors
//		}
//	}
//
// # Custom Storage Backend
//
// Implement custom storage backend:
//
//	type S3Storage struct {
//		client *s3.Client
//		bucket string
//	}
//
//	func (s *S3Storage) Put(path string, reader io.Reader) error {
//		_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
//			Bucket: &s.bucket,
//			Key:    &path,
//			Body:   reader,
//		})
//		return err
//	}
//
//	func (s *S3Storage) Get(path string) (io.ReadCloser, error) {
//		result, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
//			Bucket: &s.bucket,
//			Key:    &path,
//		})
//		if err != nil {
//			return nil, err
//		}
//		return result.Body, nil
//	}
//
//	func (s *S3Storage) Delete(path string) error {
//		_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
//			Bucket: &s.bucket,
//			Key:    &path,
//		})
//		return err
//	}
//
//	func (s *S3Storage) Exists(path string) (bool, error) {
//		_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
//			Bucket: &s.bucket,
//			Key:    &path,
//		})
//		if err != nil {
//			var notFound *types.NotFound
//			if errors.As(err, &notFound) {
//				return false, nil
//			}
//			return false, err
//		}
//		return true, nil
//	}
//
// # Best Practices
//
//   - Validate file types and sizes before storage
//   - Use unique filenames to prevent conflicts
//   - Implement proper error handling for storage operations
//   - Set appropriate file permissions for security
//   - Clean up temporary files after processing
//   - Monitor storage usage and implement quotas
//   - Use streaming for large file operations
//   - Implement backup and recovery procedures
//   - Log storage operations for auditing
//   - Consider using CDN for public file access
package storage
