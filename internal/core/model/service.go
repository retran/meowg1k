// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package model provides services for managing LLM model configurations and their capabilities.
package model

import (
	"fmt"
	"os"
	"sync"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/provider"
	"github.com/retran/meowg1k/internal/ports"
)

// DefinitionResolver retrieves provider definitions.
type DefinitionResolver interface {
	Get(providerType provider.Provider) (provider.Definition, error)
}

// Service resolves and caches model configurations.
type Service struct {
	providerDefinitionResolver DefinitionResolver
	configResolver             ports.ConfigResolver
	resolvedModels             map[model.Model]*model.ResolvedModel
	mu                         sync.RWMutex
}

// NewService creates a new model resolver service.
func NewService(configResolver ports.ConfigResolver, providerDefinitionResolver DefinitionResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if providerDefinitionResolver == nil {
		return nil, fmt.Errorf("provider registry is nil")
	}

	service := &Service{
		providerDefinitionResolver: providerDefinitionResolver,
		configResolver:             configResolver,
		resolvedModels:             make(map[model.Model]*model.ResolvedModel),
	}

	return service, nil
}

// Get retrieves a model using cached data from initialization.
func (s *Service) Get(requestedModel model.Model) (*model.ResolvedModel, error) {
	if s == nil {
		return nil, fmt.Errorf("model service is nil")
	}

	s.mu.RLock()
	if resolved, exists := s.resolvedModels[requestedModel]; exists {
		s.mu.RUnlock()
		return resolved, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if resolved, exists := s.resolvedModels[requestedModel]; exists {
		return resolved, nil
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	resolved, err := s.resolveModelInternal(requestedModel, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model %q: %w", requestedModel, err)
	}

	s.resolvedModels[requestedModel] = resolved

	return resolved, nil
}

// GetInstanceKey returns a unique key for rate limiting based on the model instance characteristics.
func (s *Service) GetInstanceKey(resolved *model.ResolvedModel) (string, error) {
	if resolved == nil {
		return "", fmt.Errorf("resolved model cannot be nil")
	}

	// Generate key based on: provider:baseURL:model:apiKeyEnv
	// This ensures different API keys or endpoints get separate rate limiters
	// IMPORTANT: Use environment variable name, never the actual API key value
	return fmt.Sprintf("%s:%s:%s:%s",
		resolved.Provider,
		resolved.BaseURL,
		resolved.Model,
		resolved.APIKeyEnv,
	), nil
}

// resolveModelInternal performs the actual model resolution logic.
func (s *Service) resolveModelInternal(
	modelName model.Model,
	cfg *config.Config,
) (*model.ResolvedModel, error) {
	if cfg.Models == nil {
		return nil, fmt.Errorf("no models defined in configuration")
	}

	modelDef, exists := cfg.Models[string(modelName)]
	if !exists {
		return nil, fmt.Errorf("model not found in configuration: %s", modelName)
	}

	providerDef, err := s.providerDefinitionResolver.Get(provider.Provider(modelDef.Provider))
	if err != nil {
		return nil, fmt.Errorf("unknown provider '%s' in modelName '%s': %w", modelDef.Provider, modelName, err)
	}

	resolved := &model.ResolvedModel{
		ID:              string(modelName),
		Provider:        providerDef.Type,
		Model:           modelDef.Model,
		MaxInputTokens:  modelDef.MaxInputTokens,
		MaxOutputTokens: modelDef.MaxOutputTokens,
		BaseURL:         modelDef.BaseURL,
		Tokenizer:       model.Tokenizer(modelDef.Tokenizer),
	}

	// Apply defaults from provider
	resolved.Model = defaultValue(resolved.Model, providerDef.DefaultModel)
	resolved.MaxInputTokens = defaultValue(resolved.MaxInputTokens, providerDef.MaxInputTokens)
	resolved.MaxOutputTokens = defaultValue(resolved.MaxOutputTokens, providerDef.MaxOutputTokens)
	resolved.BaseURL = defaultValue(resolved.BaseURL, providerDef.DefaultBaseURL)

	// Resolve API key from environment
	apiKeyEnv := modelDef.APIKeyEnv
	if apiKeyEnv == "" && providerDef.DefaultEnvVar != "" {
		apiKeyEnv = providerDef.DefaultEnvVar
	}

	resolved.APIKeyEnv = apiKeyEnv
	if apiKeyEnv != "" {
		resolved.APIKey = os.Getenv(apiKeyEnv)
	}

	// Set rate limits
	if modelDef.RateLimit != nil {
		resolved.RateLimit = model.RateLimitConfig{
			RequestsPerMinute: modelDef.RateLimit.RequestsPerMinute,
			TokensPerMinute:   modelDef.RateLimit.TokensPerMinute,
			RequestsPerDay:    modelDef.RateLimit.RequestsPerDay,
		}
	}

	if err := s.validateResolvedModel(resolved); err != nil {
		return nil, fmt.Errorf("modelName validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedModel validates a resolved model configuration.
func (s *Service) validateResolvedModel(resolved *model.ResolvedModel) error {
	if resolved == nil {
		return fmt.Errorf("resolved model cannot be nil")
	}

	if resolved.MaxOutputTokens > 200000 {
		return fmt.Errorf("max output tokens too large: %d (max 200000)", resolved.MaxOutputTokens)
	}

	if resolved.MaxInputTokens > 2000000 {
		return fmt.Errorf("max input tokens too large: %d (max 2000000)", resolved.MaxInputTokens)
	}

	if resolved.Model == "" {
		return fmt.Errorf("model name is required")
	}

	return nil
}

func defaultValue[T comparable](value, fallback T) T {
	var zero T
	if value != zero {
		return value
	}

	return fallback
}
