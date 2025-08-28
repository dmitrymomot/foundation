// Package broadcast provides a generic pub/sub messaging system with pluggable backends.
//
// This package supports in-memory broadcasting with automatic cleanup and non-blocking
// message delivery to prevent slow consumers from affecting the entire system.
//
// # Architecture
//
// The package defines two main interfaces:
//   - Broadcaster: sends messages to multiple subscribers
//   - Subscriber: receives broadcast messages
//
// The design allows for pluggable backends (Redis, NATS, etc.) while providing
// a consistent API. Currently includes an in-memory implementation.
//
// # Usage
//
// Basic broadcasting:
//
//	// Create a broadcaster with buffer size of 100 messages per subscriber
//	broadcaster := broadcast.NewMemoryBroadcaster[string](100)
//	defer broadcaster.Close()
//
//	// Subscribe to messages
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	subscriber := broadcaster.Subscribe(ctx)
//	defer subscriber.Close()
//
//	// Start receiving messages in a goroutine
//	go func() {
//		for msg := range subscriber.Receive(ctx) {
//			fmt.Printf("Received: %s\n", msg.Data)
//		}
//	}()
//
//	// Send messages
//	broadcaster.Broadcast(ctx, broadcast.Message[string]{Data: "Hello, World!"})
//	broadcaster.Broadcast(ctx, broadcast.Message[string]{Data: "Another message"})
//
// # Memory Implementation
//
// MemoryBroadcaster provides an in-memory implementation with these characteristics:
//   - Non-blocking message delivery
//   - Automatic subscriber cleanup on context cancellation
//   - Graceful handling of slow consumers
//   - Thread-safe operations
//
// Slow Consumer Handling:
//
//	// If a subscriber's buffer is full, messages are dropped for that subscriber
//	// rather than blocking the broadcast operation. This prevents slow consumers
//	// from affecting other subscribers or blocking the broadcaster.
//
// # Message Types
//
// Messages are wrapped in a generic Message[T] struct:
//
//	type Message[T any] struct {
//		Data T
//	}
//
// This allows type-safe broadcasting of any data type.
//
// # Context Integration
//
// Subscriptions are automatically cleaned up when their context is cancelled:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	subscriber := broadcaster.Subscribe(ctx)
//	// Subscription will be automatically cleaned up after 30 seconds
//
// # Performance Characteristics
//
// - Message delivery is O(n) where n is the number of active subscribers
// - Subscriber cleanup is performed asynchronously to avoid blocking broadcasts
// - Buffer sizes should be chosen based on expected message rates and processing speed
// - Recommended buffer sizes: 10-100 for low-volume, 100-1000 for high-volume
//
// # Error Handling
//
// The package defines two errors for future extensibility:
//   - ErrBroadcasterClosed: for indicating closed broadcaster state
//   - ErrSubscriberClosed: for indicating closed subscriber state
//
// Operations on closed resources are safe and will not panic. The current
// in-memory implementation returns nil for all operations rather than these
// specific errors, but they are available for custom implementations.
//
// # Thread Safety
//
// All types in this package are safe for concurrent use across multiple goroutines.
// The MemoryBroadcaster uses read-write mutexes to optimize for read-heavy broadcast
// operations while handling less frequent subscription changes.
package broadcast
