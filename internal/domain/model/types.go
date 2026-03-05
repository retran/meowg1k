// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package model defines domain types for LLM model configurations and capabilities.
package model

import (
	"github.com/retran/meowg1k/internal/domain/provider"
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
	ID                   string
	Provider             provider.Provider
	Model                string
	BaseURL              string
	APIKey               string
	APIKeyEnv            string
	AppID                string
	EditorVersion        string
	EditorPluginVersion  string
	UserAgent            string
	CopilotIntegrationID string
	OpenAIOrganization   string
	Tokenizer            Tokenizer
	RateLimit            RateLimitConfig
	MaxInputTokens       int
	MaxOutputTokens      int
}

// Tokenizer represents different tokenizer implementations.
type Tokenizer string

const (
	// TokenizerCL100K is the tokenizer used by GPT-4 and similar OpenAI models.
	TokenizerCL100K Tokenizer = "cl100k_base"
	// TokenizerGPT2 is the tokenizer used by older OpenAI models.
	TokenizerGPT2 Tokenizer = "gpt2"
	// TokenizerSentencePiece is used by many open-source models.
	TokenizerSentencePiece Tokenizer = "sentencepiece"
	// TokenizerTikToken is the general tiktoken tokenizer.
	TokenizerTikToken Tokenizer = "tiktoken"
	// TokenizerGemini is used by Google Gemini models.
	TokenizerGemini Tokenizer = "gemini"
	// TokenizerLlama is used by Llama models.
	TokenizerLlama Tokenizer = "llama"
	// TokenizerUnknown is used when the tokenizer type is unknown.
	TokenizerUnknown Tokenizer = "unknown"
)

// Info contains comprehensive information about a specific AI model.
type Info struct {
	Provider              string
	TokenizerType         Tokenizer
	Description           string
	MaxContextTokens      int
	MaxOutputTokens       int
	DefaultEmbedDimension int
}
