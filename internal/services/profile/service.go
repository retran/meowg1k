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

	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/profile"
)

// Profile service errors
var (
	ErrNoProfilesDefined          = errors.New("no profiles defined in configuration")
	ErrProfileNotFound            = errors.New("profile not found in configuration")
	ErrResolvedProfileCannotBeNil = errors.New("resolved profile cannot be nil")
	ErrTimeoutTooSmall            = errors.New("timeout must be at least 1 second")
	ErrModelReferenceRequired     = errors.New("profile must reference a model")
	ErrConfigReaderIsNil          = errors.New("config reader is nil")
	ErrProfileResolverIsNil       = errors.New("profile resolver is nil")
	ErrServiceIsNil               = errors.New("service is nil")
)

// ConfigReader reads the application configuration.
type ConfigReader interface {
	GetConfig() (*config.Config, error)
}

// ModelResolver resolves model configurations.
type ModelResolver interface {
	Get(model model.Model) (*model.ResolvedModel, error)
}

// Service resolves and caches profile configurations.
type Service struct {
	modelResolver    ModelResolver
	configReader     ConfigReader
	resolvedProfiles map[profile.Profile]*profile.ResolvedProfile
	mu               sync.RWMutex
}

// NewService creates a new profile resolver service.
func NewService(configReader ConfigReader, modelResolver ModelResolver) (*Service, error) {
	if configReader == nil {
		return nil, ErrConfigReaderIsNil
	}

	if modelResolver == nil {
		return nil, ErrProfileResolverIsNil
	}

	service := &Service{
		modelResolver:    modelResolver,
		configReader:     configReader,
		resolvedProfiles: make(map[profile.Profile]*profile.ResolvedProfile),
	}
	return service, nil
}

// Get retrieves a profile using cached data from initialization.
func (s *Service) Get(profile profile.Profile) (*profile.ResolvedProfile, error) {
	if s == nil {
		return nil, ErrServiceIsNil
	}

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

	cfg, err := s.configReader.GetConfig()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	resolved, err := s.resolveProfileInternal(profile, cfg)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	s.resolvedProfiles[profile] = resolved

	return resolved, nil
}

// resolveProfileInternal performs the actual profile resolution logic.
func (s *Service) resolveProfileInternal(
	profileName profile.Profile,
	cfg *config.Config,
) (*profile.ResolvedProfile, error) {
	if s == nil {
		return nil, ErrServiceIsNil
	}

	if profileName == "" {
		return nil, ErrProfileNotFound
	}

	if cfg == nil {
		return nil, ErrConfigReaderIsNil
	}

	if cfg.Profiles == nil {
		return nil, ErrNoProfilesDefined
	}

	profileDef, exists := cfg.Profiles[string(profileName)]
	if !exists {
		// TODO proper error
		return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, profileName)
	}

	if profileDef.Model == "" {
		// TODO proper error
		return nil, fmt.Errorf("%w: profileName '%s'", ErrModelReferenceRequired, profileName)
	}

	resolvedModel, err := s.modelResolver.Get(model.Model(profileDef.Model))
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to resolve model '%s' for profileName '%s': %w", profileDef.Model, profileName, err)
	}

	resolved := &profile.ResolvedProfile{
		Name:            string(profileName),
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
		Timeout:         profileDef.Timeout,
		Temperature:     profileDef.Temperature,
		TopP:            profileDef.TopP,
		TopK:            profileDef.TopK,
	}

	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute
	}

	if profileDef.MaxTokens != nil && *profileDef.MaxTokens > 0 {
		resolved.MaxOutputTokens = *profileDef.MaxTokens
	}

	if err := s.validateResolvedProfile(resolved); err != nil {
		// TODO proper error
		return nil, fmt.Errorf("profileName validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedProfile validates a resolved profile configuration.
func (s *Service) validateResolvedProfile(resolved *profile.ResolvedProfile) error {
	if s == nil {
		return ErrServiceIsNil
	}

	if resolved == nil {
		return ErrResolvedProfileCannotBeNil
	}

	if resolved.Timeout < time.Second {
		// TODO proper error
		return fmt.Errorf("%w, got %v", ErrTimeoutTooSmall, resolved.Timeout)
	}

	if resolved.Model == "" {
		// TODO proper error
		return fmt.Errorf("resolved profile has empty model name")
	}

	if resolved.ModelID == "" {
		// TODO proper error
		return fmt.Errorf("resolved profile has empty model ID")
	}

	return nil
}
