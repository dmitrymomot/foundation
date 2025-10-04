package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"

	"github.com/dmitrymomot/foundation/pkg/clientip"
)

// Generate creates a device fingerprint from the HTTP request.
// Returns a version-prefixed fingerprint string in format: "v1:hash"
//
// By default, excludes IP address to avoid false positives from mobile networks and VPNs.
// Use functional options to customize behavior:
//
//	fp := fingerprint.Generate(r)  // uses defaults
//	fp := fingerprint.Generate(r, WithIP())  // include IP
//	fp := fingerprint.Generate(r, WithIP(), WithoutAcceptHeaders())  // multiple options
func Generate(r *http.Request, opts ...Option) string {
	o := applyOptions(opts...)

	var components []string

	if o.includeUserAgent {
		components = append(components, r.UserAgent())
	}

	if o.includeAcceptHeaders {
		components = append(components,
			r.Header.Get("Accept-Language"),
			r.Header.Get("Accept-Encoding"),
			r.Header.Get("Accept"),
		)
	}

	if o.includeIP {
		components = append(components, clientip.GetIP(r))
	}

	if o.includeHeaderSet {
		components = append(components, getHeaders(r))
	}

	// Filter out empty components
	filtered := make([]string, 0, len(components))
	for _, comp := range components {
		if comp != "" {
			filtered = append(filtered, comp)
		}
	}

	// Create SHA256 hash of all components
	combined := strings.Join(filtered, "|")
	hash := sha256.Sum256([]byte(combined))

	// Return version-prefixed fingerprint
	return "v1:" + hex.EncodeToString(hash[:16])
}

// Validate compares the current request fingerprint with a stored fingerprint.
// Returns nil if fingerprints match, or a specific error indicating which component changed.
//
// The stored fingerprint should be in format "v1:hash". Invalid formats return ErrInvalidFingerprint.
//
// To check which component changed, use errors.Is():
//
//	if err := Validate(r, storedFP); err != nil {
//	    if errors.Is(err, ErrIPMismatch) {
//	        // Handle IP change gracefully
//	    } else {
//	        // Potential session hijacking
//	    }
//	}
func Validate(r *http.Request, sessionFingerprint string) error {
	// Validate format
	if !strings.HasPrefix(sessionFingerprint, "v1:") || len(sessionFingerprint) != 35 {
		return ErrInvalidFingerprint
	}

	// Try with default options first (most common case)
	currentFingerprint := Generate(r)
	if currentFingerprint == sessionFingerprint {
		return nil
	}

	// Fingerprint mismatch - try to determine which component changed
	// Try with IP included in case the stored fingerprint was generated with IncludeIP: true
	if Generate(r, WithIP()) == sessionFingerprint {
		// Matches with IP - means stored fingerprint had IP and it still matches
		// This shouldn't happen as we already tried default, but keep for safety
		return nil
	}

	// We can't match the current request with stored fingerprint using any combination
	// Try to identify what component is different by process of elimination

	// Test without UserAgent
	fpNoUA := Generate(r, WithoutUserAgent())

	// Test without Accept headers
	fpNoAccept := Generate(r, WithoutAcceptHeaders())

	// Test without HeaderSet
	fpNoHeaderSet := Generate(r, WithoutHeaderSet())

	// If removing UA makes it match, UA was the problem
	if fpNoUA == sessionFingerprint {
		return ErrUserAgentMismatch
	}

	// If removing Accept headers makes it match, they were the problem
	if fpNoAccept == sessionFingerprint {
		return ErrHeadersMismatch
	}

	// If removing HeaderSet makes it match, it was the problem
	if fpNoHeaderSet == sessionFingerprint {
		return ErrHeaderSetMismatch
	}

	// If nothing matches, likely the stored fingerprint included IP and it changed
	// Or multiple components changed
	return ErrIPMismatch
}

// getHeaders creates a fingerprint based on which standard HTTP headers are present.
//
// This function fingerprints the *presence* of common browser headers, not their order.
// Different browsers and HTTP clients send different sets of headers, making this
// a useful signal for device identification:
//   - Chrome sends Sec-Fetch-* headers
//   - Firefox has different Accept defaults
//   - API clients typically send minimal headers
//   - Mobile browsers may omit certain headers
//
// The function filters for stable, commonly-present headers and ignores headers
// that frequently change (cookies, cache directives, etc.) to reduce false positives.
// Headers are sorted alphabetically to ensure consistent output for the same header set.
func getHeaders(r *http.Request) string {
	var headerNames []string
	for name := range r.Header {
		// Only include stable, commonly-present headers that are useful
		// for browser/client fingerprinting. Skip headers that vary
		// frequently or are added by proxies/CDNs.
		switch strings.ToLower(name) {
		case "user-agent", "accept", "accept-language", "accept-encoding",
			"connection", "upgrade-insecure-requests", "sec-fetch-dest",
			"sec-fetch-mode", "sec-fetch-site", "cache-control":
			headerNames = append(headerNames, strings.ToLower(name))
		}
	}

	// Sort to ensure consistent output for identical header sets
	sort.Strings(headerNames)
	return strings.Join(headerNames, ",")
}

// Strict generates a fingerprint with all components including IP address.
// Use for high-security scenarios where IP changes should invalidate sessions.
// WARNING: Will cause false positives for mobile users, VPN users, and users
// behind dynamic proxies.
func Strict(r *http.Request) string {
	return Generate(r, WithIP())
}

// Cookie generates a fingerprint suitable for cookie-based sessions.
// Excludes IP address to avoid false positives from mobile networks and VPNs.
// This is the recommended default for most web applications.
func Cookie(r *http.Request) string {
	return Generate(r) // Uses defaults
}

// JWT generates a minimal fingerprint suitable for JWT-based authentication.
// Includes only User-Agent and header set, excluding Accept headers which
// may vary with content negotiation.
func JWT(r *http.Request) string {
	return Generate(r, WithoutAcceptHeaders())
}
