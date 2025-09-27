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

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"
)

// GatewayFactory is the interface for creating LLM gateways.
type GatewayFactory interface {
	// NewGenerationGateway creates a new generation gateway based on the provided profile.
	NewGenerationGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (GenerationGateway, error)
	// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
	NewEmbeddingsGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (EmbeddingsGateway, error)
}

// gatewayFactory is the implementation of GatewayFactory.
type gatewayFactory struct {
	GatewayFactory
}

// NewGatewayFactory creates a new gateway factory.
func NewGatewayFactory() GatewayFactory {
	return &gatewayFactory{}
}

// NewGenerationGateway creates a new generation gateway based on the provided profile.
func (f *gatewayFactory) NewGenerationGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (GenerationGateway, error) {
	switch profile.Provider {
	case mdGateway.Gemini:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key")
		}
		return newGeminiGateway(ctx, profile.APIKey)
	case mdGateway.Llama:
		if profile.BaseURL == "" {
			return nil, fmt.Errorf("llama provider requires a base URL")
		}
		return newLlamaGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.OpenAI:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key")
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.OpenRouter:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key")
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.Anthropic:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("anthropic provider requires an API key")
		}
		return newAnthropicGateway(profile.APIKey)
	case mdGateway.Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case mdGateway.OpenAICompatible:
		if profile.BaseURL == "" {
			return nil, fmt.Errorf("openai-compatible provider requires a base URL")
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	default:
		return nil, fmt.Errorf("a provider must be specified with WithProvider()")
	}
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
func (f *gatewayFactory) NewEmbeddingsGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (EmbeddingsGateway, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	switch profile.Provider {
	case mdGateway.Gemini:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key")
		}
		return newGeminiGateway(ctx, profile.APIKey)
	case mdGateway.Llama:
		return nil, fmt.Errorf("llama embedding gateway is not yet implemented")
	case mdGateway.Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case mdGateway.OpenAI:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key")
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.OpenRouter:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key")
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.Voyage:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("voyage provider requires an API key")
		}
		return newVoyageGateway(profile.APIKey)
	default:
		return nil, fmt.Errorf("a provider must be specified with WithProvider()")
	}
}
