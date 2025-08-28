// Package cache provides thread-safe caching implementations with different eviction policies.
// It offers high-performance, generic cache implementations suitable for various use cases
// including web applications, data processing pipelines, and microservices.
//
// # Features
//
//   - Thread-safe operations with efficient locking
//   - Generic type parameters for compile-time type safety
//   - LRU (Least Recently Used) eviction policy
//   - Configurable capacity limits
//   - Optional eviction callbacks for resource cleanup
//   - Zero-allocation operations for cache hits
//   - Concurrent-safe design suitable for high-throughput applications
//
// # Usage
//
// The primary cache implementation is LRUCache, which provides automatic eviction
// of the least recently used items when capacity is reached:
//
//	import "github.com/dmitrymomot/foundation/core/cache"
//
//	// Create a cache with capacity of 100 items
//	c := cache.NewLRUCache[string, *User](100)
//
//	// Store values
//	c.Put("user:123", &User{ID: 123, Name: "John"})
//	c.Put("user:456", &User{ID: 456, Name: "Jane"})
//
//	// Retrieve values
//	if user, found := c.Get("user:123"); found {
//		fmt.Printf("Found user: %s\n", user.Name)
//	}
//
//	// Remove values
//	if user, found := c.Remove("user:123"); found {
//		fmt.Printf("Removed user: %s\n", user.Name)
//	}
//
// # LRU Cache
//
// The LRUCache implements the Least Recently Used eviction policy, automatically
// removing the oldest accessed items when the cache reaches capacity:
//
//	// Create cache for database query results
//	queryCache := cache.NewLRUCache[string, []byte](1000)
//
//	func getQueryResult(query string) ([]byte, error) {
//		// Check cache first
//		if result, found := queryCache.Get(query); found {
//			return result, nil
//		}
//
//		// Execute query
//		result, err := executeQuery(query)
//		if err != nil {
//			return nil, err
//		}
//
//		// Cache the result
//		queryCache.Put(query, result)
//		return result, nil
//	}
//
// # Eviction Callbacks
//
// Set up callbacks to handle resource cleanup when items are evicted:
//
//	type Connection struct {
//		ID   string
//		Conn net.Conn
//	}
//
//	connectionCache := cache.NewLRUCache[string, *Connection](50)
//
//	// Set up cleanup callback
//	connectionCache.SetEvictCallback(func(key string, conn *Connection) {
//		conn.Conn.Close()
//		fmt.Printf("Closed connection: %s\n", key)
//	})
//
//	// Add connections to cache
//	conn, err := net.Dial("tcp", "example.com:80")
//	if err == nil {
//		connectionCache.Put("conn:1", &Connection{
//			ID:   "conn:1",
//			Conn: conn,
//		})
//	}
//
// # Web Application Caching
//
// Use LRUCache for caching rendered templates, API responses, or computed results:
//
//	type PageData struct {
//		Content   string
//		Timestamp time.Time
//	}
//
//	pageCache := cache.NewLRUCache[string, *PageData](200)
//
//	func renderPage(path string) (*PageData, error) {
//		// Check cache first
//		if page, found := pageCache.Get(path); found {
//			// Check if still fresh (example: 5 minute TTL)
//			if time.Since(page.Timestamp) < 5*time.Minute {
//				return page, nil
//			}
//			// Remove stale entry
//			pageCache.Remove(path)
//		}
//
//		// Generate new page
//		content, err := generatePageContent(path)
//		if err != nil {
//			return nil, err
//		}
//
//		page := &PageData{
//			Content:   content,
//			Timestamp: time.Now(),
//		}
//
//		pageCache.Put(path, page)
//		return page, nil
//	}
//
// # Session Store
//
// Implement a simple session store using LRUCache:
//
//	type Session struct {
//		UserID    string
//		Data      map[string]any
//		ExpiresAt time.Time
//	}
//
//	sessionStore := cache.NewLRUCache[string, *Session](10000)
//
//	// Clean up expired sessions
//	sessionStore.SetEvictCallback(func(sessionID string, session *Session) {
//		fmt.Printf("Session %s evicted\n", sessionID)
//	})
//
//	func getSession(sessionID string) (*Session, bool) {
//		session, found := sessionStore.Get(sessionID)
//		if !found {
//			return nil, false
//		}
//
//		// Check expiration
//		if time.Now().After(session.ExpiresAt) {
//			sessionStore.Remove(sessionID)
//			return nil, false
//		}
//
//		return session, true
//	}
//
//	func createSession(userID string) string {
//		sessionID := generateSessionID()
//		session := &Session{
//			UserID:    userID,
//			Data:      make(map[string]any),
//			ExpiresAt: time.Now().Add(24 * time.Hour),
//		}
//		sessionStore.Put(sessionID, session)
//		return sessionID
//	}
//
// # Computed Results Cache
//
// Cache expensive computations to avoid redundant processing:
//
//	type ComputeKey struct {
//		Algorithm string
//		Input     string
//	}
//
//	resultCache := cache.NewLRUCache[ComputeKey, []byte](500)
//
//	func expensiveCompute(algorithm, input string) ([]byte, error) {
//		key := ComputeKey{Algorithm: algorithm, Input: input}
//
//		// Check cache
//		if result, found := resultCache.Get(key); found {
//			return result, nil
//		}
//
//		// Perform computation
//		result, err := performComputation(algorithm, input)
//		if err != nil {
//			return nil, err
//		}
//
//		// Cache result
//		resultCache.Put(key, result)
//		return result, nil
//	}
//
// # Thread Safety
//
// All cache operations are thread-safe and can be called concurrently
// from multiple goroutines without external synchronization:
//
//	cache := cache.NewLRUCache[int, string](100)
//
//	// Safe to call from multiple goroutines
//	go func() {
//		for i := 0; i < 1000; i++ {
//			cache.Put(i, fmt.Sprintf("value-%d", i))
//		}
//	}()
//
//	go func() {
//		for i := 0; i < 1000; i++ {
//			if value, found := cache.Get(i); found {
//				fmt.Println("Found:", value)
//			}
//		}
//	}()
//
// # Memory Management
//
// The cache automatically manages memory by evicting items when capacity
// is reached. Monitor cache performance and adjust capacity as needed:
//
//	cache := cache.NewLRUCache[string, []byte](1000)
//
//	// Check current size
//	fmt.Printf("Cache size: %d items\n", cache.Len())
//
//	// Clear all items when needed (e.g., during shutdown)
//	cache.Clear()
//
// # Performance Characteristics
//
// LRUCache provides efficient operations with the following complexity:
//
//   - Get: O(1) average case
//   - Put: O(1) average case
//   - Remove: O(1) average case
//   - Memory: O(capacity)
//
// The implementation uses a combination of hash map and doubly-linked list
// to achieve constant-time operations for all cache methods.
//
// # Best Practices
//
//   - Choose appropriate capacity based on memory constraints and access patterns
//   - Use eviction callbacks for resource cleanup (closing files, connections, etc.)
//   - Consider implementing TTL logic in your application layer for time-based expiration
//   - Monitor cache hit rates and adjust capacity accordingly
//   - Use meaningful key types that implement proper equality semantics
//   - Be mindful of memory usage, especially when caching large objects
//   - Consider using pointers for large structs to reduce copying overhead
package cache
