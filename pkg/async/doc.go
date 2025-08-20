// Package async provides utilities for asynchronous programming with Go generics.
//
// This package implements a Future pattern for non-blocking operations with timeout support
// and coordination utilities for managing multiple asynchronous computations.
//
// # Core Types
//
// Future[U] represents the result of an asynchronous computation. It provides methods
// to wait for completion (Await), check status without blocking (IsComplete), and
// handle timeouts (AwaitWithTimeout).
//
// # Usage
//
// Basic asynchronous operation:
//
//	func fetchUser(ctx context.Context, userID int) (User, error) {
//		// Simulate database call
//		time.Sleep(100 * time.Millisecond)
//		return User{ID: userID, Name: "John"}, nil
//	}
//
//	// Execute asynchronously
//	future := async.Async(ctx, 123, fetchUser)
//
//	// Do other work...
//
//	// Wait for result
//	user, err := future.Await()
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Using timeout:
//
//	user, err := future.AwaitWithTimeout(50 * time.Millisecond)
//	if errors.Is(err, async.ErrTimeout) {
//		log.Println("Operation timed out")
//	}
//
// # Coordination Utilities
//
// WaitAll waits for all futures to complete and returns their results:
//
//	futures := []*async.Future[User]{
//		async.Async(ctx, 1, fetchUser),
//		async.Async(ctx, 2, fetchUser),
//		async.Async(ctx, 3, fetchUser),
//	}
//
//	users, err := async.WaitAll(futures...)
//	if err != nil {
//		log.Printf("One or more operations failed: %v", err)
//	}
//
// WaitAny returns as soon as any future completes:
//
//	index, user, err := async.WaitAny(futures...)
//	log.Printf("Future %d completed first with result: %+v", index, user)
//
// # Error Handling
//
// The package defines two main errors:
//   - ErrTimeout: returned when AwaitWithTimeout exceeds its duration
//   - ErrNoFutures: returned when WaitAny is called with no futures
//
// # Concurrency Safety
//
// All operations are safe for concurrent use. The Future type uses sync.Once
// internally to prevent race conditions on completion.
//
// # Performance Considerations
//
// - Futures spawn exactly one goroutine per Async call
// - WaitAny spawns additional goroutines (one per future) for coordination
// - Context cancellation is checked before execution to prevent goroutine leaks
// - The package uses minimal allocations through careful struct design
//
// # Context Support
//
// All asynchronous operations respect context cancellation. If a context is
// cancelled before the async function begins execution, it returns immediately
// with the context's error.
package async
