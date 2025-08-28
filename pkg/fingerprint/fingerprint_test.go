package fingerprint_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/pkg/fingerprint"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	t.Run("generates consistent fingerprint for same request", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			"Accept":          "text/html,application/xhtml+xml",
			"Accept-Language": "en-US,en;q=0.9",
			"Accept-Encoding": "gzip, deflate, br",
		}, "192.168.1.100:54321")

		fp1 := fingerprint.Generate(req)
		fp2 := fingerprint.Generate(req)

		assert.Equal(t, fp1, fp2, "fingerprints should be consistent")
		assert.Len(t, fp1, 32, "fingerprint should be 32 characters")
		assert.Regexp(t, "^[a-f0-9]{32}$", fp1, "fingerprint should be hex string")
	})

	t.Run("generates different fingerprints for different user agents", func(t *testing.T) {
		t.Parallel()
		req1 := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			"Accept":     "text/html",
		}, "192.168.1.100:54321")

		req2 := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			"Accept":     "text/html",
		}, "192.168.1.100:54321")

		fp1 := fingerprint.Generate(req1)
		fp2 := fingerprint.Generate(req2)

		assert.NotEqual(t, fp1, fp2, "different user agents should produce different fingerprints")
	})

	t.Run("generates different fingerprints for different IPs", func(t *testing.T) {
		t.Parallel()
		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			"Accept":     "text/html",
		}

		req1 := createTestRequest(headers, "192.168.1.100:54321")
		req2 := createTestRequest(headers, "192.168.1.101:54321")

		fp1 := fingerprint.Generate(req1)
		fp2 := fingerprint.Generate(req2)

		assert.NotEqual(t, fp1, fp2, "different IPs should produce different fingerprints")
	})

	t.Run("generates different fingerprints for different accept headers", func(t *testing.T) {
		t.Parallel()
		req1 := createTestRequest(map[string]string{
			"User-Agent":      "Mozilla/5.0",
			"Accept":          "text/html",
			"Accept-Language": "en-US",
			"Accept-Encoding": "gzip",
		}, "192.168.1.100:54321")

		req2 := createTestRequest(map[string]string{
			"User-Agent":      "Mozilla/5.0",
			"Accept":          "application/json",
			"Accept-Language": "fr-FR",
			"Accept-Encoding": "deflate",
		}, "192.168.1.100:54321")

		fp1 := fingerprint.Generate(req1)
		fp2 := fingerprint.Generate(req2)

		assert.NotEqual(t, fp1, fp2, "different accept headers should produce different fingerprints")
	})

	t.Run("handles missing headers gracefully", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent": "TestBot/1.0",
		}, "192.168.1.100:54321")

		fp := fingerprint.Generate(req)
		require.NotEmpty(t, fp)
		assert.Len(t, fp, 32)
	})

	t.Run("handles empty request", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{}, "127.0.0.1:8080")

		fp := fingerprint.Generate(req)
		require.NotEmpty(t, fp)
		assert.Len(t, fp, 32)
	})

	t.Run("includes header order in fingerprint", func(t *testing.T) {
		t.Parallel()
		// Different header sets should produce different fingerprints
		req1 := createTestRequest(map[string]string{
			"User-Agent":                "Mozilla/5.0",
			"Accept":                    "text/html",
			"Connection":                "keep-alive",
			"Upgrade-Insecure-Requests": "1",
		}, "192.168.1.100:54321")

		req2 := createTestRequest(map[string]string{
			"User-Agent":     "Mozilla/5.0",
			"Accept":         "text/html",
			"Cache-Control":  "no-cache",
			"Sec-Fetch-Mode": "navigate",
		}, "192.168.1.100:54321")

		fp1 := fingerprint.Generate(req1)
		fp2 := fingerprint.Generate(req2)

		assert.NotEqual(t, fp1, fp2, "different header sets should produce different fingerprints")
	})

	t.Run("uses client IP from headers when available", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent":       "Mozilla/5.0",
			"CF-Connecting-IP": "203.0.113.195",
		}, "192.168.1.100:54321")

		fp := fingerprint.Generate(req)
		require.NotEmpty(t, fp)
		assert.Len(t, fp, 32)

		// Same request without CF header should produce different fingerprint
		req2 := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0",
		}, "192.168.1.100:54321")

		fp2 := fingerprint.Generate(req2)
		assert.NotEqual(t, fp, fp2, "different client IPs should produce different fingerprints")
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()
	t.Run("validates matching fingerprints", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			"Accept":          "text/html",
			"Accept-Language": "en-US",
		}, "192.168.1.100:54321")

		storedFingerprint := fingerprint.Generate(req)
		isValid := fingerprint.Validate(req, storedFingerprint)

		assert.True(t, isValid, "should validate matching fingerprints")
	})

	t.Run("rejects non-matching fingerprints", func(t *testing.T) {
		t.Parallel()
		req1 := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
		}, "192.168.1.100:54321")

		req2 := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
		}, "192.168.1.100:54321")

		storedFingerprint := fingerprint.Generate(req1)
		isValid := fingerprint.Validate(req2, storedFingerprint)

		assert.False(t, isValid, "should reject non-matching fingerprints")
	})

	t.Run("rejects invalid stored fingerprint", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0",
		}, "192.168.1.100:54321")

		isValid := fingerprint.Validate(req, "invalid-fingerprint")
		assert.False(t, isValid, "should reject invalid fingerprint format")
	})

	t.Run("rejects empty stored fingerprint", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent": "Mozilla/5.0",
		}, "192.168.1.100:54321")

		isValid := fingerprint.Validate(req, "")
		assert.False(t, isValid, "should reject empty fingerprint")
	})
}

