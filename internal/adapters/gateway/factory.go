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
func (f *Factory) getRateLimiter(resolvedProfile *profile.ResolvedProfile) (ratelimit.Limiter, error) {
	if resolvedProfile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate key based on model instance characteristics
	// IMPORTANT: Use APIKeyEnv (environment variable name), never the actual API key
	// TODO use centralized method to generate keys
	key := string(resolvedProfile.Provider) + ":" + resolvedProfile.BaseURL + ":" + resolvedProfile.Model + ":" + resolvedProfile.APIKeyEnv

	if limiter, exists := f.limiters[key]; exists {
		return limiter, nil
	}

	config := ratelimit.Config{
		ID:                key,
		RequestsPerMinute: resolvedProfile.RateLimit.RequestsPerMinute,
		TokensPerMinute:   resolvedProfile.RateLimit.TokensPerMinute,
		RequestsPerDay:    resolvedProfile.RateLimit.RequestsPerDay,
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
func (f *Factory) shouldEnableCache(resolvedProfile *profile.ResolvedProfile) bool {
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
	return resolvedProfile.CacheEnabled
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
	resolvedProfile *profile.ResolvedProfile,
) (ports.GenerationGateway, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if resolvedProfile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	gateway, err := f.buildGenerationGateway(ctx, resolvedProfile)
	if err != nil {
		return nil, err
	}

	// Determine max concurrency based on rate limits
	// Use RPM (requests per minute) as a guide for concurrency
	// TODO review this logic
	maxConcurrency := resolvedProfile.RateLimit.RequestsPerMinute
	if maxConcurrency == 0 {
		maxConcurrency = 10 // Default to 10 requests if unlimited.
	}

	gateway = newWorkerPoolGateway(gateway, maxConcurrency)

	limiter, err := f.getRateLimiter(resolvedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	gateway = newRateLimitedGenerationGateway(gateway, limiter)

	// Conditionally wrap with caching
	if f.shouldEnableCache(resolvedProfile) {
		updateCache := f.shouldUpdateCache()
		gateway = newCachingGenerationGateway(gateway, f.cacheRepo, updateCache)
	}

	// Wrap with logging (outermost layer to log actual requests/responses)
	commandName, err := f.commandNameReader.GetCommandName()
	if err != nil {
		return nil, fmt.Errorf("failed to get command name: %w", err)
	}
	gateway = newLoggingGenerationGateway(gateway, f.traceLogger, commandName, resolvedProfile.Name, string(resolvedProfile.Provider))

	return gateway, nil
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided profile.
func (f *Factory) NewEmbeddingsGateway(
	ctx context.Context,
	resolvedProfile *profile.ResolvedProfile,
) (ports.EmbeddingsGateway, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if resolvedProfile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	gateway, err := f.buildEmbeddingsGateway(ctx, resolvedProfile)
	if err != nil {
		return nil, err
	}

	limiter, err := f.getRateLimiter(resolvedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limiter: %w", err)
	}

	gateway = newRateLimitedEmbeddingsGateway(gateway, limiter)

	// Conditionally wrap with caching
	if f.shouldEnableCache(resolvedProfile) {
		updateCache := f.shouldUpdateCache()
		gateway = newCachingEmbeddingsGateway(gateway, f.cacheRepo, updateCache)
	}

	// Wrap with logging (outermost layer to log actual requests/responses)
	commandName, err := f.commandNameReader.GetCommandName()
	if err != nil {
		return nil, fmt.Errorf("failed to get command name: %w", err)
	}
	gateway = newLoggingEmbeddingsGateway(gateway, f.traceLogger, commandName, resolvedProfile.Name, string(resolvedProfile.Provider))

	return gateway, nil
}

func (f *Factory) buildGenerationGateway(ctx context.Context, resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	switch resolvedProfile.Provider {
	case provider.Gemini:
		return f.newGeminiGenerationGateway(ctx, resolvedProfile)
	case provider.Llama:
		return f.newLlamaGenerationGateway(resolvedProfile)
	case provider.OpenAI:
		return f.newOpenAIGenerationGateway(resolvedProfile)
	case provider.OpenRouter:
		return f.newOpenRouterGenerationGateway(ctx, resolvedProfile)
	case provider.Anthropic:
		return f.newAnthropicGenerationGateway(resolvedProfile)
	case provider.Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case provider.OpenAICompatible:
		return f.newOpenAICompatibleGenerationGateway(resolvedProfile)
	default:
		return nil, fmt.Errorf("provider must be specified for model %q", resolvedProfile.Model)
	}
}

func (f *Factory) buildEmbeddingsGateway(ctx context.Context, resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	switch resolvedProfile.Provider {
	case provider.Gemini:
		return f.newGeminiEmbeddingsGateway(ctx, resolvedProfile)
	case provider.Llama:
		return f.newLlamaEmbeddingsGateway(resolvedProfile)
	case provider.Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case provider.OpenAI:
		return f.newOpenAIEmbeddingsGateway(resolvedProfile)
	case provider.OpenAICompatible:
		return f.newOpenAICompatibleEmbeddingsGateway(resolvedProfile)
	case provider.OpenRouter:
		return f.newOpenRouterEmbeddingsGateway(resolvedProfile)
	case provider.Voyage:
		return f.newVoyageEmbeddingsGateway(resolvedProfile)
	default:
		return nil, fmt.Errorf("provider must be specified for embeddings model %q", resolvedProfile.Model)
	}
}

func (f *Factory) newGeminiGenerationGateway(ctx context.Context, resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("gemini provider requires an API key for model %q", resolvedProfile.Model)
	}
	gateway, err := newGeminiGateway(ctx, resolvedProfile.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedProfile.Provider, resolvedProfile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newLlamaGenerationGateway(resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if resolvedProfile.BaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL for model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	gateway, err := newLlamaGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedProfile.Provider, resolvedProfile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newOpenAIGenerationGateway(resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires an API key for model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	return newOpenAIGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient), nil
}

func (f *Factory) newOpenAICompatibleGenerationGateway(resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if resolvedProfile.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a base URL for model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	return newOpenAIGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient), nil
}

func (f *Factory) newOpenRouterGenerationGateway(ctx context.Context, resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("openrouter provider requires an API key for model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	gateway, err := NewOpenRouterGateway(ctx, resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedProfile.Provider, resolvedProfile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newAnthropicGenerationGateway(resolvedProfile *profile.ResolvedProfile) (ports.GenerationGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("anthropic provider requires an API key for model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	gateway, err := newAnthropicGateway(resolvedProfile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedProfile.Provider, resolvedProfile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newGeminiEmbeddingsGateway(ctx context.Context, resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("gemini provider requires an API key for embeddings model %q", resolvedProfile.Model)
	}

	gateway, err := newGeminiGateway(ctx, resolvedProfile.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", resolvedProfile.Provider, resolvedProfile.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newLlamaEmbeddingsGateway(resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if resolvedProfile.BaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL for embeddings model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	llamaGW, err := newLlamaGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create llama gateway for embeddings model %q: %w", resolvedProfile.Model, err)
	}

	gateway, ok := llamaGW.(ports.EmbeddingsGateway)
	if !ok {
		return nil, fmt.Errorf("llama gateway does not implement EmbeddingsGateway for model %q", resolvedProfile.Model)
	}
	return gateway, nil
}

func (f *Factory) newOpenAIEmbeddingsGateway(resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires an API key for embeddings model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	return newOpenAIGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient), nil
}

func (f *Factory) newOpenAICompatibleEmbeddingsGateway(resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if resolvedProfile.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a base URL for embeddings model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	return newOpenAIGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient), nil
}

func (f *Factory) newOpenRouterEmbeddingsGateway(resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("openrouter provider requires an API key for embeddings model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	return newOpenAIGateway(resolvedProfile.BaseURL, resolvedProfile.APIKey, httpClient), nil
}

func (f *Factory) newVoyageEmbeddingsGateway(resolvedProfile *profile.ResolvedProfile) (ports.EmbeddingsGateway, error) {
	if resolvedProfile.APIKey == "" {
		return nil, fmt.Errorf("voyage provider requires an API key for embeddings model %q", resolvedProfile.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedProfile.Timeout)
	gateway, err := newVoyageGateway(resolvedProfile.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", resolvedProfile.Provider, resolvedProfile.Model, err)
	}
	return gateway, nil
}
