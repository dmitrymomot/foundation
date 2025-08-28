// Package opensearch provides production-ready OpenSearch client initialization with immediate health verification.
//
// This package wraps the official OpenSearch Go client with configuration-driven setup and
// fail-fast connectivity verification, preventing broken clients from being returned to callers.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/integration/database/opensearch"
//
//	func main() {
//		ctx := context.Background()
//
//		cfg := opensearch.Config{
//			Addresses:  []string{"https://localhost:9200"},
//			Username:   "admin",
//			Password:   "admin",
//			MaxRetries: 3,
//		}
//
//		// Create client with immediate health verification
//		client, err := opensearch.New(ctx, cfg)
//		if err != nil {
//			log.Fatal("Failed to connect to OpenSearch:", err)
//		}
//
//		// Use client for searches, indexing, etc.
//		resp, err := client.Info()
//		if err != nil {
//			log.Fatal("Cluster info failed:", err)
//		}
//	}
//
// # Configuration
//
// Config supports environment variable mapping for zero-config initialization:
//
//	type Config struct {
//		Addresses    []string // OPENSEARCH_ADDRESSES (required)
//		Username     string   // OPENSEARCH_USERNAME (notEmpty)
//		Password     string   // OPENSEARCH_PASSWORD (notEmpty)
//		MaxRetries   int      // OPENSEARCH_MAX_RETRIES (default: 3)
//		DisableRetry bool     // OPENSEARCH_DISABLE_RETRY (default: false)
//	}
//
// Multiple addresses enable high availability with automatic failover.
//
// # Health Checking
//
// Healthcheck returns a function suitable for liveness/readiness probes:
//
//	client, _ := opensearch.New(ctx, cfg)
//	healthCheck := opensearch.Healthcheck(client)
//
//	// Use in HTTP handler
//	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
//		if err := healthCheck(r.Context()); err != nil {
//			http.Error(w, "Search cluster unhealthy", http.StatusServiceUnavailable)
//			return
//		}
//		w.WriteHeader(http.StatusOK)
//	})
//
// # Error Handling
//
// Package defines domain-specific errors for application-level handling:
//
//   - ErrConnectionFailed: Client creation failed due to configuration/network issues
//   - ErrHealthcheckFailed: Cluster unreachable during initialization or monitoring
//
// Both errors wrap underlying client errors and can be checked with errors.Is().
package opensearch
