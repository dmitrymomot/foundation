package s3

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3aws "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dmitrymomot/gokit/core/storage"
)

// Compile-time check that S3Storage implements storage.Storage interface
var _ storage.Storage = (*S3Storage)(nil)

// S3Client defines the interface for S3 operations used by S3Storage.
type S3Client interface {
	PutObject(ctx context.Context, params *s3aws.PutObjectInput, optFns ...func(*s3aws.Options)) (*s3aws.PutObjectOutput, error)
	HeadObject(ctx context.Context, params *s3aws.HeadObjectInput, optFns ...func(*s3aws.Options)) (*s3aws.HeadObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3aws.ListObjectsV2Input, optFns ...func(*s3aws.Options)) (*s3aws.ListObjectsV2Output, error)
	DeleteObject(ctx context.Context, params *s3aws.DeleteObjectInput, optFns ...func(*s3aws.Options)) (*s3aws.DeleteObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3aws.DeleteObjectsInput, optFns ...func(*s3aws.Options)) (*s3aws.DeleteObjectsOutput, error)
}

// S3ListObjectsV2Paginator defines the interface for paginated list operations.
type S3ListObjectsV2Paginator interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*s3aws.Options)) (*s3aws.ListObjectsV2Output, error)
}

// S3Storage implements Storage interface for Amazon S3 and S3-compatible services.
// Thread-safe with automatic retry and error classification for reliable operation.
type S3Storage struct {
	client           S3Client
	bucket           string
	region           string                                                                           // AWS region for URL generation
	endpoint         string                                                                           // Custom endpoint for S3-compatible services
	baseURL          string                                                                           // Custom CDN or public URL base (if provided)
	forcePathStyle   bool                                                                             // Required for MinIO and some S3-compatible services
	uploadTimeout    time.Duration                                                                    // Optional timeout to prevent hanging uploads
	paginatorFactory func(client S3Client, params *s3aws.ListObjectsV2Input) S3ListObjectsV2Paginator // Testable pagination
}

// S3Config contains configuration for S3 storage.
type S3Config struct {
	Bucket         string
	Region         string
	AccessKeyID    string
	SecretKey      string
	Endpoint       string // For S3-compatible services like MinIO, Wasabi
	BaseURL        string // Custom CDN or public URL base (auto-generated if empty)
	ForcePathStyle bool   // Required for MinIO and some S3-compatible services
}

// S3Option defines a function that configures S3Storage.
type S3Option func(*s3Options)

// s3Options contains additional configuration options.
type s3Options struct {
	httpClient       *http.Client
	s3Client         S3Client
	s3ConfigOptions  []func(*config.LoadOptions) error
	s3ClientOptions  []func(*s3aws.Options)
	paginatorFactory func(client S3Client, params *s3aws.ListObjectsV2Input) S3ListObjectsV2Paginator
	uploadTimeout    time.Duration
}

// WithS3Client sets a custom pre-configured S3 client.
// Primarily used for testing with mocks, but also allows advanced client customization.
func WithS3Client(client S3Client) S3Option {
	return func(o *s3Options) {
		o.s3Client = client
	}
}

// WithHTTPClient sets a custom HTTP client for S3 requests.
// Useful for custom timeout, proxy, or TLS configuration.
func WithHTTPClient(client *http.Client) S3Option {
	return func(o *s3Options) {
		o.httpClient = client
	}
}

// WithS3ConfigOption adds a custom AWS config option.
func WithS3ConfigOption(option func(*config.LoadOptions) error) S3Option {
	return func(o *s3Options) {
		o.s3ConfigOptions = append(o.s3ConfigOptions, option)
	}
}

// WithS3ClientOption adds a custom S3 client option.
func WithS3ClientOption(option func(*s3aws.Options)) S3Option {
	return func(o *s3Options) {
		o.s3ClientOptions = append(o.s3ClientOptions, option)
	}
}

// WithPaginatorFactory sets a custom paginator factory.
// Essential for testing pagination behavior with mock clients.
func WithPaginatorFactory(factory func(client S3Client, params *s3aws.ListObjectsV2Input) S3ListObjectsV2Paginator) S3Option {
	return func(o *s3Options) {
		o.paginatorFactory = factory
	}
}

// WithS3UploadTimeout sets the timeout for upload operations.
// Prevents hanging uploads from consuming resources indefinitely.
// If not set, relies on context deadline from caller.
func WithS3UploadTimeout(timeout time.Duration) S3Option {
	return func(o *s3Options) {
		o.uploadTimeout = timeout
	}
}

