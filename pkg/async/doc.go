// Package async provides utilities for asynchronous programming with Go generics.
//
// This package implements a Future pattern that allows executing functions
// asynchronously with support for waiting, timeouts, and coordination utilities
// for managing multiple concurrent operations.
//
// Basic usage:
//
//	import (
//		"context"
//		"fmt"
//		"log"
//		"time"
//
//		"github.com/dmitrymomot/foundation/pkg/async"
//	)
//
//	func fetchUserData(ctx context.Context, userID int) (string, error) {
//		// Simulate database call
//		time.Sleep(100 * time.Millisecond)
//		return fmt.Sprintf("User data for ID %d", userID), nil
//	}
//
//	func main() {
//		ctx := context.Background()
//
//		// Execute function asynchronously
//		future := async.Async(ctx, 123, fetchUserData)
//
//		// Do other work while function executes...
//
//		// Wait for result
//		result, err := future.Await()
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Println(result) // "User data for ID 123"
//	}
//
// # Timeout handling
//
//	// Wait with timeout
//	result, err := future.AwaitWithTimeout(50 * time.Millisecond)
//	if errors.Is(err, async.ErrTimeout) {
//		log.Println("Operation timed out")
//	}
//
//	// Check completion without blocking
//	if future.IsComplete() {
//		result, err := future.Await() // Returns immediately
//	}
//
// # Coordinating multiple futures
//
// WaitAll waits for all futures to complete and returns all results:
//
//	futures := []*async.Future[string]{
//		async.Async(ctx, 1, fetchUserData),
//		async.Async(ctx, 2, fetchUserData),
//		async.Async(ctx, 3, fetchUserData),
//	}
//
//	results, err := async.WaitAll(futures...)
//	if err != nil {
//		// First error encountered stops and returns
//		log.Printf("One operation failed: %v", err)
//	}
//
// WaitAny returns as soon as the first future completes:
//
//	index, result, err := async.WaitAny(futures...)
//	fmt.Printf("Future %d completed first with: %s", index, result)
//
// # Error handling
//
// The package defines standard errors:
//   - ErrTimeout: returned when AwaitWithTimeout exceeds its duration
//   - ErrNoFutures: returned when WaitAny is called with empty slice
//
// Functions executed via Async can return errors normally, which are propagated
// through the Future's Await methods.
//
// # Context cancellation
//
// All asynchronous operations respect context cancellation. If the context is
// cancelled before execution begins, the Future returns immediately with the
// context's error without starting a goroutine.
package async
