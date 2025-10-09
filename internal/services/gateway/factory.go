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
	"fmt"
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
	ErrGatewayIsNil         = errors.New("gateway is nil")
	ErrRequestIsNil         = errors.New("request is nil")
	ErrContextIsNil         = errors.New("context is nil")
)

type Factory struct {
	mu       sync.Mutex
	limiters map[string]ratelimit.Limiter // key is profile name
	repoFunc func() ratelimit.Repository  // lazy function to get database repository
}

// NewFactory creates a new gateway factory with a lazy repository function.
// The repoFunc will only be called when a rate limiter with actual limits is needed.
func NewFactory(repoFunc func() ratelimit.Repository) *Factory {
	// TODO proper checks and error
	return &Factory{
		limiters: make(map[string]ratelimit.Limiter),
		repoFunc: repoFunc,
	}
}

// getRateLimiter returns or creates a rate limiter for the given profile.
// This method is thread-safe. Each unique model instance gets its own rate limiter.
// The key is based on provider:baseURL:model:apiKeyEnv to ensure different API keys
// or endpoints get separate rate limiters.
func (f *Factory) getRateLimiter(profile *profile.ResolvedProfile) (ratelimit.Limiter, error) {
	if profile == nil {
		return nil, ErrProfileCannotBeNil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate key based on model instance characteristics
	// IMPORTANT: Use APIKeyEnv (environment variable name), never the actual API key
	// TODO use centralized method to generate keys
	key := string(profile.Provider) + ":" + profile.BaseURL + ":" + profile.Model + ":" + profile.APIKeyEnv

	if limiter, exists := f.limiters[key]; exists {
		return limiter, nil
	}

	config := ratelimit.Config{
		ID:                key,
		RequestsPerMinute: profile.RateLimit.RequestsPerMinute,
		TokensPerMinute:   profile.RateLimit.TokensPerMinute,
		RequestsPerDay:    profile.RateLimit.RequestsPerDay,
	}

	var limiter ratelimit.Limiter

	if config.RequestsPerMinute == 0 && config.TokensPerMinute == 0 && config.RequestsPerDay == 0 {
		limiter = ratelimit.NewNoOpLimiter()
	} else {
		repo := f.repoFunc()
		if repo == nil {
			// TODO proper error
			return nil, fmt.Errorf("rate limiting is configured but database repository is not available")
		}
		var err error
		// TODO proper context
		limiter, err = ratelimit.NewLimiter(context.Background(), config, repo)
		if err != nil {
			// TODO proper error
			return nil, fmt.Errorf("failed to create rate limiter: %w", err)
		}
	}

	f.limiters[key] = limiter

	return limiter, nil
}

// NewGenerationGateway creates a new generation gateway based on the provided profile.
func (f *Factory) NewGenerationGateway(
	ctx context.Context,
	profile *profile.ResolvedProfile,
) (GenerationGateway, error) {
	if ctx == nil {
		return nil, ErrContextIsNil
	}

	if profile == nil {
		return nil, ErrProfileCannotBeNil
	}

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

	limiter, err := f.getRateLimiter(profile)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	return newRateLimitedGenerationGateway(gateway, limiter), nil
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
func (f *Factory) NewEmbeddingsGateway(
	ctx context.Context,
	profile *profile.ResolvedProfile,
) (EmbeddingsGateway, error) {
	if ctx == nil {
		return nil, ErrContextIsNil
	}

	if profile == nil {
		return nil, ErrProfileCannotBeNil
	}

	var gateway EmbeddingsGateway
	var err error

	switch profile.Provider {
	case provider.Gemini:
		if profile.APIKey == "" {
			return nil, ErrGeminiAPIKeyRequired
		}

		gateway, err = newGeminiGateway(ctx, profile.APIKey)
	case provider.Llama:
		return nil, ErrLlamaEmbeddingsNotImplemented
	case provider.Anthropic:
		return nil, ErrAnthropicNoEmbeddings
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
	case provider.Voyage:
		if profile.APIKey == "" {
			return nil, ErrVoyageAPIKeyRequired
		}

		gateway, err = newVoyageGateway(profile.APIKey)
	default:
		return nil, ErrProviderNotSpecified
	}

	if err != nil {
		// TODO proper error
		return nil, err
	}

	limiter, err := f.getRateLimiter(profile)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	return newRateLimitedEmbeddingsGateway(gateway, limiter), nil
}