// New creates a new S3 storage instance.
// Auto-generates baseURL if not provided, supports both AWS S3 and S3-compatible services.
func New(ctx context.Context, cfg S3Config, opts ...S3Option) (*S3Storage, error) {
	if cfg.Bucket == "" || cfg.Region == "" {
		return nil, storage.ErrInvalidConfig
	}

	options := &s3Options{}
	for _, opt := range opts {
		opt(options)
	}

	// Use provided client or create a new one
	var client S3Client
	if options.s3Client != nil {
		client = options.s3Client
	} else {
		awsOptions := []func(*config.LoadOptions) error{
			config.WithRegion(cfg.Region),
		}

		// Add static credentials if provided (fallback to IAM roles/env vars otherwise)
		if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
			awsOptions = append(awsOptions,
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
					cfg.AccessKeyID,
					cfg.SecretKey,
					"",
				)),
			)
		}

		if options.httpClient != nil {
			awsOptions = append(awsOptions, config.WithHTTPClient(options.httpClient))
		}

		awsOptions = append(awsOptions, options.s3ConfigOptions...)

		awsConfig, err := config.LoadDefaultConfig(ctx, awsOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %v", err)
		}

		client = s3aws.NewFromConfig(awsConfig, func(o *s3aws.Options) {
			if cfg.Endpoint != "" {
				o.BaseEndpoint = aws.String(cfg.Endpoint)
			}
			o.UsePathStyle = cfg.ForcePathStyle

			for _, opt := range options.s3ClientOptions {
				opt(o)
			}
		})
	}

	// Store configuration for URL generation
	baseURL := cfg.BaseURL

	// Default paginator factory with type checking for real vs mock clients
	paginatorFactory := options.paginatorFactory
	if paginatorFactory == nil {
		paginatorFactory = func(c S3Client, params *s3aws.ListObjectsV2Input) S3ListObjectsV2Paginator {
			if realClient, ok := c.(*s3aws.Client); ok {
				return s3aws.NewListObjectsV2Paginator(realClient, params)
			}
			// Mock clients must provide their own paginator via WithPaginatorFactory
			return nil
		}
	}

	return &S3Storage{
		client:           client,
		bucket:           cfg.Bucket,
		region:           cfg.Region,
		endpoint:         cfg.Endpoint,
		baseURL:          baseURL,
		forcePathStyle:   cfg.ForcePathStyle,
		uploadTimeout:    options.uploadTimeout,
		paginatorFactory: paginatorFactory,
	}, nil
}

// Save stores a file to S3.
// Validates path to prevent S3 key injection attacks and sets proper Content-Type.
func (s *S3Storage) Save(ctx context.Context, fh *multipart.FileHeader, path string) (*storage.File, error) {
	if s.uploadTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.uploadTimeout)
		defer cancel()
	}

	if fh == nil {
		return nil, storage.ErrNilFileHeader
	}

	filename := storage.SanitizeFilename(fh.Filename)

	// S3 key validation - prevent path traversal in object keys
	path = strings.TrimPrefix(path, "/")
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("%w: %s", storage.ErrInvalidPath, path)
	}

	src, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", storage.ErrFailedToOpenFile, err)
	}
	defer func() { _ = src.Close() }()

	mimeType, err := storage.GetMIMEType(fh)
	if err != nil {
		mimeType = "application/octet-stream" // Safe fallback for unknown types
	}

	_, err = s.client.PutObject(ctx, &s3aws.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        src,
		ContentType: aws.String(mimeType), // Important for proper browser handling
	})
	if err != nil {
		return nil, classifyS3Error(err, "upload file")
	}

	return &storage.File{
		Filename:     filename,
		Size:         fh.Size, // S3 handles size tracking during upload
		MIMEType:     mimeType,
		Extension:    storage.GetExtension(fh),
		AbsolutePath: "", // Not applicable for S3 (URLs are generated)
		RelativePath: path,
	}, nil
}

// Delete removes a single file from S3.
// Verifies existence before deletion to provide consistent error handling.
func (s *S3Storage) Delete(ctx context.Context, path string) error {
	path = strings.TrimPrefix(path, "/")
	if strings.Contains(path, "..") {
		return fmt.Errorf("%w: %s", storage.ErrInvalidPath, path)
	}

	// Check if object exists first for consistent error handling
	_, err := s.client.HeadObject(ctx, &s3aws.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return classifyS3Error(err, "check file")
	}

	_, err = s.client.DeleteObject(ctx, &s3aws.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return classifyS3Error(err, "delete file")
	}

	return nil
}

