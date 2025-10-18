// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package profile defines domain types for LLM provider profiles with rate limits and cost tracking.
package profile

import (
	"time"

	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/provider"
)

// Profile defines an enumeration for configured profile names.
type Profile string

// ResolvedProfile represents a profile with all values resolved from both model and profile config.
type ResolvedProfile struct {
	// Profile information
	Name string

	// Model instance information (from model config)
	ModelID         string                // Model instance ID
	Provider        provider.Provider     // Provider type
	Model           string                // Model name
	MaxInputTokens  int                   // Maximum input tokens
	MaxOutputTokens int                   // Maximum output tokens (can be overridden by profile)
	BaseURL         string                // API base URL
	APIKey          string                // Resolved API key (actual value)
	APIKeyEnv       string                // Environment variable name for API key
	TokenizerType   model.Tokenizer       // Tokenizer type
	RateLimit       model.RateLimitConfig // Rate limiting config

	// Request-specific parameters (from profile config)
	Timeout           time.Duration          // Request timeout
	Temperature       *float64               // Temperature parameter (optional)
	TopP              *float64               // TopP parameter (optional)
	TopK              *int                   // TopK parameter (optional)
	FrequencyPenalty  *float64               // Frequency penalty parameter (optional)
	PresencePenalty   *float64               // Presence penalty parameter (optional)
	Seed              *int                   // Random seed for deterministic sampling (optional)
	Stop              []string               // Stop sequences (optional)
	ResponseFormat    *string                // Response format (e.g., "text", "json_object", "json_schema")
	ResponseSchema    map[string]interface{} // JSON schema for structured output (optional)
	CandidateCount    *int                   // Number of response candidates to generate (optional)
	LogProbs          *bool                  // Enable log probabilities (optional)
	TopLogProbs       *int                   // Number of top log probabilities per token (optional)
	LogitBias         map[string]int         // Token likelihood modifiers (optional)
	ServiceTier       *string                // Service tier for the request (optional)
	User              *string                // End-user identifier (optional)
	RepetitionPenalty *float64               // Repetition penalty (OpenRouter, Llama.cpp)
	MinP              *float64               // Minimum probability threshold (OpenRouter, Llama.cpp)
	TopA              *float64               // Top-A filtering (OpenRouter)
	TypicalP          *float64               // Typical sampling parameter (Llama.cpp)
	Mirostat          *int                   // Mirostat sampling mode (Llama.cpp)
	MirostatTau       *float64               // Mirostat target entropy (Llama.cpp)
	MirostatEta       *float64               // Mirostat learning rate (Llama.cpp)
	Grammar           *string                // Grammar constraints (Llama.cpp)

	// Cache configuration (merged from global and profile-specific settings)
	CacheEnabled bool          // Whether caching is enabled for this profile
	CacheTTL     time.Duration // Cache TTL for this profile
}
