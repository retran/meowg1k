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

// Package model provides services for resolving and validating LLM model instances.
package model

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/provider"
)

// Model service errors
var (
	ErrNoModelsDefined          = errors.New("no models defined in configuration")
	ErrModelNotFound            = errors.New("model not found in configuration")
	ErrResolvedModelCannotBeNil = errors.New("resolved model cannot be nil")
	ErrMaxOutputTokensTooLarge  = errors.New("max output tokens too large")
	ErrMaxInputTokensTooLarge   = errors.New("max input tokens too large")
	ErrModelNameRequired        = errors.New("model name is required")
	ErrConfigReaderIsNil        = errors.New("config reader is nil")
	ErrProviderRegistryIsNil    = errors.New("provider registry is nil")
	ErrServiceIsNil             = errors.New("model service is nil")
)

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ProviderDefinitionRegistry retrieves provider definitions.
type ProviderDefinitionRegistry interface {
	Get(providerType provider.Provider) (provider.ProviderDefinition, error)
}

// Service resolves and caches model configurations.
type Service struct {
	providerRegistry ProviderDefinitionRegistry
	configReader     ConfigReader
	resolvedModels   map[model.Model]*model.ResolvedModel
	mu               sync.RWMutex
}

// NewService creates a new model resolver service.
func NewService(configReader ConfigReader, providerRegistry ProviderDefinitionRegistry) (*Service, error) {
	if configReader == nil {
		return nil, ErrConfigReaderIsNil
	}

	if providerRegistry == nil {
		return nil, ErrProviderRegistryIsNil
	}

	service := &Service{
		providerRegistry: providerRegistry,
		configReader:     configReader,
		resolvedModels:   make(map[model.Model]*model.ResolvedModel),
	}

	return service, nil
}

// Get retrieves a model using cached data from initialization.
func (s *Service) Get(model model.Model) (*model.ResolvedModel, error) {
	if s == nil {
		return nil, ErrServiceIsNil
	}

	s.mu.RLock()
	if resolved, exists := s.resolvedModels[model]; exists {
		s.mu.RUnlock()
		return resolved, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if resolved, exists := s.resolvedModels[model]; exists {
		return resolved, nil
	}

	cfg, err := s.configReader.GetConfig()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	resolved, err := s.resolveModelInternal(model, cfg)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	s.resolvedModels[model] = resolved

	return resolved, nil
}

// GetInstanceKey returns a unique key for rate limiting based on the model instance characteristics.
func (s *Service) GetInstanceKey(resolved *model.ResolvedModel) (string, error) {
	if resolved == nil {
		return "", ErrResolvedModelCannotBeNil
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
		return nil, ErrNoModelsDefined
	}

	modelDef, exists := cfg.Models[string(modelName)]
	if !exists {
		// TODO proper error
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, modelName)
	}

	providerDef, err := s.providerRegistry.Get(provider.Provider(modelDef.Provider))
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("unknown provider '%s' in modelName '%s': %w", modelDef.Provider, modelName, err)
	}

	resolved := &model.ResolvedModel{
		ID:              string(modelName),
		Provider:        providerDef.Type,
		Model:           modelDef.Model,
		MaxInputTokens:  modelDef.MaxInputTokens,
		MaxOutputTokens: modelDef.MaxOutputTokens,
		BaseURL:         modelDef.BaseURL,
		TokenizerType:   modelDef.TokenizerType,
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
		// TODO proper error
		return nil, fmt.Errorf("modelName validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedModel validates a resolved model configuration.
func (s *Service) validateResolvedModel(resolved *model.ResolvedModel) error {
	if resolved == nil {
		return ErrResolvedModelCannotBeNil
	}

	if resolved.MaxOutputTokens > 200000 {
		// TODO proper error
		return fmt.Errorf("%w: %d (max 200000)", ErrMaxOutputTokensTooLarge, resolved.MaxOutputTokens)
	}

	if resolved.MaxInputTokens > 2000000 {
		// TODO proper error
		return fmt.Errorf("%w: %d (max 2000000)", ErrMaxInputTokensTooLarge, resolved.MaxInputTokens)
	}

	if resolved.Model == "" {
		return ErrModelNameRequired
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
