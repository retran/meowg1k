// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package profile provides services for managing LLM provider profiles with rate limiting and cost tracking.
package profile

import (
	"fmt"
	"sync"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
)

// ModelResolver resolves model configurations.
type ModelResolver interface {
	Get(model model.Model) (*model.ResolvedModel, error)
}

// Service resolves and caches profile configurations.
type Service struct {
	modelResolver    ModelResolver
	configResolver   ports.ConfigResolver
	resolvedProfiles map[profile.Profile]*profile.ResolvedProfile
	mu               sync.RWMutex
}

// NewService creates a new profile resolver service.
func NewService(configResolver ports.ConfigResolver, modelResolver ModelResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if modelResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}

	service := &Service{
		modelResolver:    modelResolver,
		configResolver:   configResolver,
		resolvedProfiles: make(map[profile.Profile]*profile.ResolvedProfile),
	}
	return service, nil
}

// Get retrieves a profile using cached data from initialization.
func (s *Service) Get(profileID profile.Profile) (*profile.ResolvedProfile, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}

	s.mu.RLock()
	if resolved, exists := s.resolvedProfiles[profileID]; exists {
		s.mu.RUnlock()
		return resolved, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if resolved, exists := s.resolvedProfiles[profileID]; exists {
		return resolved, nil
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	resolved, err := s.resolveProfileInternal(profileID, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %q: %w", profileID, err)
	}

	s.resolvedProfiles[profileID] = resolved

	return resolved, nil
}

// resolveProfileInternal performs the actual profile resolution logic.
func (s *Service) resolveProfileInternal(
	profileName profile.Profile,
	cfg *config.Config,
) (*profile.ResolvedProfile, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}

	if profileName == "" {
		return nil, fmt.Errorf("profile not found in configuration")
	}

	if cfg == nil {
		return nil, fmt.Errorf("config reader is nil")
	}

	if cfg.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined in configuration")
	}

	profileDef, exists := cfg.Profiles[string(profileName)]
	if !exists {
		return nil, fmt.Errorf("profile not found in configuration: %s", profileName)
	}

	if profileDef.Model == "" {
		return nil, fmt.Errorf("profile must reference a model: profileName '%s'", profileName)
	}

	resolvedModel, err := s.modelResolver.Get(model.Model(profileDef.Model))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model '%s' for profileName '%s': %w", profileDef.Model, profileName, err)
	}

	resolved := &profile.ResolvedProfile{
		Name:              string(profileName),
		ModelID:           resolvedModel.ID,
		Provider:          resolvedModel.Provider,
		Model:             resolvedModel.Model,
		MaxInputTokens:    resolvedModel.MaxInputTokens,
		MaxOutputTokens:   resolvedModel.MaxOutputTokens,
		BaseURL:           resolvedModel.BaseURL,
		APIKey:            resolvedModel.APIKey,
		APIKeyEnv:         resolvedModel.APIKeyEnv,
		TokenizerType:     resolvedModel.Tokenizer,
		RateLimit:         resolvedModel.RateLimit,
		Timeout:           profileDef.Timeout,
		Temperature:       profileDef.Temperature,
		TopP:              profileDef.TopP,
		TopK:              profileDef.TopK,
		FrequencyPenalty:  profileDef.FrequencyPenalty,
		PresencePenalty:   profileDef.PresencePenalty,
		Seed:              profileDef.Seed,
		Stop:              profileDef.Stop,
		ResponseFormat:    profileDef.ResponseFormat,
		ResponseSchema:    profileDef.ResponseSchema,
		CandidateCount:    profileDef.CandidateCount,
		LogProbs:          profileDef.LogProbs,
		TopLogProbs:       profileDef.TopLogProbs,
		LogitBias:         profileDef.LogitBias,
		ServiceTier:       profileDef.ServiceTier,
		User:              profileDef.User,
		RepetitionPenalty: profileDef.RepetitionPenalty,
		MinP:              profileDef.MinP,
		TopA:              profileDef.TopA,
		TypicalP:          profileDef.TypicalP,
		Mirostat:          profileDef.Mirostat,
		MirostatTau:       profileDef.MirostatTau,
		MirostatEta:       profileDef.MirostatEta,
		Grammar:           profileDef.Grammar,
	}

	// Merge cache configuration (profile overrides global)
	if profileDef.Cache != nil {
		// Profile-specific cache config
		resolved.CacheEnabled = profileDef.Cache.Enabled
		resolved.CacheTTL = profileDef.Cache.TTL
	} else if cfg.Cache != nil {
		// Use global cache config
		resolved.CacheEnabled = cfg.Cache.Enabled
		resolved.CacheTTL = cfg.Cache.TTL
	}
	// Otherwise, caching is disabled (CacheEnabled defaults to false)

	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute
	}

	if profileDef.MaxTokens != nil && *profileDef.MaxTokens > 0 {
		resolved.MaxOutputTokens = *profileDef.MaxTokens
	}

	if err := s.validateResolvedProfile(resolved); err != nil {
		return nil, fmt.Errorf("profileName validation failed: %w", err)
	}

	return resolved, nil
}

// validateResolvedProfile validates a resolved profile configuration.
func (s *Service) validateResolvedProfile(resolved *profile.ResolvedProfile) error {
	if s == nil {
		return fmt.Errorf("service is nil")
	}

	if resolved == nil {
		return fmt.Errorf("resolved profile cannot be nil")
	}

	if resolved.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second, got %v", resolved.Timeout)
	}

	if resolved.Model == "" {
		return fmt.Errorf("resolved profile has empty model name")
	}

	if resolved.ModelID == "" {
		return fmt.Errorf("resolved profile has empty model ID")
	}

	return nil
}
