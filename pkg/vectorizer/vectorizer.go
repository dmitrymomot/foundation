package vectorizer

import "context"

// Vectorizer converts text to embeddings.
type Vectorizer interface {
	// Embed converts a single text to vector embedding.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch converts multiple texts to vector embeddings.
	// Returns embeddings in the same order as input texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the vector size this implementation produces.
	Dimensions() int
}
