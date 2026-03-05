// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package provider provides services for managing LLM provider configurations (OpenAI, Anthropic, Gemini, etc.).
package provider

import (
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/domain/provider"
)

// Service is the concrete implementation of the registry service.
type Service struct {
	providers map[provider.Provider]provider.Definition
}

// NewService creates a new provider registry service with default providers.
func NewService() *Service {
	s := &Service{
		providers: map[provider.Provider]provider.Definition{
			provider.Gemini: {
				Type:            provider.Gemini,
				Name:            "Google Gemini",
				DefaultEnvVar:   "MEOW_GEMINI_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  1000000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
				Tokenizer:       "gemini",
			},
			provider.OpenAI: {
				Type:            provider.OpenAI,
				Name:            "OpenAI",
				DefaultBaseURL:  "https://api.openai.com/v1",
				DefaultEnvVar:   "MEOW_OPENAI_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  128000,
				MaxOutputTokens: 16384,
				DefaultTimeout:  5 * time.Minute,
				Tokenizer:       "cl100k_base",
			},
			provider.Anthropic: {
				Type:            provider.Anthropic,
				Name:            "Anthropic Claude",
				DefaultEnvVar:   "MEOW_ANTHROPIC_API_KEY",
				RequiresAPIKey:  true,
				RequiresBaseURL: false,
				MaxInputTokens:  200000,
				MaxOutputTokens: 8192,
				DefaultTimeout:  5 * time.Minute,
				Tokenizer:       "cl100k_base",
			},
			provider.Llama: {
				Type:            provider.Llama,
				Name:            "Meta Llama",
				DefaultEnvVar:   "", // Llama typically doesn't use API keys
				RequiresAPIKey:  false,
				RequiresBaseURL: true,
				MaxInputTokens:  128000,
				MaxOutputTokens: 4096,
				DefaultTimeout:  10 * time.Minute,
				Tokenizer:       "llama",
			},
			provider.OpenRouter: {
				Type:            provider.OpenRouter,
				Name:            "OpenRouter",
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
			provider.GitHubCopilot: {
				Type:            provider.GitHubCopilot,
				Name:            "GitHub Copilot",
				DefaultBaseURL:  "https://api.githubcopilot.com",
				DefaultEnvVar:   "",
				RequiresAPIKey:  false,
				RequiresBaseURL: false,
				MaxInputTokens:  200000,
				MaxOutputTokens: 64000,
				DefaultTimeout:  5 * time.Minute,
				Tokenizer:       "cl100k_base",
			},
		},
	}

	return s
}

// Get retrieves a provider definition by provider type.
func (s *Service) Get(providerType provider.Provider) (provider.Definition, error) {
	if s == nil {
		return provider.Definition{}, fmt.Errorf("provider service is nil")
	}

	providerDefinition, exists := s.providers[providerType]
	if !exists {
		return provider.Definition{}, fmt.Errorf("provider not found: %s", providerType)
	}

	return providerDefinition, nil
}
