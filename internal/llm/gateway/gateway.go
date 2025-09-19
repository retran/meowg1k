/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package gateway provides a unified interface for interacting with various Large Language Models (LLMs),
// supporting both content generation and text embedding computations.
package gateway

import (
	"context"
	"fmt"
	"math"

	"github.com/retran/meowg1k/internal/models"
)

// Provider defines an enumeration for supported LLM providers.
type Provider string

const (
	// Llama identifies the Llama provider.
	Llama Provider = "llama"
	// Gemini identifies the Gemini provider.
	Gemini Provider = "gemini"
	// Nebius identifies the Nebius AI Studio provider.
	Nebius Provider = "nebius"
	// OpenAI identifies the OpenAI provider.
	OpenAI Provider = "openai"
	// OpenRouter identifies the OpenRouter provider.
	OpenRouter Provider = "openrouter"
	// OpenAICompatible identifies OpenAI-compatible providers with custom base URLs.
	OpenAICompatible Provider = "openai-compatible"
	// Anthropic identifies the Anthropic provider.
	Anthropic Provider = "anthropic"
	// Voyage identifies the Voyage AI provider (embeddings only).
	Voyage Provider = "voyage"
)

// GenerationGateway defines the contract for a client that generates content using an LLM.
type GenerationGateway interface {
	// GenerateContent sends a content generation request to the LLM and returns the generated response.
	// It returns an error if the generation process fails.
	GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error)
}

// GenerateContentRequest holds the parameters for a content generation request.
type GenerateContentRequest struct {
	model        string
	systemPrompt string
	userPrompt   string
}

// NewGenerateContentRequest creates and returns a new GenerateContentRequest.
func NewGenerateContentRequest(model, systemPrompt, userPrompt string) *GenerateContentRequest {
	return &GenerateContentRequest{
		model:        model,
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	}
}

// Model returns the model name for the content generation request.
func (r *GenerateContentRequest) Model() string {
	return r.model
}

// SystemPrompt returns the system prompt for the content generation request.
func (r *GenerateContentRequest) SystemPrompt() string {
	return r.systemPrompt
}

// UserPrompt returns the user prompt for the content generation request.
func (r *GenerateContentRequest) UserPrompt() string {
	return r.userPrompt
}

// EmbeddingGateway defines the contract for a client that computes text embeddings
// and measures the distance between them.
type EmbeddingGateway interface {
	// ComputeEmbeddings computes the vector embedding for the given text.
	ComputeEmbeddings(ctx context.Context, request *ComputeEmbeddingsRequest) ([]Embedding, error)
	// ComputeDistance calculates a similarity or distance score between two embeddings.
	// The exact metric (e.g., cosine similarity) depends on the implementation.
	ComputeDistance(first, second Embedding) (float64, error)
}

// Embedding represents a text embedding as a vector of floating-point numbers.
type Embedding []float64

// TaskType specifies the intended use case for the text embedding. This allows the model
// to produce higher-quality embeddings tailored to the specific task.
type TaskType string

const (
	SemanticSimilarity TaskType = "SEMANTIC_SIMILARITY"
	Classification     TaskType = "CLASSIFICATION"
	Clustering         TaskType = "CLUSTERING"
	RetrievalDocument  TaskType = "RETRIEVAL_DOCUMENT"
	RetrievalQuery     TaskType = "RETRIEVAL_QUERY"
	CodeRetrievalQuery TaskType = "CODE_RETRIEVAL_QUERY"
	QuestionAnswering  TaskType = "QUESTION_ANSWERING"
	FactVerification   TaskType = "FACT_VERIFICATION"
)

// ComputeEmbeddingsRequest holds the parameters for a text embedding request.
type ComputeEmbeddingsRequest struct {
	model      string
	chunks     []string
	taskType   TaskType
	dimensions int // Output dimensionality (optional, uses model default if 0)
}

// NewComputeEmbeddingRequest creates and returns a new ComputeEmbeddingRequest.
// If the model has a default embedding dimension in the registry, it will be automatically used.
func NewComputeEmbeddingRequest(model string, chunks []string, taskType TaskType) *ComputeEmbeddingsRequest {
	// Get the default embedding dimension from the model registry
	defaultDim := models.GetDefaultEmbedDimension(model)
	return &ComputeEmbeddingsRequest{
		model:      model,
		chunks:     chunks,
		taskType:   taskType,
		dimensions: defaultDim,
	}
}

// NewComputeEmbeddingRequestWithDimensions creates a new ComputeEmbeddingRequest with custom dimensions.
func NewComputeEmbeddingRequestWithDimensions(model string, chunks []string, taskType TaskType, dimensions int) *ComputeEmbeddingsRequest {
	return &ComputeEmbeddingsRequest{
		model:      model,
		chunks:     chunks,
		taskType:   taskType,
		dimensions: dimensions,
	}
}

