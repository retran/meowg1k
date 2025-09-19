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
	"os"
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
	model    string
	chunks   []string
	taskType TaskType
}

// NewComputeEmbeddingRequest creates and returns a new ComputeEmbeddingRequest.
func NewComputeEmbeddingRequest(model string, chunks []string, taskType TaskType) *ComputeEmbeddingsRequest {
	return &ComputeEmbeddingsRequest{
		model:    model,
		chunks:   chunks,
		taskType: taskType,
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

// Config holds all possible configuration for any gateway provider.
type Config struct {
	Provider Provider
	BaseURL  string // Used by Llama
	APIKey   string // Used by Gemini
}

// Option defines a function that configures the gateway.
// It can return an error if an option is invalid.
type Option func(c *Config) error

// WithProvider sets the LLM provider (e.g., Llama, Gemini, Nebius). This is a required option.
func WithProvider(p Provider) Option {
	return func(c *Config) error {
		switch p {
		case Llama, Gemini, Nebius:
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

// WithGeminiAPIKey sets the API key for the Gemini provider.
// If the key is empty, it attempts to load it from the MEOW_GEMINI_API_KEY environment variable.
func WithGeminiAPIKey(key string) Option {
	return func(c *Config) error {
		if key != "" {
			c.APIKey = key
			return nil
		}
		envKey := os.Getenv("MEOW_GEMINI_API_KEY")
		if envKey == "" {
			return fmt.Errorf("gemini API key is not provided and MEOW_GEMINI_API_KEY environment variable is not set")
		}
		c.APIKey = envKey
		return nil
	}
}

// WithNebiusAPIKey sets the API key for the Nebius AI Studio provider.
// If the key is empty, it attempts to load it from the MEOW_NEBIUS_API_KEY environment variable.
func WithNebiusAPIKey(key string) Option {
	return func(c *Config) error {
		if key != "" {
			c.APIKey = key
			return nil
		}
		envKey := os.Getenv("MEOW_NEBIUS_API_KEY")
		if envKey == "" {
			return fmt.Errorf("nebius AI Studio API key is not provided and MEOW_NEBIUS_API_KEY environment variable is not set")
		}
		c.APIKey = envKey
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
			if err := WithGeminiAPIKey("")(cfg); err != nil {
				return nil, err
			}
		}
		return NewGeminiGateway(ctx, cfg.APIKey)
	case Llama:
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("llama provider requires a base URL")
		}
		return NewLlamaGateway(cfg.BaseURL)
	case Nebius:
		if cfg.APIKey == "" {
			if err := WithNebiusAPIKey("")(cfg); err != nil {
				return nil, err
			}
		}
		return NewOpenAIGateway(ctx, "https://api.studio.nebius.com/v1/", cfg.APIKey)
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
			if err := WithGeminiAPIKey("")(cfg); err != nil {
				return nil, err
			}
		}
		return NewGeminiGateway(ctx, cfg.APIKey)
	case Llama:
		return nil, fmt.Errorf("llama embedding gateway is not yet implemented")
	case Nebius:
		if cfg.APIKey == "" {
			if err := WithNebiusAPIKey("")(cfg); err != nil {
				return nil, err
			}
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

	if (len(a) == 0) || (len(b) == 0) {
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
