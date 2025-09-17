package letsencrypt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Storage provides low-level certificate file operations.
type Storage struct {
	dir string
}

// NewStorage creates a new certificate storage handler.
func NewStorage(dir string) (*Storage, error) {
	// Ensure directory exists
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
			// Skip special files like acme_account+key
			name := entry.Name()
			if name != "" && !strings.Contains(name, "+") && !strings.Contains(name, "_") {
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

	// Write to temporary file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write certificate for %s: %w", domain, err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to save certificate for %s: %w", domain, err)
	}

	return nil
}

// Copy copies a certificate from one domain to another.
func (s *Storage) Copy(srcDomain, dstDomain string) error {
	srcPath := filepath.Join(s.dir, srcDomain)
	dstPath := filepath.Join(s.dir, dstDomain)

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source certificate: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination certificate: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy certificate: %w", err)
	}

	return dst.Chmod(0600)
}

// Dir returns the storage directory path.
func (s *Storage) Dir() string {
	return s.dir
}
