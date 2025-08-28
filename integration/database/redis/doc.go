// Package redis provides production-ready Redis client initialization and health checking for caching and session management in SaaS applications.
//
// This package wraps the popular go-redis/redis client with connection validation, retry logic, and configuration
// optimized for reliable Redis connectivity. It supports both Redis and Redis-compatible services with proper
// URL validation and exponential backoff retry logic for handling transient network issues.
//
// # Key Features
//
// The package provides Redis client creation with immediate connectivity verification:
//
//   - Connect: Creates a Redis client with exponential retry logic and connection verification
//   - Healthcheck: Returns a health check function for monitoring Redis connectivity
//
// Connection establishment validates the Redis URL format, attempts connection with retries,
// and verifies connectivity with a ping operation before returning the client.
//
// # Configuration
//
// All configuration is handled through the Config struct with environment variable mapping:
//
//	type Config struct {
//		ConnectionURL  string        `env:"REDIS_URL,required" envDefault:"redis://localhost:6379/0"`
//		RetryAttempts  int           `env:"REDIS_RETRY_ATTEMPTS" envDefault:"3"`
//		RetryInterval  time.Duration `env:"REDIS_RETRY_INTERVAL" envDefault:"5s"`
//		ConnectTimeout time.Duration `env:"REDIS_CONNECT_TIMEOUT" envDefault:"30s"`
//		ScanBatchSize  int           `env:"REDIS_SCAN_BATCH_SIZE" envDefault:"1000"`
//	}
//
// The configuration supports both redis:// and rediss:// (TLS) URL schemes and includes
// timeout and retry behavior control for reliable operation in cloud environments.
//
// # Usage Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"time"
//
//		"github.com/dmitrymomot/foundation/integration/database/redis"
//	)
//
//	func main() {
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		// Load configuration from environment variables
//		cfg := redis.Config{
//			ConnectionURL: "redis://localhost:6379/0",
//			RetryAttempts: 3,
//			RetryInterval: 5 * time.Second,
//			ConnectTimeout: 30 * time.Second,
//		}
//
//		// Create Redis client with retry logic and verification
//		client, err := redis.Connect(ctx, cfg)
//		if err != nil {
//			log.Fatal("Failed to connect to Redis:", err)
//		}
//		defer client.Close()
//
//		// Use the Redis client for caching
//		err = client.Set(ctx, "user:123", "Alice", time.Hour).Err()
//		if err != nil {
//			log.Fatal("Failed to set key:", err)
//		}
//
//		val, err := client.Get(ctx, "user:123").Result()
//		if err != nil {
//			log.Fatal("Failed to get key:", err)
//		}
//		log.Printf("Retrieved value: %s", val)
//
//		// Use for session management
//		sessionID := "sess_abc123"
//		sessionData := map[string]any{
//			"user_id": 123,
//			"role":    "admin",
//		}
//		err = client.HMSet(ctx, sessionID, sessionData).Err()
//		if err != nil {
//			log.Fatal("Failed to store session:", err)
//		}
//
//		// Set session expiration
//		err = client.Expire(ctx, sessionID, 24*time.Hour).Err()
//		if err != nil {
//			log.Fatal("Failed to set expiration:", err)
//		}
//	}
//
// # Health Checking
//
// The package provides a health check function suitable for Kubernetes readiness/liveness probes
// or HTTP health endpoints:
//
//	client, err := redis.Connect(ctx, cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	healthCheck := redis.Healthcheck(client)
//
//	// Use in HTTP handler
//	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
//		if err := healthCheck(r.Context()); err != nil {
//			http.Error(w, "Redis unhealthy", http.StatusServiceUnavailable)
//			return
//		}
//		w.WriteHeader(http.StatusOK)
//	})
//
// The health check performs a ping operation to verify Redis connectivity and responsiveness
// without impacting Redis performance or consuming significant resources.
//
// # Error Handling
//
// The package defines domain-specific errors that can be checked using errors.Is():
//
//   - ErrFailedToParseRedisConnString: Returned when the Redis connection URL is malformed
//   - ErrRedisNotReady: Returned when Redis doesn't become ready within the timeout period
//   - ErrEmptyConnectionURL: Returned when no connection URL is provided
//   - ErrHealthcheckFailed: Returned when health check ping fails
//
// These errors wrap the underlying go-redis client errors while providing stable error types
// for application-level error handling, retry logic, and appropriate user-facing messages.
//
// # Connection URL Formats
//
// The package supports standard Redis URL formats:
//
//	// Basic Redis connection
//	redis://localhost:6379/0
//
//	// Redis with authentication
//	redis://username:password@localhost:6379/0
//
//	// Redis with TLS (rediss://)
//	rediss://username:password@redis.example.com:6380/0
//
//	// Redis Cloud or managed service
//	redis://default:password@redis-12345.cloud.redislabs.com:12345
//
// The package validates URL schemes and will reject URLs that don't use redis:// or rediss:// protocols.
//
// # Retry Logic and Timeouts
//
// Connection establishment uses exponential backoff to handle transient network issues:
//
//   - RetryAttempts (3): Number of connection attempts before giving up
//   - RetryInterval (5s): Base interval between retry attempts
//   - ConnectTimeout (30s): Overall timeout for the entire connection process
//
// The retry logic respects context cancellation and will abort early if the context
// deadline is exceeded during the retry process.
//
// # Caching Patterns
//
// The Redis client can be used for various caching patterns common in SaaS applications:
//
//	// Simple key-value caching
//	err = client.Set(ctx, "expensive_computation_key", result, time.Hour).Err()
//
//	// Cache with conditional setting (only if not exists)
//	err = client.SetNX(ctx, "lock_key", "process_id", time.Minute).Err()
//
//	// Hash-based caching for structured data
//	err = client.HMSet(ctx, "user:profile:123", map[string]any{
//		"name":  "Alice",
//		"email": "alice@example.com",
//		"role":  "admin",
//	}).Err()
//
//	// List-based caching for ordered data
//	err = client.LPush(ctx, "recent_orders:123", orderID).Err()
//	err = client.LTrim(ctx, "recent_orders:123", 0, 99).Err() // Keep last 100
//
// # Session Management
//
// Redis is particularly well-suited for session management in web applications:
//
//	// Store session data
//	sessionKey := "session:" + sessionID
//	err = client.HMSet(ctx, sessionKey, map[string]any{
//		"user_id":    userID,
//		"role":       userRole,
//		"created_at": time.Now().Unix(),
//	}).Err()
//
//	// Set session expiration
//	err = client.Expire(ctx, sessionKey, 24*time.Hour).Err()
//
//	// Retrieve session data
//	sessionData, err := client.HGetAll(ctx, sessionKey).Result()
//
//	// Extend session lifetime on activity
//	err = client.Expire(ctx, sessionKey, 24*time.Hour).Err()
//
// # Performance Considerations
//
// For high-performance applications, consider these Redis usage patterns:
//
//   - Use pipeline operations for bulk operations to reduce network round trips
//   - Set appropriate expiration times to prevent memory bloat
//   - Use Redis data structures efficiently (hashes for objects, sets for unique collections)
//   - Monitor Redis memory usage and configure appropriate eviction policies
//   - Consider Redis clustering for horizontal scaling in high-traffic scenarios
//
// The ScanBatchSize configuration option controls the batch size for SCAN operations,
// which can be tuned based on your dataset size and performance requirements.
package redis
