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
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/core/provider"
)

// Service is the concrete implementation of the registry service.
type Service struct {
	providers map[provider.Provider]provider.ProviderDefinition
}

// NewService creates a new provider registry service with default providers.
func NewService() *Service {
	s := &Service{
		providers: map[provider.Provider]provider.ProviderDefinition{
			provider.Gemini: {
				Type:            provider.Gemini,
				Name:            "Google Gemini",
				DefaultModel:    "gemini-2.5-flash",
				DefaultEnvVar:   "MEOW_GEMINI_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  1000000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
			},
			provider.OpenAI: {
				Type:            provider.OpenAI,
				Name:            "OpenAI",
				DefaultModel:    "gpt-4o-mini",
				DefaultBaseURL:  "https://api.openai.com/v1",
				DefaultEnvVar:   "MEOW_OPENAI_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  128000,
				MaxOutputTokens: 16384,
				DefaultTimeout:  5 * time.Minute,
			},
			provider.Anthropic: {
				Type:            provider.Anthropic,
				Name:            "Anthropic Claude",
				DefaultModel:    "claude-3-5-haiku-20241022",
				DefaultEnvVar:   "MEOW_ANTHROPIC_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  200000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
			},
			provider.Llama: {
				Type:            provider.Llama,
				Name:            "Meta Llama",
				DefaultModel:    "llama3.2:3b",
				DefaultEnvVar:   "", // Llama typically doesn't use API keys
				RequiresAPIKey:  false,
				RequiresBaseURL: true,
				MaxInputTokens:  128000,
				MaxOutputTokens: 4096,
				DefaultTimeout:  10 * time.Minute,
			},
			provider.OpenRouter: {
				Type:            provider.OpenRouter,
				Name:            "OpenRouter",
				DefaultModel:    "anthropic/claude-3.5-haiku",
				DefaultBaseURL:  "https://openrouter.ai/api/v1",
				DefaultEnvVar:   "MEOW_OPENROUTER_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  200000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
			},
			provider.Voyage: {
				Type:            provider.Voyage,
				Name:            "Voyage AI",
				DefaultModel:    "voyage-3",
				DefaultBaseURL:  "https://api.voyageai.com/v1",
				DefaultEnvVar:   "MEOW_VOYAGE_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  32000,
				MaxOutputTokens: 0, // Embeddings don't have output tokens
				DefaultTimeout:  5 * time.Minute,
			},
			provider.OpenAICompatible: {
				Type:            provider.OpenAICompatible,
				Name:            "OpenAI Compatible",
				DefaultModel:    "",    // Must be specified by user
				DefaultEnvVar:   "",    // Depends on the service
				RequiresAPIKey:  false, // Depends on the service
				RequiresBaseURL: true,
				MaxInputTokens:  128000,
				MaxOutputTokens: 4096,
				DefaultTimeout:  5 * time.Minute,
			},
		},
	}

	return s
}

// Get retrieves a provider definition by provider type.
func (s *Service) Get(providerType provider.Provider) (provider.ProviderDefinition, error) {
	if s == nil {
		return provider.ProviderDefinition{}, fmt.Errorf("provider service is nil")
	}

	providerDefinition, exists := s.providers[providerType]
	if !exists {
		return provider.ProviderDefinition{}, fmt.Errorf("provider not found: %s", providerType)
	}

	return providerDefinition, nil
}