// Model returns the model name for the embedding request.
func (r *ComputeEmbeddingsRequest) Model() string {
	return r.model
}

// Chunks returns the text chunks to be embedded.
func (r *ComputeEmbeddingsRequest) Chunks() []string {
	return r.chunks
}

// TaskType returns the specified task type for the embedding.
func (r *ComputeEmbeddingsRequest) TaskType() TaskType {
	return r.taskType
}

// Dimensions returns the output dimensionality for the embedding.
func (r *ComputeEmbeddingsRequest) Dimensions() int {
	return r.dimensions
}

// Config holds all possible configuration for any gateway provider.
type Config struct {
	Provider Provider
	BaseURL  string // Used by Llama
	APIKey   string // Used by Gemini
}

// Option defines a function that configures the gateway.
// It can return an error if an option is invalid.
type Option func(c *Config) error

// WithProvider sets the LLM provider (e.g., Llama, Gemini, Nebius, OpenAI, OpenRouter, Anthropic, Voyage). This is a required option.
func WithProvider(p Provider) Option {
	return func(c *Config) error {
		switch p {
		case Llama, Gemini, Nebius, OpenAI, OpenRouter, Anthropic, OpenAICompatible, Voyage:
			c.Provider = p
		default:
			return fmt.Errorf("unsupported provider: %s", p)
		}
		return nil
	}
}

// WithBaseURL sets the base URL for a local Llama-compatible server.
func WithBaseURL(url string) Option {
	return func(c *Config) error {
		c.BaseURL = url
		return nil
	}
}

// WithAPIKey sets the API key directly.
func WithAPIKey(key string) Option {
	return func(c *Config) error {
		c.APIKey = key
		return nil
	}
}

// NewGenerationGateway creates a generation gateway based on the provided options.
func NewGenerationGateway(ctx context.Context, opts ...Option) (GenerationGateway, error) {
	cfg := &Config{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	switch cfg.Provider {
	case Gemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key")
		}
		return NewGeminiGateway(ctx, cfg.APIKey)
	case Llama:
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("llama provider requires a base URL")
		}
		return NewLlamaGateway(cfg.BaseURL)
	case Nebius:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("nebius provider requires an API key")
		}
		return NewOpenAIGateway(ctx, "https://api.studio.nebius.com/v1/", cfg.APIKey)
	case OpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key")
		}
		return NewOpenAIGateway(ctx, "https://api.openai.com/v1/", cfg.APIKey)
	case OpenRouter:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key")
		}
		return NewOpenAIGateway(ctx, "https://openrouter.ai/api/v1", cfg.APIKey)
	case Anthropic:
		return nil, fmt.Errorf("anthropic provider does not support content generation gateway (use voyage provider for embeddings)")
	case Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case OpenAICompatible:
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("openai-compatible provider requires a base URL")
		}
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai-compatible provider requires an API key")
		}
		return NewOpenAIGateway(ctx, cfg.BaseURL, cfg.APIKey)
	default:
		return nil, fmt.Errorf("a provider must be specified with WithProvider()")
	}
}

// NewEmbeddingGateway creates an embedding gateway based on the provided options.
func NewEmbeddingGateway(ctx context.Context, opts ...Option) (EmbeddingGateway, error) {
	cfg := &Config{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	switch cfg.Provider {
	case Gemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key")
		}
		return NewGeminiGateway(ctx, cfg.APIKey)
	case Llama:
		return nil, fmt.Errorf("llama embedding gateway is not yet implemented")
	case Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case Voyage:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("voyage provider requires an API key")
		}
		return NewVoyageGateway(cfg.APIKey)
	case Nebius:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("nebius provider requires an API key")
		}
		return NewOpenAIGateway(ctx, "https://api.studio.nebius.com/v1/", cfg.APIKey)
	default:
		return nil, fmt.Errorf("a provider must be specified with WithProvider()")
	}
}

type ComputeDistanceMixin struct {
}

// ComputeDistance calculates the cosine similarity between two embeddings.
// It returns a value between -1 (opposite) and 1 (identical), where 0 indicates orthogonality.
func (g *ComputeDistanceMixin) ComputeDistance(a, b Embedding) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length")
	}

	if len(a) == 0 || len(b) == 0 {
		return 0, fmt.Errorf("vectors must not be empty")
	}

	var dotProduct, aMagnitude, bMagnitude float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		aMagnitude += float64(a[i]) * float64(a[i])
		bMagnitude += float64(b[i]) * float64(b[i])
	}

	if aMagnitude == 0 || bMagnitude == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(aMagnitude) * math.Sqrt(bMagnitude)), nil
}
