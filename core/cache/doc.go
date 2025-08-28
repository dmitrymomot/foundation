// Package cache provides a thread-safe LRU cache implementation.
//
// The package offers a single generic cache type that automatically evicts
// the least recently used items when capacity is reached.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/core/cache"
//
//	// Create a cache with capacity of 100 items
//	c := cache.NewLRUCache[string, int](100)
//
//	// Store values
//	c.Put("key1", 42)
//	c.Put("key2", 84)
//
//	// Retrieve values
//	if value, found := c.Get("key1"); found {
//		fmt.Printf("Found: %d\n", value)
//	}
//
//	// Remove values
//	if value, removed := c.Remove("key1"); removed {
//		fmt.Printf("Removed: %d\n", value)
//	}
//
//	// Check size and clear cache
//	fmt.Printf("Cache size: %d\n", c.Len())
//	c.Clear()
//
// The Put method returns the previous value if the key existed:
//
//	// Update existing key
//	oldValue, existed := c.Put("key1", 100)
//	if existed {
//		fmt.Printf("Previous value was: %d\n", oldValue)
//	}
//
// # Eviction Callbacks
//
// Set up callbacks to handle resource cleanup when items are evicted:
//
//	c := cache.NewLRUCache[string, *os.File](10)
//
//	// Clean up files when evicted
//	c.SetEvictCallback(func(key string, file *os.File) {
//		file.Close()
//		fmt.Printf("Closed file: %s\n", key)
//	})
//
// The eviction callback is also triggered when items are manually removed
// or when the cache is cleared.
//
// # Thread Safety
//
// All cache operations are thread-safe and can be called concurrently
// from multiple goroutines without additional synchronization.
//
// # Performance
//
// LRUCache provides O(1) average-case performance for all operations
// (Get, Put, Remove) using a combination of hash map and doubly-linked list.
package cache
