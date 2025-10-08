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

// Package profile provides services for resolving and validating LLM profiles.
package profile

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/llm"
	"github.com/retran/meowg1k/internal/services/model"
	"github.com/retran/meowg1k/internal/services/provider"
)

// Profile service errors
var (
	ErrNoProfilesDefined          = errors.New("no profiles defined in configuration")
	ErrProfileNotFound            = errors.New("profile not found in configuration")
	ErrResolvedProfileCannotBeNil = errors.New("resolved profile cannot be nil")
	ErrTimeoutTooSmall            = errors.New("timeout must be at least 1 second")
	ErrModelReferenceRequired     = errors.New("profile must reference a model")
)

// Profile defines an enumeration for configured profile names.
type Profile string

// ResolvedProfile represents a profile with all values resolved from both model and profile config.
type ResolvedProfile struct {
	// Profile information
	Name string

	// Model instance information (from model config)
	ModelID         string                // Model instance ID
	Provider        provider.Provider     // Provider type
	Model           string                // Model name
	MaxInputTokens  int                   // Maximum input tokens
	MaxOutputTokens int                   // Maximum output tokens (can be overridden by profile)
	BaseURL         string                // API base URL
	APIKey          string                // Resolved API key (actual value)
	APIKeyEnv       string                // Environment variable name for API key
	TokenizerType   llm.TokenizerType     // Tokenizer type
	RateLimit       model.RateLimitConfig // Rate limiting config

	// Request-specific parameters (from profile config)
	Timeout     time.Duration // Request timeout
	Temperature *float64      // Temperature parameter (optional)
	TopP        *float64      // TopP parameter (optional)
	TopK        *int          // TopK parameter (optional)
}

// Service provides profile configuration resolution capabilities.
type Service interface {
	// Get retrieves a profile with validation using the current config.
	Get(profile Profile) (*ResolvedProfile, error)
}

// serviceImpl is the concrete implementation of the profile resolver service.
type serviceImpl struct {
	modelService     model.Service
	configService    config.Service
	resolvedProfiles map[Profile]*ResolvedProfile
	mu               sync.RWMutex
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new profile resolver service.
func NewService(configService config.Service, modelService model.Service) Service {
	service := &serviceImpl{
		modelService:     modelService,
		configService:    configService,
		resolvedProfiles: make(map[Profile]*ResolvedProfile),
	}

	return service
}

// Get retrieves a profile using cached data from initialization.
func (s *serviceImpl) Get(profile Profile) (*ResolvedProfile, error) {
	s.mu.RLock()
	if resolved, exists := s.resolvedProfiles[profile]; exists {
		s.mu.RUnlock()
		return resolved, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if resolved, exists := s.resolvedProfiles[profile]; exists {
		return resolved, nil
	}

	cfg := s.configService.GetConfig()

	resolved, err := s.resolveProfileInternal(profile, cfg)
	if err != nil {
		return nil, err
	}

	s.resolvedProfiles[profile] = resolved

	return resolved, nil
}

// resolveProfileInternal performs the actual profile resolution logic.
func (s *serviceImpl) resolveProfileInternal(
	profile Profile,
	cfg *config.Config,
) (*ResolvedProfile, error) {
	if cfg.Profiles == nil {
		return nil, ErrNoProfilesDefined
	}

	profileDef, exists := cfg.Profiles[string(profile)]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, profile)
	}

	// Profile must reference a model
	if profileDef.Model == "" {
		return nil, fmt.Errorf("%w: profile '%s'", ErrModelReferenceRequired, profile)
	}

	// Resolve the model instance
	resolvedModel, err := s.modelService.Get(model.Model(profileDef.Model))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model '%s' for profile '%s': %w", profileDef.Model, profile, err)
	}

	// Create resolved profile by combining model and profile settings
	resolved := &ResolvedProfile{
		Name: string(profile),

		// Model instance information
		ModelID:         resolvedModel.ID,
		Provider:        resolvedModel.Provider,
		Model:           resolvedModel.Model,
		MaxInputTokens:  resolvedModel.MaxInputTokens,
		MaxOutputTokens: resolvedModel.MaxOutputTokens,
		BaseURL:         resolvedModel.BaseURL,
		APIKey:          resolvedModel.APIKey,
		APIKeyEnv:       resolvedModel.APIKeyEnv,
		TokenizerType:   resolvedModel.TokenizerType,
		RateLimit:       resolvedModel.RateLimit,

		// Request-specific parameters from profile
		Timeout:     profileDef.Timeout,
		Temperature: profileDef.Temperature,
		TopP:        profileDef.TopP,
		TopK:        profileDef.TopK,
	}

	// Apply default timeout if not specified
	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute
	}

	// Profile can override max output tokens
	if profileDef.MaxTokens != nil && *profileDef.MaxTokens > 0 {
		resolved.MaxOutputTokens = *profileDef.MaxTokens
	}

	if err := s.validateResolvedProfile(resolved); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedProfile validates a resolved profile configuration.
func (s *serviceImpl) validateResolvedProfile(resolved *ResolvedProfile) error {
	if resolved == nil {
		return ErrResolvedProfileCannotBeNil
	}

	if resolved.Timeout < time.Second {
		return fmt.Errorf("%w, got %v", ErrTimeoutTooSmall, resolved.Timeout)
	}

	// Model validation is already done by model.Service
	// Just verify the resolved profile has required fields
	if resolved.Model == "" {
		return fmt.Errorf("resolved profile has empty model name")
	}

	if resolved.ModelID == "" {
		return fmt.Errorf("resolved profile has empty model ID")
	}

	return nil
}
