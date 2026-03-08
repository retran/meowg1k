// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package gateway provides adapters for LLM providers (OpenAI, Anthropic, Gemini, etc.) with caching and logging.
package gateway

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/preset"
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
}

// NewFactory creates a new gateway factory with dependencies.
func NewFactory(
	cacheRepo ports.CacheRepository,
	flagReader ports.FlagReader,
	traceLogger TraceLogger,
	commandNameReader ports.CommandNameReader,
	httpClientService ports.HTTPClientService,
) (*Factory, error) {
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
		cacheRepo:         cacheRepo,
		flagReader:        flagReader,
		traceLogger:       traceLogger,
		commandNameReader: commandNameReader,
		httpClientService: httpClientService,
	}, nil
}

// shouldEnableCache determines whether caching should be enabled based on preset config and flags.
func (f *Factory) shouldEnableCache(resolvedPreset *preset.ResolvedPreset) bool {
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

	// Check if caching is enabled for this preset
	if !resolvedPreset.CacheEnabled {
		return false
	}

	return true
}

// shouldUpdateCache determines whether cache should be forcefully updated based on flags.
func (f *Factory) shouldUpdateCache() bool {
	if f.flagReader == nil {
		return false
	}

	updateCache, err := f.flagReader.GetUpdateCacheFlag()
	return err == nil && updateCache
}

// validateGatewayRequest validates the common preconditions for creating a gateway.
func validateGatewayRequest(ctx context.Context, resolvedPreset *preset.ResolvedPreset) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if resolvedPreset == nil {
		return fmt.Errorf("preset cannot be nil")
	}
	return nil
}

// getCommandName retrieves the current command name for logging.
func (f *Factory) getCommandName() (string, error) {
	commandName, err := f.commandNameReader.GetCommandName()
	if err != nil {
		return "", fmt.Errorf("failed to get command name: %w", err)
	}
	return commandName, nil
}

// wrapGenerationGateway applies caching and logging decorators to a generation gateway.
func (f *Factory) wrapGenerationGateway(gw ports.GenerationGateway, resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if f.shouldEnableCache(resolvedPreset) {
		gw = newCachingGenerationGateway(gw, f.cacheRepo, f.shouldUpdateCache())
	}
	commandName, err := f.getCommandName()
	if err != nil {
		return nil, err
	}
	return newLoggingGenerationGateway(gw, f.traceLogger, commandName, resolvedPreset.Name, string(resolvedPreset.Provider)), nil
}

// wrapEmbeddingsGateway applies caching and logging decorators to an embeddings gateway.
func (f *Factory) wrapEmbeddingsGateway(gw ports.EmbeddingsGateway, resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if f.shouldEnableCache(resolvedPreset) {
		gw = newCachingEmbeddingsGateway(gw, f.cacheRepo, f.shouldUpdateCache())
	}
	commandName, err := f.getCommandName()
	if err != nil {
		return nil, err
	}
	return newLoggingEmbeddingsGateway(gw, f.traceLogger, commandName, resolvedPreset.Name, string(resolvedPreset.Provider)), nil
}

// NewGenerationGateway creates a new generation gateway based on the provided preset.
// The gateway is optionally wrapped with caching and always wrapped with logging.
func (f *Factory) NewGenerationGateway(
	ctx context.Context,
	resolvedPreset *preset.ResolvedPreset,
) (ports.GenerationGateway, error) {
	if err := validateGatewayRequest(ctx, resolvedPreset); err != nil {
		return nil, err
	}
	gw, err := f.buildGenerationGateway(ctx, resolvedPreset)
	if err != nil {
		return nil, err
	}
	return f.wrapGenerationGateway(gw, resolvedPreset)
}

// NewEmbeddingsGateway creates a new embeddings gateway based on the provided preset.
// The gateway is optionally wrapped with caching and always wrapped with logging.
func (f *Factory) NewEmbeddingsGateway(
	ctx context.Context,
	resolvedPreset *preset.ResolvedPreset,
) (ports.EmbeddingsGateway, error) {
	if err := validateGatewayRequest(ctx, resolvedPreset); err != nil {
		return nil, err
	}
	gw, err := f.buildEmbeddingsGateway(ctx, resolvedPreset)
	if err != nil {
		return nil, err
	}
	return f.wrapEmbeddingsGateway(gw, resolvedPreset)
}

func (f *Factory) buildGenerationGateway(ctx context.Context, resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	switch resolvedPreset.Provider {
	case provider.Gemini:
		return f.newGeminiGenerationGateway(ctx, resolvedPreset)
	case provider.Llama:
		return f.newLlamaGenerationGateway(resolvedPreset)
	case provider.OpenAI:
		return f.newOpenAIGenerationGateway(resolvedPreset)
	case provider.OpenRouter:
		return f.newOpenRouterGenerationGateway(ctx, resolvedPreset)
	case provider.Anthropic:
		return f.newAnthropicGenerationGateway(resolvedPreset)
	case provider.Voyage:
		return nil, fmt.Errorf("voyage provider only supports embeddings, not content generation")
	case provider.OpenAICompatible:
		return f.newOpenAICompatibleGenerationGateway(resolvedPreset)
	case provider.GitHubCopilot:
		return f.newCopilotGenerationGateway(ctx, resolvedPreset)
	default:
		return nil, fmt.Errorf("provider must be specified for model %q", resolvedPreset.Model)
	}
}

