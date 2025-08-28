// Package mongo provides production-ready MongoDB client initialization and health checking for SaaS applications.
//
// This package wraps the official MongoDB Go driver with application-level retry logic
// optimized for cloud deployments, particularly MongoDB Atlas. It handles common deployment
// challenges like cold starts, network hiccups, and connection pool management.
//
// Both New and NewWithDatabase functions implement retry logic to handle MongoDB Atlas
// cold starts (5-8 seconds) and brief network interruptions that could otherwise cause
// application startup failures.
//
// Basic usage:
//
//	import (
//		"context"
//		"log"
//
//		"github.com/caarlos0/env/v11"
//		"github.com/dmitrymomot/foundation/integration/database/mongo"
//		"go.mongodb.org/mongo-driver/v2/bson"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		// Load configuration from environment variables
//		var cfg mongo.Config
//		if err := env.Parse(&cfg); err != nil {
//			log.Fatal("Failed to parse config:", err)
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
// # Configuration
//
// Configuration is handled through environment variables via the Config struct.
// The default values are optimized for MongoDB Atlas deployments:
//
//	MONGODB_URL                 (required)
//	MONGODB_CONNECT_TIMEOUT     (default: 10s)
//	MONGODB_MAX_POOL_SIZE       (default: 100)
//	MONGODB_MIN_POOL_SIZE       (default: 1)
//	MONGODB_MAX_CONN_IDLE_TIME  (default: 300s)
//	MONGODB_RETRY_WRITES        (default: true)
//	MONGODB_RETRY_READS         (default: true)
//	MONGODB_RETRY_ATTEMPTS      (default: 3)
//	MONGODB_RETRY_INTERVAL      (default: 5s)
//
// # Health Checking
//
// The package provides a health check function for Kubernetes probes or HTTP endpoints:
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
// # Error Handling
//
// The package defines domain-specific errors:
//
//	ErrFailedToConnectToMongo - Returned when all retry attempts are exhausted
//	ErrHealthcheckFailed      - Returned when health check ping fails
//
// The New function includes connection verification via Ping to ensure the connection
// is actually usable before returning.
package mongo
