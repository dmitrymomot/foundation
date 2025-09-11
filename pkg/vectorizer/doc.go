// Package vectorizer provides unified interfaces and implementations for converting
// text into vector embeddings using different AI providers.
//
// This package abstracts away the differences between various embedding APIs,
// allowing applications to switch between providers with minimal code changes.
// It supports both OpenAI and Google AI embedding models with configurable
// dimensions optimized for RAG (Retrieval-Augmented Generation) systems.
//
// # Basic Usage
//
// Create a vectorizer and generate embeddings:
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/foundation/pkg/vectorizer"
//	)
//
//	func main() {
//		ctx := context.Background()
//
//		// OpenAI implementation with default settings
//		openaiVectorizer, err := vectorizer.NewOpenAI("your-openai-api-key")
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Single text embedding
//		embedding, err := openaiVectorizer.Embed(ctx, "Hello world")
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Printf("Embedding dimensions: %d\n", len(embedding))
//
//		// Batch embeddings (more efficient)
//		texts := []string{"Hello", "World", "AI"}
//		embeddings, err := openaiVectorizer.EmbedBatch(ctx, texts)
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Printf("Generated %d embeddings\n", len(embeddings))
//	}
//
// # Provider Configuration
//
// ## OpenAI Configuration
//
// OpenAI supports text-embedding-3-small and text-embedding-3-large models:
//
//	// High-quality embeddings with custom dimensions
//	openaiVectorizer, err := vectorizer.NewOpenAI("api-key",
//		vectorizer.WithOpenAIModel(vectorizer.OpenAITextEmbedding3Large),
//		vectorizer.WithOpenAIDimensions(3072), // Maximum quality
//		vectorizer.WithOpenAIMaxBatchSize(500), // Custom batch size
//	)
//
//	// Cost-optimized embeddings
//	costOptimized, err := vectorizer.NewOpenAI("api-key",
//		vectorizer.WithOpenAIModel(vectorizer.OpenAITextEmbedding3Small),
//		vectorizer.WithOpenAIDimensions(512), // Smaller, cheaper
//	)
//
// ## Google AI Configuration
//
// Google AI supports Gemini API and Vertex AI backends:
//
//	// Gemini API (simpler setup)
//	googleVectorizer, err := vectorizer.NewGoogle(ctx, "your-gemini-api-key",
//		vectorizer.WithGoogleModel(vectorizer.GoogleTextEmbedding005),
//		vectorizer.WithGoogleDimensions(768),
//	)
//
//	// Vertex AI (enterprise features)
//	vertexVectorizer, err := vectorizer.NewGoogleVertexAI(ctx, "your-project", "us-central1",
//		vectorizer.WithGoogleModel(vectorizer.GoogleTextMultilingualEmbedding002),
//		vectorizer.WithGoogleDimensions(1536),
//	)
//
// # Model Capabilities and Dimensions
//
// ## OpenAI Models:
//   - text-embedding-3-small: 512 or 1536 dimensions (default: 1536)
//   - text-embedding-3-large: 256, 1024, or 3072 dimensions (default: 3072)
//
// ## Google Models:
//   - text-embedding-005: 256, 768, 1536, or 3072 dimensions (default: 768)
//   - text-multilingual-embedding-002: 256, 768, 1536, or 3072 dimensions (default: 768)
//
// # Advanced Usage Examples
//
// ## RAG System Implementation
//
//	type RAGSystem struct {
//		vectorizer vectorizer.Vectorizer
//		store      VectorStore // Your vector database
//	}
//
//	func (r *RAGSystem) IndexDocuments(ctx context.Context, docs []string) error {
//		// Process in batches for efficiency
//		batchSize := 50
//		for i := 0; i < len(docs); i += batchSize {
//			end := min(i+batchSize, len(docs))
//			batch := docs[i:end]
//
//			embeddings, err := r.vectorizer.EmbedBatch(ctx, batch)
//			if err != nil {
//				return fmt.Errorf("failed to embed batch: %w", err)
//			}
//
//			if err := r.store.Store(ctx, batch, embeddings); err != nil {
//				return fmt.Errorf("failed to store embeddings: %w", err)
//			}
//		}
//		return nil
//	}
//
// ## Provider Switching
//
//	func createVectorizer(provider string) (vectorizer.Vectorizer, error) {
//		switch provider {
//		case "openai":
//			return vectorizer.NewOpenAI(os.Getenv("OPENAI_API_KEY"))
//		case "google":
//			return vectorizer.NewGoogle(ctx, os.Getenv("GOOGLE_API_KEY"))
//		default:
//			return nil, fmt.Errorf("unsupported provider: %s", provider)
//		}
//	}
//
// # Error Handling
//
// The package defines specific error types for robust error handling:
//
//	embedding, err := v.Embed(ctx, text)
//	if err != nil {
//		switch {
//		case errors.Is(err, vectorizer.ErrInvalidAPIKey):
//			// Handle authentication issues
//			log.Fatal("Invalid API key provided")
//		case errors.Is(err, vectorizer.ErrRateLimitExceeded):
//			// Implement retry with backoff
//			time.Sleep(time.Second * 5)
//			return v.Embed(ctx, text)
//		case errors.Is(err, vectorizer.ErrTextTooLong):
//			// Split or truncate text
//			return v.Embed(ctx, text[:maxLength])
//		default:
//			return nil, fmt.Errorf("embedding failed: %w", err)
//		}
//	}
//
// Common error types:
//   - ErrInvalidAPIKey: Missing or invalid API credentials
//   - ErrModelNotSupported: Unsupported model specified
//   - ErrInvalidDimensions: Invalid dimension count for the model
//   - ErrBatchTooLarge: Batch size exceeds provider limits
//   - ErrRateLimitExceeded: API rate limit exceeded
//   - ErrTextTooLong: Input text exceeds token limits
//
// # Performance Considerations
//
// ## Batch Processing
//
// Always use batch operations when processing multiple texts:
//
//	// Efficient: Single API call
//	embeddings, err := v.EmbedBatch(ctx, texts)
//
//	// Inefficient: Multiple API calls
//	for _, text := range texts {
//		embedding, err := v.Embed(ctx, text)
//		// ...
//	}
//
// ## Batch Size Limits
//
// Configure batch sizes based on provider limits and your use case:
//
//   - OpenAI: Up to 2048 texts per batch (default: 100)
//   - Google: Up to 100 texts per batch (default: 100)
//
// ## Dimension Selection
//
// Choose dimensions based on your requirements:
//
//   - Lower dimensions: Faster processing, less storage, lower costs
//   - Higher dimensions: Better semantic understanding, higher accuracy
//   - RAG systems: 768-1536 dimensions provide good balance
//   - High-precision tasks: Use maximum dimensions available
//
// ## Cost Optimization
//
// For cost-sensitive applications:
//
//	// Use smaller, cheaper models
//	economical, err := vectorizer.NewOpenAI("api-key",
//		vectorizer.WithOpenAIModel(vectorizer.OpenAITextEmbedding3Small),
//		vectorizer.WithOpenAIDimensions(512), // Minimum dimensions
//	)
//
// # API Rate Limits and Constraints
//
// ## OpenAI Limits
//   - Rate limits: Varies by subscription tier
//   - Token limits: ~8191 tokens per request
//   - Batch size: Up to 2048 texts
//
// ## Google AI Limits
//   - Rate limits: Varies by quota settings
//   - Token limits: Model-dependent
//   - Batch size: Up to 100 texts
//
// ## Best Practices
//
//   - Implement exponential backoff for rate limit errors
//   - Monitor API usage and costs
//   - Cache embeddings when possible
//   - Use appropriate batch sizes for your workload
//   - Consider using multiple API keys for higher throughput
package vectorizer
