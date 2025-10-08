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

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/llm"
	"github.com/retran/meowg1k/internal/services/provider"
)

// Model service errors
var (
	ErrNoModelsDefined          = errors.New("no models defined in configuration")
	ErrModelNotFound            = errors.New("model not found in configuration")
	ErrResolvedModelCannotBeNil = errors.New("resolved model cannot be nil")
	ErrMaxOutputTokensTooLarge  = errors.New("max output tokens too large")
	ErrMaxInputTokensTooLarge   = errors.New("max input tokens too large")
	ErrModelNameRequired        = errors.New("model name is required")
)

// Model defines an enumeration for configured model instance names.
type Model string

// RateLimitConfig contains rate limiting configuration for a model instance.
type RateLimitConfig struct {
	RequestsPerMinute int
	TokensPerMinute   int
	RequestsPerDay    int
}

// ResolvedModel represents a model instance with all values resolved.
type ResolvedModel struct {
	ID              string            // Model instance ID from config
	Provider        provider.Provider // Resolved provider
	Model           string            // Model name
	MaxInputTokens  int               // Maximum input tokens
	MaxOutputTokens int               // Maximum output tokens
	BaseURL         string            // API base URL
	APIKey          string            // Resolved API key (actual value)
	APIKeyEnv       string            // Environment variable name for API key
	TokenizerType   llm.TokenizerType // Tokenizer type
	RateLimit       RateLimitConfig   // Rate limiting config
}

// ApplicationConfigReader reads the application configuration.
type ApplicationConfigReader interface {
	GetConfig() *config.Config
}

// ProviderDefinitionRegistry retrieves provider definitions.
type ProviderDefinitionRegistry interface {
	Get(providerType provider.Provider) (provider.ProviderDefinition, error)
}

// Service resolves and caches model configurations.
type Service struct {
	providerRegistry ProviderDefinitionRegistry
	configReader     ApplicationConfigReader
	resolvedModels   map[Model]*ResolvedModel
	mu               sync.RWMutex
}

// NewService creates a new model resolver service.
func NewService(configReader ApplicationConfigReader, providerRegistry ProviderDefinitionRegistry) *Service {
	service := &Service{
		providerRegistry: providerRegistry,
		configReader:     configReader,
		resolvedModels:   make(map[Model]*ResolvedModel),
	}

	return service
}

// Get retrieves a model using cached data from initialization.
func (s *Service) Get(model Model) (*ResolvedModel, error) {
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

	cfg := s.configReader.GetConfig()

	resolved, err := s.resolveModelInternal(model, cfg)
	if err != nil {
		return nil, err
	}

	s.resolvedModels[model] = resolved

	return resolved, nil
}

// GetInstanceKey returns a unique key for rate limiting based on the model instance characteristics.
func (s *Service) GetInstanceKey(resolved *ResolvedModel) string {
	// Generate key based on: provider:baseURL:model:apiKeyEnv
	// This ensures different API keys or endpoints get separate rate limiters
	// IMPORTANT: Use environment variable name, never the actual API key value
	return fmt.Sprintf("%s:%s:%s:%s",
		resolved.Provider,
		resolved.BaseURL,
		resolved.Model,
		resolved.APIKeyEnv, // Environment variable name, not the actual key
	)
}

// resolveModelInternal performs the actual model resolution logic.
func (s *Service) resolveModelInternal(
	model Model,
	cfg *config.Config,
) (*ResolvedModel, error) {
	if cfg.Models == nil {
		return nil, ErrNoModelsDefined
	}

	modelDef, exists := cfg.Models[string(model)]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, model)
	}

	providerDef, err := s.providerRegistry.Get(provider.Provider(modelDef.Provider))
	if err != nil {
		return nil, fmt.Errorf("unknown provider '%s' in model '%s': %w", modelDef.Provider, model, err)
	}

	resolved := &ResolvedModel{
		ID:              string(model),
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
	resolved.TokenizerType = defaultValue(resolved.TokenizerType, providerDef.TokenizerType)
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
		resolved.RateLimit = RateLimitConfig{
			RequestsPerMinute: modelDef.RateLimit.RequestsPerMinute,
			TokensPerMinute:   modelDef.RateLimit.TokensPerMinute,
			RequestsPerDay:    modelDef.RateLimit.RequestsPerDay,
		}
	}

	if err := s.validateResolvedModel(resolved); err != nil {
		return nil, fmt.Errorf("model validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedModel validates a resolved model configuration.
func (s *Service) validateResolvedModel(resolved *ResolvedModel) error {
	if resolved == nil {
		return ErrResolvedModelCannotBeNil
	}

	if resolved.MaxOutputTokens > 200000 {
		return fmt.Errorf("%w: %d (max 200000)", ErrMaxOutputTokensTooLarge, resolved.MaxOutputTokens)
	}

	if resolved.MaxInputTokens > 2000000 {
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
