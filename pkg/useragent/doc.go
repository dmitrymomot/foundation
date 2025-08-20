// Package useragent provides comprehensive User-Agent string parsing for device detection.
//
// This package extracts browser, operating system, device type and model information with
// optimized bot detection and human-readable session identifiers for analytics, logging,
// and personalization. It's designed to handle the complexities of modern User-Agent strings
// including mobile devices, bots, and edge cases.
//
// # Features
//
// - Comprehensive browser detection (Chrome, Firefox, Safari, Edge, etc.)
// - Operating system identification (Windows, macOS, iOS, Android, Linux)
// - Device type classification (mobile, desktop, tablet, bot, TV, console)
// - Device model extraction (iPhone, iPad, specific Android models)
// - Optimized bot detection with fast-path lookups
// - Human-readable session identifiers for logging
// - Performance-optimized parsing with minimal allocations
//
// # Usage
//
// Basic User-Agent parsing:
//
//	userAgentString := r.Header.Get("User-Agent")
//	ua, err := useragent.Parse(userAgentString)
//	if err != nil {
//		log.Printf("Failed to parse User-Agent: %v", err)
//		return
//	}
//
//	fmt.Printf("Device: %s\n", ua.DeviceType())      // "mobile"
//	fmt.Printf("OS: %s\n", ua.OS())                  // "iOS"
//	fmt.Printf("Browser: %s\n", ua.BrowserName())    // "Safari"
//	fmt.Printf("Version: %s\n", ua.BrowserVer())     // "14.2"
//	fmt.Printf("Model: %s\n", ua.DeviceModel())      // "iPhone"
//
//	// Convenient boolean methods
//	if ua.IsMobile() {
//		fmt.Println("Mobile device detected")
//	}
//
//	if ua.IsBot() {
//		fmt.Printf("Bot detected: %s\n", ua.GetShortIdentifier())
//	}
//
// # Device Type Detection
//
// The package categorizes devices into specific types:
//
//	ua, _ := useragent.Parse(userAgentString)
//
//	switch ua.DeviceType() {
//	case useragent.DeviceTypeMobile:
//		// Handle mobile-specific logic
//		serveCompactUI()
//	case useragent.DeviceTypeDesktop:
//		// Handle desktop-specific logic
//		serveFullUI()
//	case useragent.DeviceTypeTablet:
//		// Handle tablet-specific logic
//		serveTabletUI()
//	case useragent.DeviceTypeBot:
//		// Handle bot requests
//		serveAPIResponse()
//	case useragent.DeviceTypeTV:
//		// Handle smart TV requests
//		serveTVUI()
//	default:
//		// Handle unknown devices
//		serveDefaultUI()
//	}
//
// # HTTP Integration
//
// Middleware for automatic User-Agent parsing:
//
//	func UserAgentMiddleware(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			userAgentHeader := r.Header.Get("User-Agent")
//			ua, err := useragent.Parse(userAgentHeader)
//			if err != nil {
//				// Continue with unknown device info
//				ua = useragent.New(userAgentHeader, "unknown", "", "unknown", "unknown", "")
//			}
//
//			// Add parsed info to request context
//			ctx := context.WithValue(r.Context(), "useragent", ua)
//
//			// Add headers for debugging/analytics
//			w.Header().Set("X-Device-Type", ua.DeviceType())
//			w.Header().Set("X-Browser", ua.BrowserName())
//			w.Header().Set("X-OS", ua.OS())
//
//			next.ServeHTTP(w, r.WithContext(ctx))
//		})
//	}
//
// Content negotiation based on device:
//
//	func handleHomePage(w http.ResponseWriter, r *http.Request) {
//		ua := r.Context().Value("useragent").(useragent.UserAgent)
//
//		if ua.IsMobile() {
//			// Serve mobile-optimized content
//			http.ServeFile(w, r, "templates/mobile/home.html")
//		} else if ua.IsTablet() {
//			// Serve tablet-optimized content
//			http.ServeFile(w, r, "templates/tablet/home.html")
//		} else {
//			// Serve desktop content
//			http.ServeFile(w, r, "templates/desktop/home.html")
//		}
//	}
//
// # Bot Detection
//
// The package provides sophisticated bot detection:
//
//	ua, _ := useragent.Parse(userAgentString)
//
//	if ua.IsBot() {
//		// Handle bot traffic
//		log.Printf("Bot request: %s", ua.GetShortIdentifier())
//
//		// Serve different content or rate limit
//		if isSearchBot(ua) {
//			serveSearchOptimizedContent(w, r)
//		} else if isSocialBot(ua) {
//			serveSocialMetaTags(w, r)
//		} else {
//			serveAPIResponse(w, r)
//		}
//		return
//	}
//
// Common bot identification:
//
//	func identifyBotType(ua useragent.UserAgent) string {
//		if !ua.IsBot() {
//			return ""
//		}
//
//		shortID := ua.GetShortIdentifier()
//
//		if strings.Contains(shortID, "Googlebot") {
//			return "search_engine"
//		} else if strings.Contains(shortID, "Facebook") {
//			return "social_media"
//		} else if strings.Contains(shortID, "Twitterbot") {
//			return "social_media"
//		} else if strings.Contains(shortID, "bot") {
//			return "generic_bot"
//		}
//
//		return "unknown_bot"
//	}
//
// # Analytics and Logging
//
// Session identification for analytics:
//
//	func logRequest(r *http.Request) {
//		ua, err := useragent.Parse(r.Header.Get("User-Agent"))
//		if err != nil {
//			log.Printf("Request from unknown device: %s", r.RemoteAddr)
//			return
//		}
//
//		// Create readable session identifier
//		sessionID := ua.GetShortIdentifier()
//
//		log.Printf("Request: %s | Device: %s | IP: %s | Path: %s",
//			sessionID, ua.DeviceType(), r.RemoteAddr, r.URL.Path)
//	}
//
// Analytics data collection:
//
//	type RequestMetrics struct {
//		Timestamp   time.Time `json:"timestamp"`
//		DeviceType  string    `json:"device_type"`
//		OS          string    `json:"os"`
//		Browser     string    `json:"browser"`
//		BrowserVer  string    `json:"browser_version"`
//		DeviceModel string    `json:"device_model,omitempty"`
//		IsBot       bool      `json:"is_bot"`
//		Path        string    `json:"path"`
//		UserAgent   string    `json:"user_agent,omitempty"`
//	}
//
//	func collectMetrics(r *http.Request) {
//		ua, err := useragent.Parse(r.Header.Get("User-Agent"))
//		if err != nil {
//			return // Skip invalid User-Agents
//		}
//
//		metrics := RequestMetrics{
//			Timestamp:   time.Now(),
//			DeviceType:  ua.DeviceType(),
//			OS:          ua.OS(),
//			Browser:     ua.BrowserName(),
//			BrowserVer:  ua.BrowserVer(),
//			DeviceModel: ua.DeviceModel(),
//			IsBot:       ua.IsBot(),
//			Path:        r.URL.Path,
//		}
//
//		// Only store full UA for debugging purposes
//		if shouldLogFullUA(ua) {
//			metrics.UserAgent = ua.String()
//		}
//
//		sendToAnalytics(metrics)
//	}
//
// # Mobile-Specific Features
//
// Mobile detection and optimization:
//
//	func optimizeForMobile(w http.ResponseWriter, r *http.Request) {
//		ua, _ := useragent.Parse(r.Header.Get("User-Agent"))
//
//		if ua.IsMobile() {
//			// Set mobile-specific headers
//			w.Header().Set("Vary", "User-Agent")
//			w.Header().Set("Cache-Control", "public, max-age=3600")
//
//			// Different handling for specific mobile OS
//			switch ua.OS() {
//			case "iOS":
//				// iOS-specific optimizations
//				w.Header().Set("X-iOS-Version", extractiOSVersion(ua))
//			case "Android":
//				// Android-specific optimizations
//				w.Header().Set("X-Android-Version", extractAndroidVersion(ua))
//			}
//
//			// Load mobile assets
//			serveCompactCSS(w)
//			serveWebPImages(w)
//		}
//	}
//
// Device-specific feature detection:
//
//	func enableFeatures(ua useragent.UserAgent) map[string]bool {
//		features := map[string]bool{
//			"push_notifications": false,
//			"offline_support":    false,
//			"camera_access":      false,
//			"geolocation":        false,
//		}
//
//		if ua.IsMobile() {
//			features["push_notifications"] = true
//			features["camera_access"] = true
//			features["geolocation"] = true
//
//			// iOS-specific features
//			if ua.OS() == "iOS" && isRecentiOS(ua.BrowserVer()) {
//				features["offline_support"] = true
//			}
//		}
//
//		if ua.IsDesktop() {
//			features["offline_support"] = true
//			// Desktop doesn't typically have camera/GPS access
//		}
//
//		return features
//	}
//
// # Performance Considerations
//
// Parsing performance:
//   - User-Agent parsing: ~10-100 µs per request
//   - Bot detection: ~5-20 µs (optimized with fast-path lookups)
//   - Memory usage: ~1-2 KB per parsed UserAgent struct
//   - Regex compilation is done once at package initialization
//
// Optimization strategies:
//   - Cache parsed results for identical User-Agent strings
//   - Use middleware to parse once per request
//   - Consider storing parsed data in sessions for repeat visitors
//
// Caching example:
//
//	var uaCache = sync.Map{} // string -> UserAgent
//
//	func parseWithCache(uaString string) (useragent.UserAgent, error) {
//		if cached, ok := uaCache.Load(uaString); ok {
//			return cached.(useragent.UserAgent), nil
//		}
//
//		ua, err := useragent.Parse(uaString)
//		if err != nil {
//			return ua, err
//		}
//
//		uaCache.Store(uaString, ua)
//		return ua, nil
//	}
//
// # Error Handling
//
// The package defines specific error types:
//   - ErrEmptyUserAgent: User-Agent string is empty
//   - ErrUnknownDevice: Device type could not be determined
//   - ErrMalformedUserAgent: User-Agent format is invalid
//
// Graceful error handling:
//
//	func handleUserAgent(uaString string) {
//		ua, err := useragent.Parse(uaString)
//		if err != nil {
//			switch {
//			case errors.Is(err, useragent.ErrEmptyUserAgent):
//				log.Println("Request without User-Agent header")
//				// Use default device assumptions
//			case errors.Is(err, useragent.ErrUnknownDevice):
//				log.Printf("Unknown device: %s", uaString)
//				// Continue with partial information
//			case errors.Is(err, useragent.ErrMalformedUserAgent):
//				log.Printf("Malformed User-Agent: %s", uaString)
//				// Log for investigation
//			default:
//				log.Printf("User-Agent parsing error: %v", err)
//			}
//
//			// Create fallback UserAgent
//			ua = useragent.New(uaString, "unknown", "", "unknown", "unknown", "")
//		}
//
//		// Continue processing with ua
//		processRequest(ua)
//	}
//
// # Testing Support
//
// The package provides utilities for testing:
//
//	func TestUserAgentParsing(t *testing.T) {
//		testCases := []struct {
//			ua       string
//			expected useragent.UserAgent
//		}{
//			{
//				ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_2 like Mac OS X) AppleWebKit/605.1.15",
//				expected: useragent.New("...", "mobile", "iPhone", "iOS", "Safari", "14.2"),
//			},
//			// Add more test cases
//		}
//
//		for _, tc := range testCases {
//			parsed, err := useragent.Parse(tc.ua)
//			assert.NoError(t, err)
//			assert.Equal(t, tc.expected.DeviceType(), parsed.DeviceType())
//			assert.Equal(t, tc.expected.OS(), parsed.OS())
//		}
//	}
//
// # Browser Support
//
// The package recognizes major browsers and their variants:
//   - Chrome (including Chromium-based browsers)
//   - Firefox (including mobile versions)
//   - Safari (macOS and iOS)
//   - Edge (legacy and Chromium-based)
//   - Opera (including mobile)
//   - Internet Explorer
//   - Various mobile browsers (Samsung Internet, etc.)
//
// # Operating System Support
//
// Supported operating systems:
//   - Windows (all versions)
//   - macOS
//   - iOS (iPhone and iPad)
//   - Android (all versions)
//   - Linux distributions
//   - Chrome OS
//   - Various embedded and IoT systems
//
// # Security Considerations
//
// User-Agent spoofing:
//   - User-Agent strings can be easily spoofed
//   - Use for UX optimization, not security decisions
//   - Combine with other detection methods for critical features
//   - Log unusual patterns for investigation
//
// Privacy considerations:
//   - User-Agent strings can contain identifying information
//   - Consider data retention policies
//   - Be transparent about data collection
//   - Hash or anonymize for long-term storage
package useragent
