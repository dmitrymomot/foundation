package vectorizer

import (
	"context"
	"fmt"
	"slices"

	"google.golang.org/genai"
)

// Google model constants.
const (
	GoogleTextEmbedding005             = "text-embedding-005"
	GoogleTextMultilingualEmbedding002 = "text-multilingual-embedding-002"
)

// Default dimensions for Google models.
// Optimized for RAG systems - good semantic understanding.
const (
	defaultDimensionsGoogle = 768 // Optimal balance for RAG semantic search
)

// Google implements the Vectorizer interface using Google's Generative AI API.
type Google struct {
	client     *genai.Client
	model      string
	dimensions int
	maxBatch   int
	backend    genai.Backend
	project    string
	location   string
}

// GoogleOption is a functional option for configuring Google.
type GoogleOption func(*Google)

// WithGoogleModel sets the model to use.
func WithGoogleModel(model string) GoogleOption {
	return func(g *Google) {
		g.model = model
	}
}

// WithGoogleDimensions sets the output dimensions for the embeddings.
// Supported values: 256, 768, 1536, 3072.
func WithGoogleDimensions(dims int) GoogleOption {
	return func(g *Google) {
		g.dimensions = dims
	}
}

// WithGoogleMaxBatchSize sets the maximum batch size for batch operations.
func WithGoogleMaxBatchSize(size int) GoogleOption {
	return func(g *Google) {
		if size > 0 && size <= 100 {
			g.maxBatch = size
		}
	}
}

// WithGoogleBackend sets the backend to use (Gemini API or Vertex AI).
func WithGoogleBackend(backend genai.Backend) GoogleOption {
	return func(g *Google) {
		g.backend = backend
	}
}

// WithGoogleProject sets the GCP project ID for Vertex AI.
func WithGoogleProject(project string) GoogleOption {
	return func(g *Google) {
		g.project = project
	}
}

// WithGoogleLocation sets the GCP location/region for Vertex AI.
func WithGoogleLocation(location string) GoogleOption {
	return func(g *Google) {
		g.location = location
	}
}

// NewGoogle creates a new Google vectorizer with Gemini API and API key authentication.
func NewGoogle(ctx context.Context, apiKey string, opts ...GoogleOption) (*Google, error) {
	if apiKey == "" {
		return nil, ErrInvalidAPIKey
	}

	g := &Google{
		model:      GoogleTextMultilingualEmbedding002, // Best multilingual support
		dimensions: defaultDimensionsGoogle,            // 768 dims for RAG quality
		maxBatch:   100,
		backend:    genai.BackendGeminiAPI,
	}

	// Apply options first to get any backend/project/location settings
	for _, opt := range opts {
		opt(g)
	}

	// Create client with proper backend configuration
	config := &genai.ClientConfig{
		APIKey:   apiKey,
		Backend:  g.backend,
		Project:  g.project,
		Location: g.location,
	}

	client, err := genai.NewClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google AI client: %w", err)
	}
	g.client = client

	// Validate model and dimensions
	if err := g.validateConfiguration(); err != nil {
		return nil, err
	}

	return g, nil
}

// NewGoogleVertexAI creates a new Google vectorizer using Vertex AI with project and location.
func NewGoogleVertexAI(ctx context.Context, project, location string, opts ...GoogleOption) (*Google, error) {
	if project == "" || location == "" {
		return nil, fmt.Errorf("project and location are required for Vertex AI backend")
	}

	g := &Google{
		model:      GoogleTextMultilingualEmbedding002, // Best multilingual support
		dimensions: defaultDimensionsGoogle,            // 768 dims for RAG quality
		maxBatch:   100,
		backend:    genai.BackendVertexAI,
		project:    project,
		location:   location,
	}

	// Apply options
	for _, opt := range opts {
		opt(g)
	}

	// Create client with Vertex AI configuration
	config := &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  g.project,
		Location: g.location,
	}

	client, err := genai.NewClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Vertex AI client: %w", err)
	}
	g.client = client

	// Validate model and dimensions
	if err := g.validateConfiguration(); err != nil {
		return nil, err
	}

	return g, nil
}

// validateConfiguration validates the model and dimensions configuration.
func (g *Google) validateConfiguration() error {
	switch g.model {
	case GoogleTextEmbedding005, GoogleTextMultilingualEmbedding002:
		// These models support configurable dimensions
		validDims := []int{256, 768, 1536, 3072}
		if !slices.Contains(validDims, g.dimensions) {
			return fmt.Errorf("%w: %s only supports dimensions 256, 768, 1536, or 3072, got %d",
				ErrInvalidDimensions, g.model, g.dimensions)
		}
	default:
		return fmt.Errorf("%w: %s", ErrModelNotSupported, g.model)
	}
	return nil
}

// Embed converts a single text to vector embedding.
func (g *Google) Embed(ctx context.Context, text string) ([]float32, error) {
	content := &genai.Content{
		Parts: []*genai.Part{genai.NewPartFromText(text)},
	}

	config := &genai.EmbedContentConfig{}
	// Configure dimensions if supported
	if g.supportsConfigurableDimensions() {
		dims := int32(g.dimensions)
		config.OutputDimensionality = &dims
	}

	resp, err := g.client.Models.EmbedContent(ctx, "models/"+g.model, []*genai.Content{content}, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}

	if resp == nil || len(resp.Embeddings) == 0 || resp.Embeddings[0] == nil {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Embeddings[0].Values, nil
}

// EmbedBatch converts multiple texts to vector embeddings.
// Returns embeddings in the same order as input texts.
func (g *Google) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	if len(texts) > g.maxBatch {
		return nil, fmt.Errorf("%w: got %d texts, max is %d", ErrBatchTooLarge, len(texts), g.maxBatch)
	}

	// Create contents for each text
	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(text)},
		}
	}

	config := &genai.EmbedContentConfig{}
	// Configure dimensions if supported
	if g.supportsConfigurableDimensions() {
		dims := int32(g.dimensions)
		config.OutputDimensionality = &dims
	}

	resp, err := g.client.Models.EmbedContent(ctx, "models/"+g.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(resp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(resp.Embeddings))
	}

	// Extract embeddings in order
	result := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		if emb == nil || emb.Values == nil {
			return nil, fmt.Errorf("empty embedding at index %d", i)
		}
		result[i] = emb.Values
	}

	return result, nil
}

// Dimensions returns the vector size this implementation produces.
func (g *Google) Dimensions() int {
	return g.dimensions
}

// supportsConfigurableDimensions returns true if the model supports configurable dimensions.
func (g *Google) supportsConfigurableDimensions() bool {
	switch g.model {
	case GoogleTextEmbedding005, GoogleTextMultilingualEmbedding002:
		return true
	default:
		return false
	}
}
