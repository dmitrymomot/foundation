// Package opensearch provides production-ready OpenSearch client initialization and health checking for search and analytics workloads.
//
// This package wraps the official OpenSearch Go client with configuration-driven setup and immediate
// connectivity verification. It's designed for applications that need reliable search capabilities with
// both self-hosted OpenSearch clusters and managed services like Amazon OpenSearch Service.
//
// # Key Features
//
// The package provides a single client creation function with immediate health verification:
//
//   - New: Creates an OpenSearch client with immediate cluster connectivity verification
//
// The New function performs an immediate health check to fail fast if the cluster is unreachable,
// preventing broken clients from being returned to callers and avoiding runtime failures.
//
// # Configuration
//
// All configuration is handled through the Config struct with environment variable mapping:
//
//	type Config struct {
//		Addresses    []string `env:"OPENSEARCH_ADDRESSES,required"`
//		Username     string   `env:"OPENSEARCH_USERNAME,notEmpty"`
//		Password     string   `env:"OPENSEARCH_PASSWORD,notEmpty"`
//		MaxRetries   int      `env:"OPENSEARCH_MAX_RETRIES" envDefault:"3"`
//		DisableRetry bool     `env:"OPENSEARCH_DISABLE_RETRY" envDefault:"false"`
//	}
//
// The configuration supports multiple cluster addresses for high availability and includes
// authentication credentials and retry behavior control.
//
// # Usage Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"strings"
//
//		"github.com/dmitrymomot/gokit/integration/database/opensearch"
//		"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		// Load configuration from environment variables
//		cfg := opensearch.Config{
//			Addresses:  []string{"https://localhost:9200"},
//			Username:   "admin",
//			Password:   "admin",
//			MaxRetries: 3,
//		}
//
//		// Create OpenSearch client with immediate health verification
//		client, err := opensearch.New(ctx, cfg)
//		if err != nil {
//			log.Fatal("Failed to connect to OpenSearch:", err)
//		}
//
//		// Perform a simple search
//		searchReq := opensearchapi.SearchRequest{
//			Index: []string{"my-index"},
//			Body:  strings.NewReader(`{"query": {"match_all": {}}}`),
//		}
//
//		resp, err := searchReq.Do(ctx, client)
//		if err != nil {
//			log.Fatal("Search failed:", err)
//		}
//		defer resp.Body.Close()
//
//		log.Printf("Search response status: %s", resp.Status())
//	}
//
// # Health Checking
//
// The package provides a health check function suitable for Kubernetes readiness/liveness probes
// or HTTP health endpoints:
//
//	client, err := opensearch.New(ctx, cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//
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
// The health check calls client.Info() to verify cluster connectivity and includes error tracing
// for detailed diagnostics when issues occur.
//
// # Error Handling
//
// The package defines domain-specific errors that can be checked using errors.Is():
//
//   - ErrConnectionFailed: Returned when the OpenSearch client cannot be created due to configuration or network issues
//   - ErrHealthcheckFailed: Returned when the cluster is unreachable or unhealthy during initialization or monitoring
//
// Both errors wrap the underlying OpenSearch client errors while providing stable error types
// for application-level error handling and appropriate user-facing messages.
//
// # Multiple Addresses and High Availability
//
// The configuration supports multiple OpenSearch cluster addresses for high availability:
//
//	cfg := opensearch.Config{
//		Addresses: []string{
//			"https://node1.opensearch.example.com:9200",
//			"https://node2.opensearch.example.com:9200",
//			"https://node3.opensearch.example.com:9200",
//		},
//		Username: "search-user",
//		Password: "secure-password",
//	}
//
// The OpenSearch client will automatically handle failover between the provided addresses,
// ensuring continued operation even if individual cluster nodes become unavailable.
//
// # Security Considerations
//
// For production deployments, always use HTTPS endpoints and proper authentication:
//
//   - Use strong, unique passwords for OpenSearch users
//   - Configure appropriate role-based access control (RBAC) in OpenSearch
//   - Consider using certificate-based authentication for enhanced security
//   - Store credentials securely using environment variables or secret management systems
//
// # Performance and Tuning
//
// The default MaxRetries setting (3) provides a good balance between reliability and performance.
// For high-throughput applications, you may want to:
//
//   - Adjust MaxRetries based on your error tolerance and latency requirements
//   - Set DisableRetry to true for applications that handle retries at a higher level
//   - Monitor cluster health and adjust timeouts based on your cluster's performance characteristics
//
// For batch operations or background indexing, consider implementing exponential backoff
// in your application code to handle temporary cluster overload situations gracefully.
package opensearch
