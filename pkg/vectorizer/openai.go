package vectorizer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAI model constants.
const (
	OpenAITextEmbedding3Small = "text-embedding-3-small"
	OpenAITextEmbedding3Large = "text-embedding-3-large"
)

// Default dimensions for OpenAI models.
// Optimized for RAG systems - balancing quality and cost.
const (
	defaultDimensionsSmall = 1536 // Optimal for RAG semantic search
	defaultDimensionsLarge = 3072 // Maximum quality for complex RAG
)

// OpenAI implements the Vectorizer interface using OpenAI's API.
type OpenAI struct {
	client     openai.Client
	model      string
	dimensions int
	maxBatch   int
}

// OpenAIOption is a functional option for configuring OpenAI.
type OpenAIOption func(*OpenAI)

// WithOpenAIModel sets the model to use.
func WithOpenAIModel(model string) OpenAIOption {
	return func(o *OpenAI) {
		o.model = model
	}
}

// WithOpenAIDimensions sets the output dimensions for the embeddings.
// Only applicable to text-embedding-3-* models.
func WithOpenAIDimensions(dims int) OpenAIOption {
	return func(o *OpenAI) {
		o.dimensions = dims
	}
}

// WithOpenAIMaxBatchSize sets the maximum batch size for batch operations.
func WithOpenAIMaxBatchSize(size int) OpenAIOption {
	return func(o *OpenAI) {
		if size > 0 && size <= 2048 { // OpenAI API limit
			o.maxBatch = size
		}
	}
}

// WithOpenAIHTTPClient sets a custom HTTP client.
func WithOpenAIHTTPClient(client *http.Client) OpenAIOption {
	return func(o *OpenAI) {
		if client != nil {
			o.client = openai.NewClient(
				option.WithHTTPClient(client),
			)
		}
	}
}

// NewOpenAI creates a new OpenAI vectorizer.
func NewOpenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error) {
	if apiKey == "" {
		return nil, ErrInvalidAPIKey
	}

	o := &OpenAI{
		client:   openai.NewClient(option.WithAPIKey(apiKey)),
		model:    OpenAITextEmbedding3Small, // Best multilingual support and 5x cheaper than ada-002
		maxBatch: 100,
	}

	for _, opt := range opts {
		opt(o)
	}

	// Set default dimensions based on model if not specified
	if o.dimensions == 0 {
		switch o.model {
		case OpenAITextEmbedding3Small:
			o.dimensions = defaultDimensionsSmall
		case OpenAITextEmbedding3Large:
			o.dimensions = defaultDimensionsLarge
		default:
			return nil, fmt.Errorf("%w: %s", ErrModelNotSupported, o.model)
		}
	}

	if err := o.validateDimensions(); err != nil {
		return nil, err
	}

	return o, nil
}

// validateDimensions validates the dimensions for the configured model.
func (o *OpenAI) validateDimensions() error {
	switch o.model {
	case OpenAITextEmbedding3Small:
		// text-embedding-3-small supports 512 or 1536
		if o.dimensions != 512 && o.dimensions != 1536 {
			return fmt.Errorf("%w: %s only supports 512 or 1536 dimensions, got %d",
				ErrInvalidDimensions, o.model, o.dimensions)
		}
	case OpenAITextEmbedding3Large:
		// text-embedding-3-large supports 256, 1024, or 3072
		if o.dimensions != 256 && o.dimensions != 1024 && o.dimensions != 3072 {
			return fmt.Errorf("%w: %s only supports 256, 1024, or 3072 dimensions, got %d",
				ErrInvalidDimensions, o.model, o.dimensions)
		}
	default:
		return fmt.Errorf("%w: %s", ErrModelNotSupported, o.model)
	}
	return nil
}

// Embed converts a single text to vector embedding.
func (o *OpenAI) Embed(ctx context.Context, text string) ([]float32, error) {
	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(o.model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
	}

	params.Dimensions = openai.Int(int64(o.dimensions))

	resp, err := o.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	// Convert API response (float64) to our standard format (float32)
	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}
	return embedding, nil
}

// EmbedBatch converts multiple texts to vector embeddings.
// Returns embeddings in the same order as input texts.
func (o *OpenAI) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if len(texts) > o.maxBatch {
		return nil, fmt.Errorf("%w: got %d texts, max is %d", ErrBatchTooLarge, len(texts), o.maxBatch)
	}

	inputs := make([]string, len(texts))
	copy(inputs, texts)

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(o.model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: inputs,
		},
	}

	params.Dimensions = openai.Int(int64(o.dimensions))

	resp, err := o.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(resp.Data))
	}

	// Convert API response (float64) to our standard format (float32)
	result := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		result[i] = embedding
	}

	return result, nil
}

// Dimensions returns the vector size this implementation produces.
func (o *OpenAI) Dimensions() int {
	return o.dimensions
}
