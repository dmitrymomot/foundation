package letsencrypt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// acmeAccountKeyMarker is used by autocert to mark account key files.
	acmeAccountKeyMarker = "+"
	// acmeMetadataMarker is used by autocert to mark metadata files.
	acmeMetadataMarker = "_"
)

// Storage provides low-level certificate file operations.
type Storage struct {
	dir string
}

// NewStorage creates a new certificate storage handler.
func NewStorage(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certificate directory: %w", err)
	}

	return &Storage{dir: dir}, nil
}

// List returns all certificate file names (domains) in the storage directory.
func (s *Storage) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	var domains []string
	for _, entry := range entries {
		if !entry.IsDir() {
			// Exclude autocert metadata files (contain + or _)
			name := entry.Name()
			if name != "" && !strings.Contains(name, acmeAccountKeyMarker) && !strings.Contains(name, acmeMetadataMarker) {
				domains = append(domains, name)
			}
		}
	}

	return domains, nil
}

// Exists checks if a certificate file exists for the domain.
func (s *Storage) Exists(domain string) bool {
	path := filepath.Join(s.dir, domain)
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes the certificate file for a domain.
func (s *Storage) Delete(domain string) error {
	path := filepath.Join(s.dir, domain)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete certificate for %s: %w", domain, err)
	}
	return nil
}

// Read reads the certificate data for a domain.
func (s *Storage) Read(domain string) ([]byte, error) {
	path := filepath.Join(s.dir, domain)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate for %s: %w", domain, err)
	}
	return data, nil
}

// Write writes certificate data for a domain.
func (s *Storage) Write(domain string, data []byte) error {
	path := filepath.Join(s.dir, domain)

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write certificate for %s: %w", domain, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Best effort cleanup
		return fmt.Errorf("failed to save certificate for %s: %w", domain, err)
	}

	return nil
}

// Copy copies a certificate from one domain to another using atomic operations.
func (s *Storage) Copy(srcDomain, dstDomain string) error {
	srcPath := filepath.Join(s.dir, srcDomain)
	dstPath := filepath.Join(s.dir, dstDomain)
	tmpPath := dstPath + ".tmp"

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source certificate: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary destination: %w", err)
	}

	// Ensure cleanup on error
	defer func() {
		if dst != nil {
			_ = dst.Close()
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to copy certificate: %w", err)
	}

	if err := dst.Chmod(0600); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := dst.Sync(); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to sync data: %w", err)
	}

	// Close before rename
	if err := dst.Close(); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to close destination: %w", err)
	}
	dst = nil // Mark as closed

	// Atomic rename
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to finalize copy: %w", err)
	}

	return nil
}

// Dir returns the storage directory path.
func (s *Storage) Dir() string {
	return s.dir
}
