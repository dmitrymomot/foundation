// Package mongo provides production-ready MongoDB client initialization and health checking for SaaS applications.
//
// This package wraps the official MongoDB Go driver with application-level retry logic and configuration
// optimized for cloud deployments, particularly MongoDB Atlas. It handles common deployment challenges like
// cold starts, network hiccups, and connection pool management through configuration-driven setup.
//
// # Key Features
//
// The package provides two main client creation functions with retry logic and immediate connection verification:
//
//   - New: Creates a MongoDB client with retry logic and connection verification
//   - NewWithDatabase: Convenience wrapper that returns a database instance directly
//
// Both functions implement exponential backoff retry logic to handle MongoDB Atlas cold starts (5-8 seconds)
// and brief network interruptions that could otherwise cause application startup failures.
//
// # Configuration
//
// All configuration is handled through the Config struct with environment variable mapping:
//
//	type Config struct {
//		ConnectionURL     string        `env:"MONGODB_URL,required"`
//		ConnectTimeout    time.Duration `env:"MONGODB_CONNECT_TIMEOUT" envDefault:"10s"`
//		MaxPoolSize       uint64        `env:"MONGODB_MAX_POOL_SIZE" envDefault:"100"`
//		MinPoolSize       uint64        `env:"MONGODB_MIN_POOL_SIZE" envDefault:"1"`
//		MaxConnIdleTime   time.Duration `env:"MONGODB_MAX_CONN_IDLE_TIME" envDefault:"300s"`
//		RetryWrites       bool          `env:"MONGODB_RETRY_WRITES" envDefault:"true"`
//		RetryReads        bool          `env:"MONGODB_RETRY_READS" envDefault:"true"`
//		RetryAttempts     int           `env:"MONGODB_RETRY_ATTEMPTS" envDefault:"3"`
//		RetryInterval     time.Duration `env:"MONGODB_RETRY_INTERVAL" envDefault:"5s"`
//	}
//
// The default values are optimized for typical SaaS workloads with MongoDB Atlas, balancing performance,
// resource usage, and reliability.
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
//		"github.com/dmitrymomot/gokit/integration/database/mongo"
//		"go.mongodb.org/mongo-driver/v2/bson"
//	)
//
//	func main() {
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		// Load configuration from environment variables
//		cfg := mongo.Config{
//			ConnectionURL: "mongodb+srv://user:pass@cluster.mongodb.net/mydb",
//			RetryAttempts: 3,
//			RetryInterval: 5 * time.Second,
//		}
//
//		// Create MongoDB client with retry logic
//		client, err := mongo.New(ctx, cfg)
//		if err != nil {
//			log.Fatal("Failed to connect to MongoDB:", err)
//		}
//		defer client.Disconnect(ctx)
//
//		// Or get database directly
//		db, err := mongo.NewWithDatabase(ctx, cfg, "myapp")
//		if err != nil {
//			log.Fatal("Failed to connect to database:", err)
//		}
//
//		// Use the database
//		collection := db.Collection("users")
//		result, err := collection.InsertOne(ctx, bson.M{"name": "Alice", "age": 30})
//		if err != nil {
//			log.Fatal("Failed to insert document:", err)
//		}
//		log.Printf("Inserted document with ID: %v", result.InsertedID)
//	}
//
// # Health Checking
//
// The package provides a health check function suitable for Kubernetes readiness/liveness probes
// or HTTP health endpoints:
//
//	client, err := mongo.New(ctx, cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	healthCheck := mongo.Healthcheck(client)
//
//	// Use in HTTP handler
//	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
//		if err := healthCheck(r.Context()); err != nil {
//			http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
//			return
//		}
//		w.WriteHeader(http.StatusOK)
//	})
//
// The health check performs a lightweight Ping operation that verifies MongoDB connectivity
// without impacting database performance.
//
// # Error Handling
//
// The package defines domain-specific errors that can be checked using errors.Is():
//
//   - ErrFailedToConnectToMongo: Returned when all retry attempts are exhausted
//   - ErrHealthcheckFailed: Returned when health check ping fails
//
// These errors wrap the underlying MongoDB driver errors while providing stable error types
// for application-level error handling, retry logic, and appropriate user-facing messages.
//
// # Production Considerations
//
// The default configuration values are optimized for MongoDB Atlas deployments:
//
//   - Connection timeouts are aggressive enough to fail fast but accommodate Atlas cold starts
//   - Pool sizes balance burst traffic handling with database resource constraints
//   - Retry logic handles Atlas replica set primary elections and transient network issues
//   - Connection lifecycle management prevents issues with connection poolers and load balancers
//
// For on-premises MongoDB deployments, you may need to adjust timeout and pool settings
// based on your infrastructure characteristics and expected load patterns.
package mongo
