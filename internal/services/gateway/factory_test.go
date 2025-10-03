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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/services/llm"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/services/provider"
)

func TestNewGatewayFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.IsType(t, &gatewayFactory{}, factory)
}

func TestGatewayFactory_NewGenerationGateway(t *testing.T) {
	factory := NewFactory()
	ctx := context.Background()

	tests := []struct {
		name        string
		profile     *profile.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name: "OpenAI provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name: "Anthropic provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerUnknown,
			},
			expectError: false,
		},
		{
			name: "Anthropic provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "anthropic API key is required",
		},
		{
			name: "Gemini provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "gemini-1.5-flash",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerGemini,
			},
			expectError: false,
		},
		{
			name: "Gemini provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "gemini-1.5-flash",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerGemini,
			},
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name: "Llama provider with base URL",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   llm.TokenizerLlama,
			},
			expectError: false,
		},
		{
			name: "Llama provider without base URL",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "llama provider requires a base URL",
		},
		{
			name: "OpenAI-compatible provider with base URL and API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI-compatible provider without base URL",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai-compatible provider requires a base URL",
		},
		{
			name: "OpenAI-compatible provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openrouter/auto",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openrouter/auto",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name: "Voyage provider (should fail for generation)",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "voyage provider only supports embeddings, not content generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.NewGenerationGateway(ctx, tt.profile)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, gateway)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
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
		profile     *profile.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name: "OpenAI provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name: "Gemini provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "models/embedding-001",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerGemini,
			},
			expectError: false,
		},
		{
			name: "Gemini provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "models/embedding-001",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerGemini,
			},
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name: "Voyage provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerUnknown,
			},
			expectError: false,
		},
		{
			name: "Voyage provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "voyage provider requires an API key",
		},
		{
			name: "OpenRouter provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openai/text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openai/text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   llm.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name: "Llama provider (not implemented)",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   llm.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "llama embedding gateway is not yet implemented",
		},
		{
			name: "Anthropic provider (not supported)",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   llm.TokenizerUnknown,
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
				require.Error(t, err)
				assert.Nil(t, gateway)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, gateway)
			}
		})
	}
}
