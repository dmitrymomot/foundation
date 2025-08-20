// Package clientip extracts real client IP addresses from HTTP requests.
//
// This package handles various proxy headers in priority order to determine the
// actual client IP address, which is essential for rate limiting, geolocation,
// and security logging in web applications behind proxies, load balancers, or CDNs.
//
// # Header Priority
//
// The package checks headers in this specific order:
//  1. CF-Connecting-IP (Cloudflare)
//  2. DO-Connecting-IP (DigitalOcean)
//  3. X-Forwarded-For (most common proxy header)
//  4. X-Real-IP (nginx and other proxies)
//  5. RemoteAddr (direct connection)
//
// This priority order ensures that the most reliable sources are checked first.
//
// # Usage
//
// Basic IP extraction:
//
//	func handleRequest(w http.ResponseWriter, r *http.Request) {
//		clientIP := clientip.GetIP(r)
//		log.Printf("Request from IP: %s", clientIP)
//
//		// Use for rate limiting
//		if isRateLimited(clientIP) {
//			http.Error(w, "Rate limited", http.StatusTooManyRequests)
//			return
//		}
//
//		// Continue processing...
//	}
//
// # Validation and Security
//
// All IP addresses are validated and normalized:
//   - Invalid IP strings are rejected
//   - IPv6 addresses are properly handled
//   - The special address 0.0.0.0 is rejected (indicates no valid client IP)
//   - All IPs are normalized using Go's net.IP.String() method
//
// X-Forwarded-For handling:
//
//	// X-Forwarded-For may contain multiple IPs: "client, proxy1, proxy2"
//	// The package correctly extracts the leftmost (original client) IP
//	// and validates it before returning
//
// # IPv6 Support
//
// The package fully supports IPv6 addresses and handles them correctly
// in all proxy headers:
//
//	// Examples of supported IPv6 formats:
//	// 2001:db8::1
//	// ::1 (localhost)
//	// ::ffff:192.0.2.1 (IPv4-mapped IPv6)
//
// # Error Handling
//
// The function never panics and always returns a string:
//   - If no valid IP can be determined, returns the raw RemoteAddr
//   - Malformed headers are silently skipped
//   - Network parsing errors are handled gracefully
//
// # Performance Considerations
//
// - Header lookup is O(1) for each header checked
// - IP validation uses Go's optimized net.ParseIP function
// - Early exit on first valid IP found reduces processing time
// - No memory allocations beyond what's required by net.ParseIP
//
// # Common Use Cases
//
// Rate limiting by IP:
//
//	limiter := ratelimit.NewIPLimiter()
//	if !limiter.Allow(clientip.GetIP(r)) {
//		http.Error(w, "Rate limited", 429)
//		return
//	}
//
// Geolocation services:
//
//	ip := clientip.GetIP(r)
//	location, err := geoService.Lookup(ip)
//	if err == nil {
//		log.Printf("Request from %s, %s", location.City, location.Country)
//	}
//
// Security logging:
//
//	logger.WithField("client_ip", clientip.GetIP(r)).
//		WithField("user_agent", r.UserAgent()).
//		Info("Authentication attempt")
//
// # Proxy Configuration
//
// When deploying behind proxies, ensure they set the appropriate headers:
//   - Nginx: proxy_set_header X-Real-IP $remote_addr;
//   - Apache: RequestHeader set X-Forwarded-For %h
//   - Cloudflare: Automatically sets CF-Connecting-IP
//   - DigitalOcean Load Balancer: Automatically sets DO-Connecting-IP
package clientip