func TestFingerprintConsistency(t *testing.T) {
	t.Parallel()
	t.Run("produces consistent fingerprints across multiple calls", func(t *testing.T) {
		t.Parallel()
		req := createTestRequest(map[string]string{
			"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			"Accept":          "text/html,application/xhtml+xml",
			"Accept-Language": "en-US,en;q=0.9",
			"Accept-Encoding": "gzip, deflate, br",
			"Connection":      "keep-alive",
		}, "192.168.1.100:54321")

		fingerprints := make(map[string]bool)
		for range 100 {
			fp := fingerprint.Generate(req)
			fingerprints[fp] = true
		}

		assert.Len(t, fingerprints, 1, "should produce only one unique fingerprint for identical requests")
	})
}

func TestFingerprintUniqueness(t *testing.T) {
	t.Parallel()
	t.Run("generates unique fingerprints for different clients", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name    string
			headers map[string]string
			ip      string
		}{
			{
				name: "Chrome on Mac",
				headers: map[string]string{
					"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
					"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9",
					"Accept-Language": "en-US,en;q=0.9",
					"Accept-Encoding": "gzip, deflate, br",
				},
				ip: "192.168.1.100:54321",
			},
			{
				name: "Firefox on Windows",
				headers: map[string]string{
					"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:91.0) Gecko/20100101",
					"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
					"Accept-Language": "en-US,en;q=0.5",
					"Accept-Encoding": "gzip, deflate",
				},
				ip: "192.168.1.101:54321",
			},
			{
				name: "Safari on iOS",
				headers: map[string]string{
					"User-Agent":      "Mozilla/5.0 (iPhone; CPU iPhone OS 14_7_1 like Mac OS X)",
					"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
					"Accept-Language": "en-us",
					"Accept-Encoding": "gzip, deflate",
				},
				ip: "192.168.1.102:54321",
			},
			{
				name: "API Client",
				headers: map[string]string{
					"User-Agent": "MyApp/1.0",
					"Accept":     "application/json",
				},
				ip: "192.168.1.103:54321",
			},
		}

		fingerprints := make(map[string]string)
		for _, tc := range testCases {
			req := createTestRequest(tc.headers, tc.ip)
			fp := fingerprint.Generate(req)

			// Check for collisions
			if existing, exists := fingerprints[fp]; exists {
				t.Errorf("Fingerprint collision: %s and %s produced same fingerprint %s",
					existing, tc.name, fp)
			}
			fingerprints[fp] = tc.name
		}

		assert.Len(t, fingerprints, len(testCases), "each client should have unique fingerprint")
	})
}

func BenchmarkGenerate(b *testing.B) {
	req := createTestRequest(map[string]string{
		"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":           "en-US,en;q=0.9",
		"Accept-Encoding":           "gzip, deflate, br",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Cache-Control":             "max-age=0",
	}, "192.168.1.100:54321")

	b.ResetTimer()
	for b.Loop() {
		fingerprint.Generate(req)
	}
}

func BenchmarkValidate(b *testing.B) {
	req := createTestRequest(map[string]string{
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
		"Accept":          "text/html",
		"Accept-Language": "en-US",
		"Accept-Encoding": "gzip, deflate",
	}, "192.168.1.100:54321")

	storedFingerprint := fingerprint.Generate(req)

	b.ResetTimer()
	for b.Loop() {
		fingerprint.Validate(req, storedFingerprint)
	}
}

func BenchmarkGenerateMinimalHeaders(b *testing.B) {
	req := createTestRequest(map[string]string{
		"User-Agent": "TestBot/1.0",
	}, "127.0.0.1:8080")

	b.ResetTimer()
	for b.Loop() {
		fingerprint.Generate(req)
	}
}

// Helper function to create test requests
func createTestRequest(headers map[string]string, remoteAddr string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = remoteAddr

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req
}
