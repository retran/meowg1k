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
	"sync"

	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/provider"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/ratelimit"
)

type Factory struct {
	mu            sync.Mutex
	limiters      map[string]ratelimit.Limiter // key is profile name
	rateLimitRepo ratelimit.Repository         // rate limit repository
	cacheRepo     ports.CacheRepository        // cache repository for LLM responses
	flagReader    ports.FlagReader             // command-line flag reader
}

// NewFactory creates a new gateway factory with dependencies.
// rateLimitRepo is required for rate limiting functionality.
// cacheRepo and flagReader can be nil if caching is not needed.
func NewFactory(
	rateLimitRepo ratelimit.Repository,
	cacheRepo ports.CacheRepository,
	flagReader ports.FlagReader,
) (*Factory, error) {
	if rateLimitRepo == nil {
		return nil, fmt.Errorf("rate limit repository is nil")
	}
	return &Factory{
		limiters:      make(map[string]ratelimit.Limiter),
		rateLimitRepo: rateLimitRepo,
		cacheRepo:     cacheRepo,
		flagReader:    flagReader,
	}, nil
}

// getRateLimiter returns or creates a rate limiter for the given profile.
// This method is thread-safe. Each unique model instance gets its own rate limiter.
// The key is based on provider:baseURL:model:apiKeyEnv to ensure different API keys
// or endpoints get separate rate limiters.
func (f *Factory) getRateLimiter(profile *profile.ResolvedProfile) (ratelimit.Limiter, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
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
		var err error
		// TODO proper context
		limiter, err = ratelimit.NewLimiter(context.Background(), config, f.rateLimitRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create rate limiter: %w", err)
		}
	}

	f.limiters[key] = limiter

	return limiter, nil
}

// shouldEnableCache determines whether caching should be enabled based on profile config and flags.
func (f *Factory) shouldEnableCache(profile *profile.ResolvedProfile) bool {
	// Cache must be available
	if f.cacheRepo == nil {
		return false
	}

	// Check if --no-cache flag is set
	if f.flagReader != nil {
		if noCache, err := f.flagReader.GetNoCacheFlag(); err == nil && noCache {
			return false
		}
	}

	// Check if caching is enabled for this profile
	return profile.CacheEnabled
}

// shouldUpdateCache determines whether cache should be forcefully updated based on flags.
func (f *Factory) shouldUpdateCache() bool {
	if f.flagReader == nil {
		return false
	}

	updateCache, err := f.flagReader.GetUpdateCacheFlag()
	return err == nil && updateCache
}

// NewGenerationGateway creates a new generation gateway based on the provided profile.
func (f *Factory) NewGenerationGateway(
	ctx context.Context,
	profile *profile.ResolvedProfile,
) (ports.GenerationGateway, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	var gateway ports.GenerationGateway
	var err error

	switch profile.Provider {
	case provider.Gemini:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key for model %q", profile.Model)
		}

		gateway, err = newGeminiGateway(ctx, profile.APIKey)
	case provider.Llama:
		if profile.BaseURL == "" {
			return nil, fmt.Errorf("llama provider requires a base URL for model %q", profile.Model)
		}

		gateway, err = newLlamaGateway(profile.BaseURL, profile.APIKey)
	case provider.OpenAI:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key for model %q", profile.Model)
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case provider.OpenRouter:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key for model %q", profile.Model)
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case provider.Anthropic:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("anthropic provider requires an API key for model %q", profile.Model)
		}

		gateway, err = newAnthropicGateway(profile.APIKey)
	case provider.Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case provider.OpenAICompatible:
		if profile.BaseURL == "" {
			return nil, fmt.Errorf("openai-compatible provider requires a base URL for model %q", profile.Model)
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	default:
		return nil, fmt.Errorf("provider must be specified for model %q", profile.Model)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", profile.Provider, profile.Model, err)
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
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	gateway = newRateLimitedGenerationGateway(gateway, limiter)

	// Conditionally wrap with caching
	if f.shouldEnableCache(profile) {
		updateCache := f.shouldUpdateCache()
		gateway = newCachingGenerationGateway(gateway, f.cacheRepo, updateCache)
	}

	return gateway, nil
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
func (f *Factory) NewEmbeddingsGateway(
	ctx context.Context,
	profile *profile.ResolvedProfile,
) (ports.EmbeddingsGateway, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	var gateway ports.EmbeddingsGateway
	var err error

	switch profile.Provider {
	case provider.Gemini:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("gemini provider requires an API key for embeddings model %q", profile.Model)
		}

		gateway, err = newGeminiGateway(ctx, profile.APIKey)
	case provider.Llama:
		return nil, fmt.Errorf("llama embedding gateway is not yet implemented for model %q", profile.Model)
	case provider.Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case provider.OpenAI:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires an API key for embeddings model %q", profile.Model)
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case provider.OpenRouter:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("openrouter provider requires an API key for embeddings model %q", profile.Model)
		}

		gateway = newOpenAIGateway(profile.BaseURL, profile.APIKey)
	case provider.Voyage:
		if profile.APIKey == "" {
			return nil, fmt.Errorf("voyage provider requires an API key for embeddings model %q", profile.Model)
		}

		gateway, err = newVoyageGateway(profile.APIKey)
	default:
		return nil, fmt.Errorf("provider must be specified for embeddings model %q", profile.Model)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", profile.Provider, profile.Model, err)
	}

	limiter, err := f.getRateLimiter(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	gateway = newRateLimitedEmbeddingsGateway(gateway, limiter)

	// Conditionally wrap with caching
	if f.shouldEnableCache(profile) {
		updateCache := f.shouldUpdateCache()
		gateway = newCachingEmbeddingsGateway(gateway, f.cacheRepo, updateCache)
	}

	return gateway, nil
}
