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
	"time"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdLLM "github.com/retran/meowg1k/internal/models/llm"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"
	"github.com/stretchr/testify/assert"
)

func TestNewGatewayFactory(t *testing.T) {
	factory := NewGatewayFactory()
	assert.NotNil(t, factory)
	assert.IsType(t, &gatewayFactory{}, factory)
}

func TestGatewayFactory_NewGenerationGateway(t *testing.T) {
	factory := &gatewayFactory{}
	ctx := context.Background()

	tests := []struct {
		name        string
		profile     *mdProfile.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name: "OpenAI provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAI,
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAI,
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name: "Anthropic provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerUnknown,
			},
			expectError: false,
		},
		{
			name: "Anthropic provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "anthropic provider requires an API key",
		},
		{
			name: "Gemini provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Gemini,
				Model:           "gemini-1.5-flash",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerGemini,
			},
			expectError: false,
		},
		{
			name: "Gemini provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Gemini,
				Model:           "gemini-1.5-flash",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerGemini,
			},
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name: "Llama provider with base URL",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerLlama,
			},
			expectError: false,
		},
		{
			name: "Llama provider without base URL",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "llama provider requires a base URL",
		},
		{
			name: "OpenAI-compatible provider with base URL and API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI-compatible provider without base URL",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai-compatible provider requires a base URL",
		},
		{
			name: "OpenAI-compatible provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenRouter,
				Model:           "openrouter/auto",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenRouter,
				Model:           "openrouter/auto",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name: "Nebius provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Nebius,
				Model:           "meta-llama/Llama-3.1-70B-Instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerLlama,
			},
			expectError: false,
		},
		{
			name: "Nebius provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Nebius,
				Model:           "meta-llama/Llama-3.1-70B-Instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "nebius provider requires an API key",
		},
		{
			name: "Voyage provider (should fail for generation)",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "voyage provider only supports embeddings, not content generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.NewGenerationGateway(ctx, tt.profile)

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

func TestGatewayFactory_NewEmbeddingsGateway(t *testing.T) {
	factory := &gatewayFactory{}
	ctx := context.Background()

	tests := []struct {
		name        string
		profile     *mdProfile.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name: "OpenAI provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAI,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenAI,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name: "Gemini provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Gemini,
				Model:           "models/embedding-001",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerGemini,
			},
			expectError: false,
		},
		{
			name: "Gemini provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Gemini,
				Model:           "models/embedding-001",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerGemini,
			},
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name: "Voyage provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerUnknown,
			},
			expectError: false,
		},
		{
			name: "Voyage provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "voyage provider requires an API key",
		},
		{
			name: "OpenRouter provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenRouter,
				Model:           "openai/text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.OpenRouter,
				Model:           "openai/text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name: "Nebius provider with API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Nebius,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "Nebius provider without API key",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Nebius,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "nebius provider requires an API key",
		},
		{
			name: "Llama provider (not implemented)",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   mdLLM.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "llama embedding gateway is not yet implemented",
		},
		{
			name: "Anthropic provider (not supported)",
			profile: &mdProfile.ResolvedProfile{
				Provider:        mdGateway.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   mdLLM.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "anthropic provider does not provide embedding models",
		},
		{
			name:        "Nil profile",
			profile:     nil,
			expectError: true,
			errorMsg:    "profile cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.NewEmbeddingsGateway(ctx, tt.profile)

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
