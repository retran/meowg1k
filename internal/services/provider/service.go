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

// Package provider implements a registry for LLM providers.
package provider

import (
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/services/llm"
)

// Provider service errors
var (
	ErrProviderNotFound = errors.New("provider not found")
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

// ProviderDefinition defines the characteristics of a provider.
type ProviderDefinition struct {
	Type            Provider          `json:"type"`
	Name            string            `json:"name"`
	DefaultModel    string            `json:"default_model"`
	DefaultBaseURL  string            `json:"default_base_url"`
	DefaultEnvVar   string            `json:"default_env_var"`
	RequiresAPIKey  bool              `json:"requires_api_key"`
	RequiresBaseURL bool              `json:"requires_base_url"`
	TokenizerType   llm.TokenizerType `json:"tokenizer_type"`
	MaxInputTokens  int               `json:"max_input_tokens"`
	MaxOutputTokens int               `json:"max_output_tokens"`
	DefaultTimeout  time.Duration     `json:"default_timeout"`
}

// Service is the concrete implementation of the registry service.
type Service struct {
	providers map[Provider]ProviderDefinition
}

// NewService creates a new provider registry service with default providers.
func NewService() *Service {
	s := &Service{
		providers: map[Provider]ProviderDefinition{
			Gemini: {
				Type:            Gemini,
				Name:            "Google Gemini",
				DefaultModel:    "gemini-2.5-flash",
				DefaultEnvVar:   "MEOW_GEMINI_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				TokenizerType:   llm.TokenizerGemini,
				MaxInputTokens:  1000000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
			},
			OpenAI: {
				Type:            OpenAI,
				Name:            "OpenAI",
				DefaultModel:    "gpt-4o-mini",
				DefaultBaseURL:  "https://api.openai.com/v1",
				DefaultEnvVar:   "MEOW_OPENAI_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				TokenizerType:   llm.TokenizerCL100K,
				MaxInputTokens:  128000,
				MaxOutputTokens: 16384,
				DefaultTimeout:  5 * time.Minute,
			},
			Anthropic: {
				Type:            Anthropic,
				Name:            "Anthropic Claude",
				DefaultModel:    "claude-3-5-haiku-20241022",
				DefaultEnvVar:   "MEOW_ANTHROPIC_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				TokenizerType:   llm.TokenizerCL100K,
				MaxInputTokens:  200000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
			},
			Llama: {
				Type:            Llama,
				Name:            "Meta Llama",
				DefaultModel:    "llama3.2:3b",
				DefaultEnvVar:   "", // Llama typically doesn't use API keys
				RequiresAPIKey:  false,
				RequiresBaseURL: true,
				TokenizerType:   llm.TokenizerLlama,
				MaxInputTokens:  128000,
				MaxOutputTokens: 4096,
				DefaultTimeout:  10 * time.Minute,
			},
			OpenRouter: {
				Type:            OpenRouter,
				Name:            "OpenRouter",
				DefaultModel:    "anthropic/claude-3.5-haiku",
				DefaultBaseURL:  "https://openrouter.ai/api/v1",
				DefaultEnvVar:   "MEOW_OPENROUTER_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				TokenizerType:   llm.TokenizerCL100K,
				MaxInputTokens:  200000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
			},
			Voyage: {
				Type:            Voyage,
				Name:            "Voyage AI",
				DefaultModel:    "voyage-3",
				DefaultBaseURL:  "https://api.voyageai.com/v1",
				DefaultEnvVar:   "MEOW_VOYAGE_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				TokenizerType:   llm.TokenizerCL100K,
				MaxInputTokens:  32000,
				MaxOutputTokens: 0, // Embeddings don't have output tokens
				DefaultTimeout:  5 * time.Minute,
			},
			OpenAICompatible: {
				Type:            OpenAICompatible,
				Name:            "OpenAI Compatible",
				DefaultModel:    "",    // Must be specified by user
				DefaultEnvVar:   "",    // Depends on the service
				RequiresAPIKey:  false, // Depends on the service
				RequiresBaseURL: true,
				TokenizerType:   llm.TokenizerCL100K,
				MaxInputTokens:  128000,
				MaxOutputTokens: 4096,
				DefaultTimeout:  5 * time.Minute,
			},
		},
	}

	return s
}

// Get retrieves a provider definition by provider type.
func (s *Service) Get(providerType Provider) (ProviderDefinition, error) {
	provider, exists := s.providers[providerType]
	if !exists {
		return ProviderDefinition{}, fmt.Errorf("%w: %s", ErrProviderNotFound, providerType)
	}

	return provider, nil
}