// DeleteDir removes all objects with the given prefix from S3.
// Uses batch deletion (1000 objects per request) for efficiency on large directories.
func (s *S3Storage) DeleteDir(ctx context.Context, dir string) error {
	dir = strings.TrimPrefix(dir, "/")
	if strings.Contains(dir, "..") {
		return fmt.Errorf("%w: %s", storage.ErrInvalidPath, dir)
	}

	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	paginator := s.paginatorFactory(s.client, &s3aws.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(dir),
	})
	if paginator == nil {
		return storage.ErrPaginatorNil
	}

	var objects []types.ObjectIdentifier
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return classifyS3Error(err, "list directory")
		}

		for _, obj := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	if len(objects) == 0 {
		return fmt.Errorf("%w: %s", storage.ErrDirectoryNotFound, dir)
	}

	// Batch delete in chunks of 1000 (S3 API limit)
	const batchSize = 1000
	for i := range (len(objects) + batchSize - 1) / batchSize {
		start := i * batchSize
		end := min(start+batchSize, len(objects))
		_, err := s.client.DeleteObjects(ctx, &s3aws.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{
				Objects: objects[start:end],
			},
		})
		if err != nil {
			return classifyS3Error(err, "delete directory")
		}
	}

	return nil
}

// Exists checks if an object exists in S3.
func (s *S3Storage) Exists(ctx context.Context, path string) bool {
	path = strings.TrimPrefix(path, "/")
	if strings.Contains(path, "..") {
		return false
	}

	_, err := s.client.HeadObject(ctx, &s3aws.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	return err == nil
}

// List returns all entries in a directory (non-recursive).
// Uses S3 delimiter to simulate directory structure and avoid deep recursion.
func (s *S3Storage) List(ctx context.Context, dir string) ([]storage.Entry, error) {
	dir = strings.TrimPrefix(dir, "/")
	if strings.Contains(dir, "..") {
		return nil, fmt.Errorf("%w: %s", storage.ErrInvalidPath, dir)
	}

	prefix := dir
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Use delimiter to get only immediate children, not deep recursion
	resp, err := s.client.ListObjectsV2(ctx, &s3aws.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"), // Critical for directory-like behavior
	})
	if err != nil {
		return nil, classifyS3Error(err, "list directory")
	}

	var entries []storage.Entry

	// CommonPrefixes represent "subdirectories"
	for _, commonPrefix := range resp.CommonPrefixes {
		name := strings.TrimPrefix(*commonPrefix.Prefix, prefix)
		name = strings.TrimSuffix(name, "/")
		entries = append(entries, storage.Entry{
			Name:  name,
			Path:  *commonPrefix.Prefix,
			IsDir: true,
			Size:  0,
		})
	}

	// Contents represent actual files at this level
	for _, obj := range resp.Contents {
		if *obj.Key == prefix {
			continue // Skip directory marker itself
		}
		name := strings.TrimPrefix(*obj.Key, prefix)
		if !strings.Contains(name, "/") { // Only immediate children
			entries = append(entries, storage.Entry{
				Name:  name,
				Path:  *obj.Key,
				IsDir: false,
				Size:  *obj.Size,
			})
		}
	}

	return entries, nil
}

// URL returns the public URL for a file.
// Generates the appropriate URL format based on configuration:
// - Custom BaseURL: uses the provided base URL
// - S3-compatible (with Endpoint): path-style or virtual-hosted-style based on ForcePathStyle
// - AWS S3: standard AWS URL format (path-style or virtual-hosted-style)
func (s *S3Storage) URL(path string) string {
	path = strings.TrimPrefix(path, "/")

	// Case 1: Custom base URL provided (e.g., CDN)
	if s.baseURL != "" {
		// Ensure exactly one slash between base URL and path
		base := strings.TrimSuffix(s.baseURL, "/")
		return base + "/" + path
	}

	// Case 2: S3-compatible service with custom endpoint
	if s.endpoint != "" {
		endpoint := strings.TrimSuffix(s.endpoint, "/")
		// Remove protocol if present for cleaner manipulation
		protocol := "https://"
		if after, ok := strings.CutPrefix(endpoint, "http://"); ok {
			protocol = "http://"
			endpoint = after
		} else if after, ok := strings.CutPrefix(endpoint, "https://"); ok {
			endpoint = after
		}

		if s.forcePathStyle {
			// Path-style: {endpoint}/{bucket}/{path}
			return fmt.Sprintf("%s%s/%s/%s", protocol, endpoint, s.bucket, path)
		}
		// Virtual-hosted-style: {bucket}.{endpoint}/{path}
		return fmt.Sprintf("%s%s.%s/%s", protocol, s.bucket, endpoint, path)
	}

	// Case 3: AWS S3
	if s.forcePathStyle {
		// Path-style: https://s3.{region}.amazonaws.com/{bucket}/{path}
		return fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", s.region, s.bucket, path)
	}
	// Virtual-hosted-style: https://{bucket}.s3.{region}.amazonaws.com/{path}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, path)
}
