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

package gateway

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/models/gateway"
	"github.com/stretchr/testify/assert"
)

func TestNewGatewayFactory(t *testing.T) {
	factory := NewGatewayFactory()
	assert.NotNil(t, factory)
	assert.IsType(t, &gatewayFactory{}, factory)
}

func TestGatewayFactory_buildConfig(t *testing.T) {
	factory := &gatewayFactory{}

	tests := []struct {
		name        string
		provider    gateway.Provider
		baseURL     string
		apiKey      string
		expectedCfg *gateway.Config
		expectError bool
	}{
		{
			name:     "Valid configuration with all parameters",
			provider: gateway.OpenAI,
			baseURL:  "https://api.openai.com/v1",
			apiKey:   "test-key",
			expectedCfg: &gateway.Config{
				Provider: gateway.OpenAI,
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "test-key",
			},
			expectError: false,
		},
		{
			name:     "Valid configuration with only provider and API key",
			provider: gateway.OpenAI,
			baseURL:  "",
			apiKey:   "test-key",
			expectedCfg: &gateway.Config{
				Provider: gateway.OpenAI,
				BaseURL:  "",
				APIKey:   "test-key",
			},
			expectError: false,
		},
		{
			name:     "Valid configuration with only provider",
			provider: gateway.OpenAI,
			baseURL:  "",
			apiKey:   "",
			expectedCfg: &gateway.Config{
				Provider: gateway.OpenAI,
				BaseURL:  "",
				APIKey:   "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := factory.buildConfig(tt.provider, tt.baseURL, tt.apiKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, tt.expectedCfg.Provider, cfg.Provider)
				assert.Equal(t, tt.expectedCfg.BaseURL, cfg.BaseURL)
				assert.Equal(t, tt.expectedCfg.APIKey, cfg.APIKey)
			}
		})
	}
}

func TestGatewayFactory_CreateGenerationGateway(t *testing.T) {
	factory := &gatewayFactory{}
	ctx := context.Background()

	tests := []struct {
		name        string
		provider    gateway.Provider
		baseURL     string
		apiKey      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "OpenAI provider with API key",
			provider:    gateway.OpenAI,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "OpenAI provider without API key",
			provider:    gateway.OpenAI,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name:        "Anthropic provider with API key",
			provider:    gateway.Anthropic,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Anthropic provider without API key",
			provider:    gateway.Anthropic,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "anthropic provider requires an API key",
		},
		{
			name:        "Gemini provider with API key",
			provider:    gateway.Gemini,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Gemini provider without API key",
			provider:    gateway.Gemini,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name:        "Llama provider with base URL",
			provider:    gateway.Llama,
			baseURL:     "http://localhost:8080",
			apiKey:      "",
			expectError: false,
		},
		{
			name:        "Llama provider without base URL",
			provider:    gateway.Llama,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "llama provider requires a base URL",
		},
		{
			name:        "OpenAI-compatible provider with base URL and API key",
			provider:    gateway.OpenAICompatible,
			baseURL:     "http://localhost:8080",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "OpenAI-compatible provider without base URL",
			provider:    gateway.OpenAICompatible,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: true,
			errorMsg:    "openai-compatible provider requires a base URL",
		},
		{
			name:        "OpenAI-compatible provider without API key",
			provider:    gateway.OpenAICompatible,
			baseURL:     "http://localhost:8080",
			apiKey:      "",
			expectError: true,
			errorMsg:    "openai-compatible provider requires an API key",
		},
		{
			name:        "OpenRouter provider with API key",
			provider:    gateway.OpenRouter,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "OpenRouter provider without API key",
			provider:    gateway.OpenRouter,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name:        "Nebius provider with API key",
			provider:    gateway.Nebius,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Nebius provider without API key",
			provider:    gateway.Nebius,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "nebius provider requires an API key",
		},
		{
			name:        "Voyage provider (should fail for generation)",
			provider:    gateway.Voyage,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: true,
			errorMsg:    "voyage provider only supports embeddings, not content generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.CreateGenerationGateway(ctx, tt.provider, tt.baseURL, tt.apiKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, gateway)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gateway)
			}
		})
	}
}

func TestGatewayFactory_CreateEmbeddingsGateway(t *testing.T) {
	factory := &gatewayFactory{}
	ctx := context.Background()

	tests := []struct {
		name        string
		provider    gateway.Provider
		baseURL     string
		apiKey      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "OpenAI provider with API key",
			provider:    gateway.OpenAI,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "OpenAI provider without API key",
			provider:    gateway.OpenAI,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name:        "Gemini provider with API key",
			provider:    gateway.Gemini,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Gemini provider without API key",
			provider:    gateway.Gemini,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name:        "Voyage provider with API key",
			provider:    gateway.Voyage,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Voyage provider without API key",
			provider:    gateway.Voyage,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "voyage provider requires an API key",
		},
		{
			name:        "OpenRouter provider with API key",
			provider:    gateway.OpenRouter,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "OpenRouter provider without API key",
			provider:    gateway.OpenRouter,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name:        "Nebius provider with API key",
			provider:    gateway.Nebius,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: false,
		},
		{
			name:        "Nebius provider without API key",
			provider:    gateway.Nebius,
			baseURL:     "",
			apiKey:      "",
			expectError: true,
			errorMsg:    "nebius provider requires an API key",
		},
		{
			name:        "Llama provider (not implemented)",
			provider:    gateway.Llama,
			baseURL:     "http://localhost:8080",
			apiKey:      "",
			expectError: true,
			errorMsg:    "llama embedding gateway is not yet implemented",
		},
		{
			name:        "Anthropic provider (not supported)",
			provider:    gateway.Anthropic,
			baseURL:     "",
			apiKey:      "test-key",
			expectError: true,
			errorMsg:    "anthropic provider does not provide embedding models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.CreateEmbeddingsGateway(ctx, tt.provider, tt.baseURL, tt.apiKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, gateway)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gateway)
			}
		})
	}
}