func (f *Factory) buildEmbeddingsGateway(ctx context.Context, resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	switch resolvedPreset.Provider {
	case provider.Gemini:
		return f.newGeminiEmbeddingsGateway(ctx, resolvedPreset)
	case provider.Llama:
		return f.newLlamaEmbeddingsGateway(resolvedPreset)
	case provider.Anthropic:
		return nil, fmt.Errorf("anthropic provider does not provide embedding models (use voyage provider for embeddings recommended by Anthropic)")
	case provider.OpenAI:
		return f.newOpenAIEmbeddingsGateway(resolvedPreset)
	case provider.OpenAICompatible:
		return f.newOpenAICompatibleEmbeddingsGateway(resolvedPreset)
	case provider.OpenRouter:
		return f.newOpenRouterEmbeddingsGateway(resolvedPreset)
	case provider.Voyage:
		return f.newVoyageEmbeddingsGateway(resolvedPreset)
	case provider.GitHubCopilot:
		return nil, fmt.Errorf("github-copilot provider does not support embeddings")
	default:
		return nil, fmt.Errorf("provider must be specified for embeddings model %q", resolvedPreset.Model)
	}
}

func (f *Factory) newGeminiGenerationGateway(ctx context.Context, resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("gemini provider requires an API key for model %q", resolvedPreset.Model)
	}
	gateway, err := newGeminiGateway(ctx, resolvedPreset.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newLlamaGenerationGateway(resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if resolvedPreset.BaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL for model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	gateway, err := newLlamaGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newOpenAIGenerationGateway(resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires an API key for model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	return newOpenAIGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient), nil
}

func (f *Factory) newOpenAICompatibleGenerationGateway(resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if resolvedPreset.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a base URL for model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	return newOpenAIGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient), nil
}

func (f *Factory) newOpenRouterGenerationGateway(ctx context.Context, resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("openrouter provider requires an API key for model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	gateway, err := NewOpenRouterGateway(ctx, resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newAnthropicGenerationGateway(resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("anthropic provider requires an API key for model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	gateway, err := newAnthropicGateway(resolvedPreset.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newGeminiEmbeddingsGateway(ctx context.Context, resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("gemini provider requires an API key for embeddings model %q", resolvedPreset.Model)
	}

	gateway, err := newGeminiGateway(ctx, resolvedPreset.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newLlamaEmbeddingsGateway(resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if resolvedPreset.BaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL for embeddings model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	llamaGW, err := newLlamaGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create llama gateway for embeddings model %q: %w", resolvedPreset.Model, err)
	}

	gateway, ok := llamaGW.(ports.EmbeddingsGateway)
	if !ok {
		return nil, fmt.Errorf("llama gateway does not implement EmbeddingsGateway for model %q", resolvedPreset.Model)
	}
	return gateway, nil
}

func (f *Factory) newOpenAIEmbeddingsGateway(resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("openai provider requires an API key for embeddings model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	return newOpenAIGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient), nil
}

func (f *Factory) newOpenAICompatibleEmbeddingsGateway(resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if resolvedPreset.BaseURL == "" {
		return nil, fmt.Errorf("openai-compatible provider requires a base URL for embeddings model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	return newOpenAIGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient), nil
}

func (f *Factory) newOpenRouterEmbeddingsGateway(resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("openrouter provider requires an API key for embeddings model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	return newOpenAIGateway(resolvedPreset.BaseURL, resolvedPreset.APIKey, httpClient), nil
}

func (f *Factory) newVoyageEmbeddingsGateway(resolvedPreset *preset.ResolvedPreset) (ports.EmbeddingsGateway, error) {
	if resolvedPreset.APIKey == "" {
		return nil, fmt.Errorf("voyage provider requires an API key for embeddings model %q", resolvedPreset.Model)
	}

	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	gateway, err := newVoyageGateway(resolvedPreset.APIKey, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s embeddings gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}

func (f *Factory) newCopilotGenerationGateway(ctx context.Context, resolvedPreset *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	httpClient := f.httpClientService.GetWithTimeout(resolvedPreset.Timeout)
	gateway, err := newCopilotGateway(ctx, &copilotGatewayOptions{
		BaseURL:             resolvedPreset.BaseURL,
		AppID:               resolvedPreset.AppID,
		EditorVersion:       resolvedPreset.EditorVersion,
		EditorPluginVersion: resolvedPreset.EditorPluginVersion,
		UserAgent:           resolvedPreset.UserAgent,
		IntegrationID:       resolvedPreset.CopilotIntegrationID,
		OpenAIOrganization:  resolvedPreset.OpenAIOrganization,
		HTTPClient:          httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create %s gateway for model %q: %w", resolvedPreset.Provider, resolvedPreset.Model, err)
	}
	return gateway, nil
}
