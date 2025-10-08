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
	"sync"

	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/services/provider"
	"github.com/retran/meowg1k/pkg/ratelimit"
)

var (
	ErrGeminiAPIKeyRequired            = errors.New("gemini provider requires an API key")
	ErrLlamaBaseURLRequired            = errors.New("llama provider requires a base URL")
	ErrOpenAIAPIKeyRequired            = errors.New("openai provider requires an API key")
	ErrOpenRouterAPIKeyRequired        = errors.New("openrouter provider requires an API key")
	ErrVoyageNoContentGeneration       = errors.New("voyage provider only supports embeddings, not content generation")
	ErrOpenAICompatibleBaseURLRequired = errors.New("openai-compatible provider requires a base URL")
	ErrProviderNotSpecified            = errors.New("a provider must be specified with Withgateway.Provider()")
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
	NewGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (GenerationGateway, error)
	// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
	NewEmbeddingsGateway(ctx context.Context, profile *profile.ResolvedProfile) (EmbeddingsGateway, error)
}

// gatewayFactory is the implementation of GatewayFactory.
type gatewayFactory struct {
	mu       sync.Mutex
	limiters map[string]*ratelimit.Limiter // key is profile name
	repo     ratelimit.Repository          // database repository for rate limiting
}

// NewFactory creates a new gateway factory.
func NewFactory(repo ratelimit.Repository) Factory {
	return &gatewayFactory{
		limiters: make(map[string]*ratelimit.Limiter),
		repo:     repo,
	}
}

// getRateLimiter returns or creates a rate limiter for the given profile.
// This method is thread-safe. Each unique model instance gets its own rate limiter.
// The key is based on provider:baseURL:model:apiKeyEnv to ensure different API keys
// or endpoints get separate rate limiters.
func (f *gatewayFactory) getRateLimiter(profile *profile.ResolvedProfile) *ratelimit.Limiter {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate key based on model instance characteristics
	// IMPORTANT: Use APIKeyEnv (environment variable name), never the actual API key
	key := string(profile.Provider) + ":" + profile.BaseURL + ":" + profile.Model + ":" + profile.APIKeyEnv

	if limiter, exists := f.limiters[key]; exists {
		return limiter
	}

	config := ratelimit.Config{
		RequestsPerMinute: profile.RateLimit.RequestsPerMinute,
		TokensPerMinute:   profile.RateLimit.TokensPerMinute,
		RequestsPerDay:    profile.RateLimit.RequestsPerDay,
	}

	// If no limits are set, use unlimited
	if config.RequestsPerMinute == 0 && config.TokensPerMinute == 0 && config.RequestsPerDay == 0 {
		config = ratelimit.Unlimited
	}

	limiter, err := ratelimit.NewLimiter(key, config, f.repo)
	if err != nil {
		// If we can't create a limiter, return an unlimited one
		// This ensures the gateway can still function
		limiter, _ = ratelimit.NewLimiter(key, ratelimit.Unlimited, f.repo)
	}

	f.limiters[key] = limiter

	return limiter
}

// NewGenerationGateway creates a new generation gateway based on the provided profile.
func (f *gatewayFactory) NewGenerationGateway(
	ctx context.Context,
	profile *profile.ResolvedProfile,
) (GenerationGateway, error) {
	var gateway GenerationGateway
	var err error

	switch profile.Provider {
	case provider.Gemini:
		if profile.APIKey == "" {
			return nil, ErrGeminiAPIKeyRequired
		}

		gateway, err = newGeminiGateway(ctx, profile.APIKey)
	case provider.Llama:
		if profile.BaseURL == "" {
			return nil, ErrLlamaBaseURLRequired
		}

		gateway, err = newLlamaGateway(profile.BaseURL, profile.APIKey)
	case provider.OpenAI:
		if profile.APIKey == "" {
			return nil, ErrOpenAIAPIKeyRequired
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case provider.OpenRouter:
		if profile.APIKey == "" {
			return nil, ErrOpenRouterAPIKeyRequired
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case provider.Anthropic:
		if profile.APIKey == "" {
			return nil, ErrAnthropicAPIKeyRequired
		}

		gateway, err = newAnthropicGateway(profile.APIKey)
	case provider.Voyage:
		return nil, ErrVoyageNoContentGeneration
	case provider.OpenAICompatible:
		if profile.BaseURL == "" {
			return nil, ErrOpenAICompatibleBaseURLRequired
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	default:
		return nil, ErrProviderNotSpecified
	}

	if err != nil {
		return nil, err
	}

	// Determine max concurrency based on rate limits
	// Use RPM (requests per minute) as a guide for concurrency
	// TODO review this logic
	maxConcurrency := profile.RateLimit.RequestsPerMinute
	if maxConcurrency == 0 {
		maxConcurrency = 10 // Default to 10 concurrent requests if unlimited
	}

	gateway = newWorkerPoolGateway(gateway, maxConcurrency)

	limiter := f.getRateLimiter(profile)
	return newRateLimitedGenerationGateway(gateway, limiter), nil
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
func (f *gatewayFactory) NewEmbeddingsGateway(
	ctx context.Context,
	profile *profile.ResolvedProfile,
) (EmbeddingsGateway, error) {
	if profile == nil {
		return nil, ErrProfileCannotBeNil
	}

	switch profile.Provider {
	case provider.Gemini:
		if profile.APIKey == "" {
			return nil, ErrGeminiAPIKeyRequired
		}

		return newGeminiGateway(ctx, profile.APIKey)
	case provider.Llama:
		return nil, ErrLlamaEmbeddingsNotImplemented
	case provider.Anthropic:
		return nil, ErrAnthropicNoEmbeddings
	case provider.OpenAI:
		if profile.APIKey == "" {
			return nil, ErrOpenAIAPIKeyRequired
		}

		return newOpenAIGateway(profile.BaseURL, profile.APIKey), nil
	case provider.OpenRouter:
		if profile.APIKey == "" {
			return nil, ErrOpenRouterAPIKeyRequired
		}

		return newOpenAIGateway(profile.BaseURL, profile.APIKey), nil
	case provider.Voyage:
		if profile.APIKey == "" {
			return nil, ErrVoyageAPIKeyRequired
		}

		return newVoyageGateway(profile.APIKey)
	default:
		return nil, ErrProviderNotSpecified
	}
}
