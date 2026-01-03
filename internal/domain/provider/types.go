// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package provider defines domain types for LLM provider configurations (OpenAI, Anthropic, Gemini, etc.).
package provider

import (
	"time"
)

// Provider defines an enumeration for supported LLM providers.
type Provider string

const (
	// Llama identifies the Llama provider.
	Llama Provider = "llama"
	// Gemini identifies the Gemini provider.
	Gemini Provider = "gemini"
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

// Definition defines the characteristics of a provider.
type Definition struct {
	Type            Provider      `json:"type"`
	Name            string        `json:"name"`
	DefaultModel    string        `json:"default_model"`
	DefaultBaseURL  string        `json:"default_base_url"`
	DefaultEnvVar   string        `json:"default_env_var"`
	Tokenizer       string        `json:"tokenizer"`
	MaxInputTokens  int           `json:"max_input_tokens"`
	MaxOutputTokens int           `json:"max_output_tokens"`
	DefaultTimeout  time.Duration `json:"default_timeout"`
	RequiresAPIKey  bool          `json:"requires_api_key"`
	RequiresBaseURL bool          `json:"requires_base_url"`
}
