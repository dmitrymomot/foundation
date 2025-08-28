// Package redis provides Redis client initialization with connection retry logic and health checking.
//
// This package wraps the go-redis client with connection validation, exponential backoff retry,
// and proper error handling for reliable Redis connectivity.
//
// # Basic Usage
//
//	import (
//		"context"
//		"time"
//
//		"github.com/dmitrymomot/foundation/integration/database/redis"
//	)
//
//	cfg := redis.Config{
//		ConnectionURL:  "redis://localhost:6379/0",
//		RetryAttempts:  3,
//		RetryInterval:  5 * time.Second,
//		ConnectTimeout: 30 * time.Second,
//	}
//
//	client, err := redis.Connect(ctx, cfg)
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	// Use standard go-redis client methods
//	err = client.Set(ctx, "key", "value", time.Hour).Err()
//
// # Health Checking
//
// Use Healthcheck for monitoring Redis connectivity:
//
//	healthCheck := redis.Healthcheck(client)
//	if err := healthCheck(ctx); err != nil {
//		// Handle Redis health check failure
//	}
//
// # Configuration
//
// Config struct supports environment variable mapping:
//
//	type Config struct {
//		ConnectionURL  string        // REDIS_URL (required)
//		RetryAttempts  int           // REDIS_RETRY_ATTEMPTS (default: 3)
//		RetryInterval  time.Duration // REDIS_RETRY_INTERVAL (default: 5s)
//		ConnectTimeout time.Duration // REDIS_CONNECT_TIMEOUT (default: 30s)
//		ScanBatchSize  int           // REDIS_SCAN_BATCH_SIZE (default: 1000)
//	}
//
// Supports both redis:// and rediss:// (TLS) URL schemes.
//
// # Error Handling
//
// Package defines specific errors for reliable error handling:
//
//   - ErrFailedToParseRedisConnString: Invalid Redis connection URL
//   - ErrRedisNotReady: Connection failed after all retry attempts
//   - ErrEmptyConnectionURL: Missing connection URL
//   - ErrHealthcheckFailed: Health check ping failed
package redis
