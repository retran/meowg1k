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
	"os"
	"time"

	mdConfig "github.com/retran/meowg1k/internal/models/config"
	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdProfile "github.com/retran/meowg1k/internal/models/profile"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/provider"
)

// Profile service errors
var (
	ErrNoProfilesDefined          = errors.New("no profiles defined in configuration")
	ErrProfileNotFound            = errors.New("profile not found in configuration")
	ErrResolvedProfileCannotBeNil = errors.New("resolved profile cannot be nil")
	ErrTimeoutTooSmall            = errors.New("timeout must be at least 1 second")
	ErrMaxOutputTokensTooLarge    = errors.New("max output tokens too large")
	ErrMaxInputTokensTooLarge     = errors.New("max input tokens too large")
	ErrModelNameRequired          = errors.New("model name is required")
)

// Service provides profile configuration resolution capabilities.
type Service interface {
	// Get retrieves a profile with validation using the current config.
	Get(profile mdProfile.Profile) (*mdProfile.ResolvedProfile, error)
}

// serviceImpl is the concrete implementation of the profile resolver service.
type serviceImpl struct {
	providerService  provider.Service
	configService    config.Service
	resolvedProfiles map[mdProfile.Profile]*mdProfile.ResolvedProfile
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new profile resolver service.
func NewService(configService config.Service, providerService provider.Service) Service {
	service := &serviceImpl{
		providerService:  providerService,
		configService:    configService,
		resolvedProfiles: make(map[mdProfile.Profile]*mdProfile.ResolvedProfile),
	}

	return service
}

// Get retrieves a profile using cached data from initialization.
func (s *serviceImpl) Get(profile mdProfile.Profile) (*mdProfile.ResolvedProfile, error) {
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
	profile mdProfile.Profile,
	cfg *mdConfig.Config,
) (*mdProfile.ResolvedProfile, error) {
	if cfg.Profiles == nil {
		return nil, ErrNoProfilesDefined
	}

	profileDef, exists := cfg.Profiles[string(profile)]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, profile)
	}

	providerDef, err := s.providerService.Get(mdGateway.Provider(profileDef.Provider))
	if err != nil {
		return nil, fmt.Errorf("unknown provider '%s' in profile '%s': %w", profileDef.Provider, profile, err)
	}

	resolved := &mdProfile.ResolvedProfile{
		Provider:        providerDef.Type,
		Model:           profileDef.Model,
		MaxInputTokens:  profileDef.MaxInputTokens,
		MaxOutputTokens: profileDef.MaxOutputTokens,
		Timeout:         profileDef.Timeout,
		BaseURL:         profileDef.BaseURL,
		TokenizerType:   profileDef.TokenizerType,
	}

	if resolved.Model == "" {
		resolved.Model = providerDef.DefaultModel
	}

	if resolved.MaxInputTokens == 0 {
		resolved.MaxInputTokens = providerDef.MaxInputTokens
	}

	if resolved.MaxOutputTokens == 0 {
		resolved.MaxOutputTokens = providerDef.MaxOutputTokens
	}

	if resolved.Timeout == 0 {
		resolved.Timeout = providerDef.DefaultTimeout
	}

	if resolved.TokenizerType == "" {
		resolved.TokenizerType = providerDef.TokenizerType
	}

	if resolved.BaseURL == "" {
		resolved.BaseURL = providerDef.DefaultBaseURL
	}

	apiKeyEnv := profileDef.APIKeyEnv
	if apiKeyEnv == "" && providerDef.DefaultEnvVar != "" {
		apiKeyEnv = providerDef.DefaultEnvVar
	}

	if apiKeyEnv != "" {
		resolved.APIKey = os.Getenv(apiKeyEnv)
	}

	if err := s.validateResolvedProfile(resolved); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedProfile validates a resolved profile configuration.
func (s *serviceImpl) validateResolvedProfile(resolved *mdProfile.ResolvedProfile) error {
	if resolved == nil {
		return ErrResolvedProfileCannotBeNil
	}

	if resolved.Timeout < time.Second {
		return fmt.Errorf("%w, got %v", ErrTimeoutTooSmall, resolved.Timeout)
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
