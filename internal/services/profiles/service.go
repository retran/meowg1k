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

package profiles

import (
	"fmt"
	"os"
	"time"

	"github.com/retran/meowg1k/internal/models/config"
	configservice "github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/providers"
)

// Service provides profile configuration resolution capabilities.
type Service interface {
	// ResolveProfile resolves a profile with validation using the current config.
	ResolveProfile(profileName string) (*config.ResolvedProfile, error)
}

// serviceImpl is the concrete implementation of the profile resolver service.
type serviceImpl struct {
	registryService providers.Service
	configService   configservice.Service
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new profile resolver service.
func NewService(registryService providers.Service, configService configservice.Service) Service {
	return &serviceImpl{
		registryService: registryService,
		configService:   configService,
	}
}

// ResolveProfile resolves a profile with validation using the current config.
func (s *serviceImpl) ResolveProfile(profileName string) (*config.ResolvedProfile, error) {
	cfg := s.configService.GetConfig()
	// No need to check for nil since manager service guarantees a loaded config

	if cfg.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined in configuration")
	}

	profile, exists := cfg.Profiles[profileName]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found in configuration", profileName)
	}

	// Get provider definition
	providerDef, err := s.registryService.GetProvider(profile.Provider)
	if err != nil {
		return nil, fmt.Errorf("unknown provider '%s' in profile '%s': %w", profile.Provider, profileName, err)
	}

	// Apply defaults for missing values
	resolved := &config.ResolvedProfile{
		Provider:        providerDef.Type,
		Model:           profile.Model,
		MaxInputTokens:  profile.MaxInputTokens,
		MaxOutputTokens: profile.MaxOutputTokens,
		Timeout:         profile.Timeout,
		BaseURL:         profile.BaseURL,
		TokenizerType:   profile.TokenizerType,
	}

	// Apply provider defaults if values are not set
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

	// Resolve API key from environment
	apiKeyEnv := profile.APIKeyEnv
	if apiKeyEnv == "" && providerDef.DefaultEnvVar != "" {
		// Use default environment variable name from provider definition
		apiKeyEnv = providerDef.DefaultEnvVar
	}

	if apiKeyEnv != "" {
		resolved.APIKey = os.Getenv(apiKeyEnv)
	}

	// Validate the resolved profile
	if err := s.validateResolvedProfile(resolved); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedProfile validates a resolved profile configuration.
func (s *serviceImpl) validateResolvedProfile(resolved *config.ResolvedProfile) error {
	if resolved == nil {
		return fmt.Errorf("resolved profile cannot be nil")
	}

	// Check timeout
	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute // Apply default
	} else if resolved.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second, got %v", resolved.Timeout)
	}

	// Check output tokens
	if resolved.MaxOutputTokens <= 0 {
		resolved.MaxOutputTokens = 4096 // Apply default
	} else if resolved.MaxOutputTokens > 200000 {
		return fmt.Errorf("max output tokens too large: %d (max 200000)", resolved.MaxOutputTokens)
	}

	// Check input tokens
	if resolved.MaxInputTokens <= 0 {
		resolved.MaxInputTokens = 128000 // Apply default
	} else if resolved.MaxInputTokens > 2000000 {
		return fmt.Errorf("max input tokens too large: %d (max 2000000)", resolved.MaxInputTokens)
	}

	// Check model
	if resolved.Model == "" {
		return fmt.Errorf("model name is required")
	}

	return nil
}
