// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package gateway provides adapters for LLM providers (OpenAI, Anthropic, Gemini, etc.) with caching, rate limiting, and logging.
package gateway

import (
	"context"
	"fmt"
	"sync"

	ratelimit2 "github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
	"github.com/retran/meowg1k/internal/core/ratelimit"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/provider"
	"github.com/retran/meowg1k/internal/ports"
)

// Factory constructs provider-specific gateways with shared dependencies.
type Factory struct {
	cacheRepo         ports.CacheRepository
	flagReader        ports.FlagReader
	traceLogger       TraceLogger
	commandNameReader ports.CommandNameReader
	httpClientService ports.HTTPClientService
	limiters          map[string]ratelimit.Limiter
	rateLimitRepo     *ratelimit2.Repository
	mu                sync.Mutex
}

// NewFactory creates a new gateway factory with dependencies.
func NewFactory(
	rateLimitRepo *ratelimit2.Repository,
	cacheRepo ports.CacheRepository,
	flagReader ports.FlagReader,
	traceLogger TraceLogger,
	commandNameReader ports.CommandNameReader,
	httpClientService ports.HTTPClientService,
) (*Factory, error) {
	if rateLimitRepo == nil {
		return nil, fmt.Errorf("rate limit repository is nil")
	}
	if cacheRepo == nil {
		return nil, fmt.Errorf("cache repository is nil")
	}
	if flagReader == nil {
		return nil, fmt.Errorf("flag reader is nil")
	}
	if traceLogger == nil {
		return nil, fmt.Errorf("trace logger is nil")
	}
	if commandNameReader == nil {
		return nil, fmt.Errorf("command name reader is nil")
	}
	if httpClientService == nil {
		return nil, fmt.Errorf("http client service is nil")
	}

	return &Factory{
		limiters:          make(map[string]ratelimit.Limiter),
		rateLimitRepo:     rateLimitRepo,
		cacheRepo:         cacheRepo,
		flagReader:        flagReader,
		traceLogger:       traceLogger,
		commandNameReader: commandNameReader,
		httpClientService: httpClientService,
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

	gateway, err := f.buildGenerationGateway(ctx, profile)
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
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	gateway = newRateLimitedGenerationGateway(gateway, limiter)

	// Conditionally wrap with caching
	if f.shouldEnableCache(profile) {
		updateCache := f.shouldUpdateCache()
		gateway = newCachingGenerationGateway(gateway, f.cacheRepo, updateCache)
	}

	// Wrap with logging (outermost layer to log actual requests/responses)
	commandName, err := f.commandNameReader.GetCommandName()
	if err != nil {
		return nil, fmt.Errorf("failed to get command name: %w", err)
	}
	gateway = newLoggingGenerationGateway(gateway, f.traceLogger, commandName, profile.Name, string(profile.Provider))

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

	gateway, err := f.buildEmbeddingsGateway(ctx, profile)
	if err != nil {
		return nil, err
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

	// Wrap with logging (outermost layer to log actual requests/responses)
	commandName, err := f.commandNameReader.GetCommandName()
	if err != nil {
		return nil, fmt.Errorf("failed to get command name: %w", err)
	}
	gateway = newLoggingEmbeddingsGateway(gateway, f.traceLogger, commandName, profile.Name, string(profile.Provider))

	return gateway, nil
}

func (f *Factory) buildGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	switch profile.Provider {
	case provider.Gemini:
		return f.newGeminiGenerationGateway(ctx, profile)
	case provider.Llama:
		return f.newLlamaGenerationGateway(profile)
	case provider.OpenAI:
		return f.newOpenAIGenerationGateway(profile)
	case provider.OpenRouter:
		return f.newOpenRouterGenerationGateway(ctx, profile)
	case provider.Anthropic:
		return f.newAnthropicGenerationGateway(profile)
	case provider.Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case provider.OpenAICompatible:
		return f.newOpenAICompatibleGenerationGateway(profile)
	default:
		return nil, fmt.Errorf("provider must be specified for model %q", profile.Model)
	}
}

func (f *Factory) buildEmbeddingsGateway(ctx context.Context, profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	switch profile.Provider {
	case provider.Gemini:
		return f.newGeminiEmbeddingsGateway(ctx, profile)
	case provider.Llama:
		return f.newLlamaEmbeddingsGateway(profile)
	case provider.Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case provider.OpenAI:
		return f.newOpenAIEmbeddingsGateway(profile)
	case provider.OpenAICompatible:
		return f.newOpenAICompatibleEmbeddingsGateway(profile)
	case provider.OpenRouter:
		return f.newOpenRouterEmbeddingsGateway(profile)
	case provider.Voyage:
		return f.newVoyageEmbeddingsGateway(profile)
	default:
		return nil, fmt.Errorf("provider must be specified for embeddings model %q", profile.Model)
	}
}

func (f *Factory) newGeminiGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("gemini provider requires an API key for model %q", profile.Model)
	}
	gateway, err := newGeminiGateway(ctx, profile.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", profile.Provider, profile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newLlamaGenerationGateway(profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if profile.BaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL for model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	gateway, err := newLlamaGateway(profile.BaseURL, profile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", profile.Provider, profile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newOpenAIGenerationGateway(profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires an API key for model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	return newOpenAIGateway(profile.BaseURL, profile.APIKey, httpClient), nil
}

func (f *Factory) newOpenAICompatibleGenerationGateway(profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if profile.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a base URL for model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	return newOpenAIGateway(profile.BaseURL, profile.APIKey, httpClient), nil
}

func (f *Factory) newOpenRouterGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("openrouter provider requires an API key for model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	gateway, err := NewOpenRouterGateway(ctx, profile.BaseURL, profile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", profile.Provider, profile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newAnthropicGenerationGateway(profile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("anthropic provider requires an API key for model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	gateway, err := newAnthropicGateway(profile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", profile.Provider, profile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newGeminiEmbeddingsGateway(ctx context.Context, profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("gemini provider requires an API key for embeddings model %q", profile.Model)
	}

	gateway, err := newGeminiGateway(ctx, profile.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", profile.Provider, profile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newLlamaEmbeddingsGateway(profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if profile.BaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL for embeddings model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	llamaGW, err := newLlamaGateway(profile.BaseURL, profile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create llama gateway for embeddings model %q: %w", profile.Model, err)
	}

	gateway, ok := llamaGW.(ports.EmbeddingsGateway)
	if !ok {
		return nil, fmt.Errorf("llama gateway does not implement EmbeddingsGateway for model %q", profile.Model)
	}
	return gateway, nil
}

func (f *Factory) newOpenAIEmbeddingsGateway(profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires an API key for embeddings model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	return newOpenAIGateway(profile.BaseURL, profile.APIKey, httpClient), nil
}

func (f *Factory) newOpenAICompatibleEmbeddingsGateway(profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if profile.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a base URL for embeddings model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	return newOpenAIGateway(profile.BaseURL, profile.APIKey, httpClient), nil
}

func (f *Factory) newOpenRouterEmbeddingsGateway(profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("openrouter provider requires an API key for embeddings model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	return newOpenAIGateway(profile.BaseURL, profile.APIKey, httpClient), nil
}

func (f *Factory) newVoyageEmbeddingsGateway(profile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if profile.APIKey == "" {
		return nil, fmt.Errorf("voyage provider requires an API key for embeddings model %q", profile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(profile.Timeout)
	gateway, err := newVoyageGateway(profile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", profile.Provider, profile.Model, err)
	}
	return gateway, nil
}
