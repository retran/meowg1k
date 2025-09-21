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
	"fmt"
)

// GatewayFactory is the interface for creating LLM gateways.
type GatewayFactory interface {
	CreateGenerationGateway(ctx context.Context, provider Provider, baseURL, apiKey string) (GenerationGateway, error)
	CreateEmbeddingsGateway(ctx context.Context, provider Provider, baseURL, apiKey string) (EmbeddingsGateway, error)
}

// gatewayFactory is the implementation of GatewayFactory.
type gatewayFactory struct{}

// NewGatewayFactory creates a new gateway factory.
func NewGatewayFactory() GatewayFactory {
	return &gatewayFactory{}
}

// buildConfig builds the configuration from the provided parameters.
func (f *gatewayFactory) buildConfig(provider Provider, baseURL, apiKey string) (*Config, error) {
	opts := []Option{
		WithProvider(provider),
	}

	// Add baseURL for providers that need it
	if baseURL != "" {
		opts = append(opts, WithBaseURL(baseURL))
	}

	// Add API key for providers that need it
	if apiKey != "" {
		opts = append(opts, WithAPIKey(apiKey))
	}

	cfg := &Config{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// CreateGenerationGateway creates a gateway with options based on the provided parameters.
func (f *gatewayFactory) CreateGenerationGateway(ctx context.Context, provider Provider, baseURL, apiKey string) (GenerationGateway, error) {
	cfg, err := f.buildConfig(provider, baseURL, apiKey)
	if err != nil {
		return nil, err
	}

	switch cfg.Provider {
	case Gemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key")
		}
		return newGeminiGateway(ctx, cfg.APIKey)
	case Llama:
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("llama provider requires a base URL")
		}
		return newLlamaGateway(cfg.BaseURL, cfg.APIKey)
	case Nebius:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("nebius provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	case OpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	case OpenRouter:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	case Anthropic:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("anthropic provider requires an API key")
		}
		return newAnthropicGateway(cfg.APIKey)
	case Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case OpenAICompatible:
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("openai-compatible provider requires a base URL")
		}
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai-compatible provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	default:
		return nil, fmt.Errorf("a provider must be specified with WithProvider()")
	}
}

// CreateEmbeddingsGateway creates an embedding gateway with options based on the provided parameters.
func (f *gatewayFactory) CreateEmbeddingsGateway(ctx context.Context, provider Provider, baseURL, apiKey string) (EmbeddingsGateway, error) {
	cfg, err := f.buildConfig(provider, baseURL, apiKey)
	if err != nil {
		return nil, err
	}

	switch cfg.Provider {
	case Gemini:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key")
		}
		return newGeminiGateway(ctx, cfg.APIKey)
	case Llama:
		return nil, fmt.Errorf("llama embedding gateway is not yet implemented")
	case Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case OpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	case OpenRouter:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	case Voyage:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("voyage provider requires an API key")
		}
		return newVoyageGateway(cfg.APIKey)
	case Nebius:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("nebius provider requires an API key")
		}
		return newOpenAIGateway(cfg.BaseURL, cfg.APIKey)
	default:
		return nil, fmt.Errorf("a provider must be specified with WithProvider()")
	}
}
