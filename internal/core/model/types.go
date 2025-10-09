package model

import (
	"github.com/retran/meowg1k/internal/core/provider"
)

// Model defines an enumeration for configured model instance names.
type Model string

// RateLimitConfig contains rate limiting configuration for a model instance.
type RateLimitConfig struct {
	RequestsPerMinute int
	TokensPerMinute   int
	RequestsPerDay    int
}

// ResolvedModel represents a model instance with all values resolved.
type ResolvedModel struct {
	ID              string            // Model instance ID from config
	Provider        provider.Provider // Resolved provider
	Model           string            // Model name
	MaxInputTokens  int               // Maximum input tokens
	MaxOutputTokens int               // Maximum output tokens
	BaseURL         string            // API base URL
	APIKey          string            // Resolved API key (actual value)
	APIKeyEnv       string            // Environment variable name for API key
	TokenizerType   TokenizerType     // Tokenizer type
	RateLimit       RateLimitConfig   // Rate limiting config
}

// TokenizerType represents different tokenizer implementations.
type TokenizerType string

const (
	// TokenizerCL100K is the tokenizer used by GPT-4 and similar OpenAI models.
	TokenizerCL100K TokenizerType = "cl100k_base"
	// TokenizerGPT2 is the tokenizer used by older OpenAI models.
	TokenizerGPT2 TokenizerType = "gpt2"
	// TokenizerSentencePiece is used by many open-source models.
	TokenizerSentencePiece TokenizerType = "sentencepiece"
	// TokenizerTikToken is the general tiktoken tokenizer.
	TokenizerTikToken TokenizerType = "tiktoken"
	// TokenizerGemini is used by Google Gemini models.
	TokenizerGemini TokenizerType = "gemini"
	// TokenizerLlama is used by Llama models.
	TokenizerLlama TokenizerType = "llama"
	// TokenizerUnknown is used when the tokenizer type is unknown.
	TokenizerUnknown TokenizerType = "unknown"
)

// ModelInfo contains comprehensive information about a specific AI model.
type ModelInfo struct {
	Provider              string        // The provider offering this model
	MaxContextTokens      int           // Maximum number of context tokens
	MaxOutputTokens       int           // Maximum number of output tokens (0 if not limited)
	TokenizerType         TokenizerType // Type of tokenizer used
	Description           string        // Human-readable description
	DefaultEmbedDimension int           // Default embedding dimension (0 if not applicable)
}
