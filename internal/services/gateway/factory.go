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
	"errors"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"
)

var (
	ErrGeminiAPIKeyRequired            = errors.New("gemini provider requires an API key")
	ErrLlamaBaseURLRequired            = errors.New("llama provider requires a base URL")
	ErrOpenAIAPIKeyRequired            = errors.New("openai provider requires an API key")
	ErrOpenRouterAPIKeyRequired        = errors.New("openrouter provider requires an API key")
	ErrVoyageNoContentGeneration       = errors.New("voyage provider only supports embeddings, not content generation")
	ErrOpenAICompatibleBaseURLRequired = errors.New("openai-compatible provider requires a base URL")
	ErrProviderNotSpecified            = errors.New("a provider must be specified with WithProvider()")
	ErrProfileCannotBeNil              = errors.New("profile cannot be nil")
	ErrLlamaEmbeddingsNotImplemented   = errors.New("llama embedding gateway is not yet implemented")
	ErrAnthropicNoEmbeddings           = errors.New(
		"anthropic provider does not provide embedding models " +
			"(use voyage provider for embeddings recommended by Anthropic)",
	)
	ErrVoyageAPIKeyRequired = errors.New("voyage provider requires an API key")
)

// Factory is the interface for creating LLM gateways.
type Factory interface {
	// NewGenerationGateway creates a new generation gateway based on the provided profile.
	NewGenerationGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (GenerationGateway, error)
	// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
	NewEmbeddingsGateway(ctx context.Context, profile *mdProfile.ResolvedProfile) (EmbeddingsGateway, error)
}

// gatewayFactory is the implementation of GatewayFactory.
type gatewayFactory struct{}

// NewFactory creates a new gateway factory.
func NewFactory() Factory {
	return &gatewayFactory{}
}

// NewGenerationGateway creates a new generation gateway based on the provided profile.
func (f *gatewayFactory) NewGenerationGateway(
	ctx context.Context,
	profile *mdProfile.ResolvedProfile,
) (GenerationGateway, error) {
	switch profile.Provider {
	case mdGateway.Gemini:
		if profile.APIKey == "" {
			return nil, ErrGeminiAPIKeyRequired
		}
		return newGeminiGateway(ctx, profile.APIKey)
	case mdGateway.Llama:
		if profile.BaseURL == "" {
			return nil, ErrLlamaBaseURLRequired
		}
		return newLlamaGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.OpenAI:
		if profile.APIKey == "" {
			return nil, ErrOpenAIAPIKeyRequired
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.OpenRouter:
		if profile.APIKey == "" {
			return nil, ErrOpenRouterAPIKeyRequired
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.Anthropic:
		if profile.APIKey == "" {
			return nil, ErrAnthropicAPIKeyRequired
		}
		return newAnthropicGateway(profile.APIKey)
	case mdGateway.Voyage:
		return nil, ErrVoyageNoContentGeneration
	case mdGateway.OpenAICompatible:
		if profile.BaseURL == "" {
			return nil, ErrOpenAICompatibleBaseURLRequired
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	default:
		return nil, ErrProviderNotSpecified
	}
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
func (f *gatewayFactory) NewEmbeddingsGateway(
	ctx context.Context,
	profile *mdProfile.ResolvedProfile,
) (EmbeddingsGateway, error) {
	if profile == nil {
		return nil, ErrProfileCannotBeNil
	}

	switch profile.Provider {
	case mdGateway.Gemini:
		if profile.APIKey == "" {
			return nil, ErrGeminiAPIKeyRequired
		}
		return newGeminiGateway(ctx, profile.APIKey)
	case mdGateway.Llama:
		return nil, ErrLlamaEmbeddingsNotImplemented
	case mdGateway.Anthropic:
		return nil, ErrAnthropicNoEmbeddings
	case mdGateway.OpenAI:
		if profile.APIKey == "" {
			return nil, ErrOpenAIAPIKeyRequired
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.OpenRouter:
		if profile.APIKey == "" {
			return nil, ErrOpenRouterAPIKeyRequired
		}
		return newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case mdGateway.Voyage:
		if profile.APIKey == "" {
			return nil, ErrVoyageAPIKeyRequired
		}
		return newVoyageGateway(profile.APIKey)
	default:
		return nil, ErrProviderNotSpecified
	}
}
