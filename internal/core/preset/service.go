// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package preset provides services for managing LLM provider presets with rate limiting and cost tracking.
package preset

import (
	"fmt"
	"sync"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/model"
	domainpreset "github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
)

// ModelResolver resolves model configurations.
type ModelResolver interface {
	Get(model model.Model) (*model.ResolvedModel, error)
}

// Service resolves and caches preset configurations.
type Service struct {
	modelResolver   ModelResolver
	configResolver  ports.ConfigResolver
	resolvedPresets map[domainpreset.Preset]*domainpreset.ResolvedPreset
	mu              sync.RWMutex
}

// NewService creates a new preset resolver service.
func NewService(configResolver ports.ConfigResolver, modelResolver ModelResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	if modelResolver == nil {
		return nil, fmt.Errorf("preset resolver is nil")
	}

	service := &Service{
		modelResolver:   modelResolver,
		configResolver:  configResolver,
		resolvedPresets: make(map[domainpreset.Preset]*domainpreset.ResolvedPreset),
	}
	return service, nil
}

// Get retrieves a preset using cached data from initialization.
func (s *Service) Get(presetID domainpreset.Preset) (*domainpreset.ResolvedPreset, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}

	s.mu.RLock()
	if resolved, exists := s.resolvedPresets[presetID]; exists {
		s.mu.RUnlock()
		return resolved, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if resolved, exists := s.resolvedPresets[presetID]; exists {
		return resolved, nil
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get application config: %w", err)
	}

	resolved, err := s.resolvePresetInternal(presetID, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %q: %w", presetID, err)
	}

	s.resolvedPresets[presetID] = resolved

	return resolved, nil
}

// resolvePresetInternal performs the actual preset resolution logic.
func (s *Service) resolvePresetInternal(
	presetName domainpreset.Preset,
	cfg *config.Config,
) (*domainpreset.ResolvedPreset, error) {
	if s == nil {
		return nil, fmt.Errorf("service is nil")
	}

	if presetName == "" {
		return nil, fmt.Errorf("preset not found in configuration")
	}

	if cfg == nil {
		return nil, fmt.Errorf("config reader is nil")
	}

	if cfg.Presets == nil {
		return nil, fmt.Errorf("no presets defined in configuration")
	}

	preset, err := resolvePreset(presetName, cfg)
	if err != nil {
		return nil, fmt.Errorf("preset resolution failed: %w", err)
	}

	if preset.Model == "" {
		return nil, fmt.Errorf("preset must reference a model: preset '%s'", presetName)
	}

	resolvedModel, err := s.modelResolver.Get(model.Model(preset.Model))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model '%s' for preset '%s': %w", preset.Model, presetName, err)
	}

	resolved := &domainpreset.ResolvedPreset{
		Name:              string(presetName),
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
		Timeout:           preset.Timeout,
		Temperature:       preset.Request.Temperature,
		TopP:              preset.Request.TopP,
		TopK:              preset.Request.TopK,
		FrequencyPenalty:  preset.Request.FrequencyPenalty,
		PresencePenalty:   preset.Request.PresencePenalty,
		Seed:              preset.Request.Seed,
		Stop:              preset.Request.Stop,
		ResponseFormat:    preset.Request.ResponseFormat,
		ResponseSchema:    preset.Request.ResponseSchema,
		CandidateCount:    preset.Request.CandidateCount,
		LogProbs:          preset.Request.LogProbs,
		TopLogProbs:       preset.Request.TopLogProbs,
		LogitBias:         preset.Request.LogitBias,
		ServiceTier:       preset.Request.ServiceTier,
		User:              preset.Request.User,
		RepetitionPenalty: preset.Request.RepetitionPenalty,
		MinP:              preset.Request.MinP,
		TopA:              preset.Request.TopA,
		TypicalP:          preset.Request.TypicalP,
		Mirostat:          preset.Request.Mirostat,
		MirostatTau:       preset.Request.MirostatTau,
		MirostatEta:       preset.Request.MirostatEta,
		Grammar:           preset.Request.Grammar,
	}

	// Merge cache configuration (preset overrides parent)
	if preset.Cache != nil {
		resolved.CacheEnabled = preset.Cache.Enabled
		resolved.CacheTTL = preset.Cache.TTL
	}
	// Otherwise, caching is disabled (CacheEnabled defaults to false)

	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute
	}

	if preset.Request.MaxTokens != nil && *preset.Request.MaxTokens > 0 {
		resolved.MaxOutputTokens = *preset.Request.MaxTokens
	}

	if err := s.validateResolvedPreset(resolved); err != nil {
		return nil, fmt.Errorf("preset validation failed: %w", err)
	}

	return resolved, nil
}

func resolvePreset(
	presetName domainpreset.Preset,
	cfg *config.Config,
) (*config.PresetConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if cfg.Presets == nil {
		return nil, fmt.Errorf("no presets defined in configuration")
	}

	cache := make(map[string]*config.PresetConfig)
	resolving := make(map[string]bool)

	var resolve func(string) (*config.PresetConfig, error)
	resolve = func(name string) (*config.PresetConfig, error) {
		if cached, ok := cache[name]; ok {
			return cached, nil
		}
		if resolving[name] {
			return nil, fmt.Errorf("preset inheritance cycle detected at %q", name)
		}

		presetDef, ok := cfg.Presets[name]
		if !ok {
			return nil, fmt.Errorf("preset not found in configuration: %s", name)
		}

		resolving[name] = true
		resolved := &config.PresetConfig{
			Request: &config.RequestConfig{},
		}

		if presetDef.Extends != "" {
			parent, err := resolve(presetDef.Extends)
			if err != nil {
				return nil, err
			}
			resolved = clonePreset(parent)
		}

		applyPreset(resolved, presetDef)

		resolving[name] = false
		cache[name] = resolved
		return resolved, nil
	}

	resolvedPreset, err := resolve(string(presetName))
	if err != nil {
		return nil, err
	}

	if resolvedPreset.Request == nil {
		resolvedPreset.Request = &config.RequestConfig{}
	}

	return resolvedPreset, nil
}

func applyPreset(dst *config.PresetConfig, src *config.PresetConfig) {
	if src == nil || dst == nil {
		return
	}

	if src.Model != "" {
		dst.Model = src.Model
	}

	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}

	if src.Cache != nil {
		dst.Cache = cloneCache(src.Cache)
	}

	if src.Request != nil {
		dst.Request = mergeRequest(dst.Request, src.Request)
	}
}

func clonePreset(src *config.PresetConfig) *config.PresetConfig {
	if src == nil {
		return &config.PresetConfig{Request: &config.RequestConfig{}}
	}
	return &config.PresetConfig{
		Extends: src.Extends,
		Model:   src.Model,
		Timeout: src.Timeout,
		Cache:   cloneCache(src.Cache),
		Request: cloneRequest(src.Request),
		Labels:  src.Labels,
	}
}

func cloneCache(src *config.CacheConfig) *config.CacheConfig {
	if src == nil {
		return nil
	}
	return &config.CacheConfig{
		Enabled: src.Enabled,
		TTL:     src.TTL,
	}
}

func cloneRequest(src *config.RequestConfig) *config.RequestConfig {
	if src == nil {
		return &config.RequestConfig{}
	}
	return mergeRequest(&config.RequestConfig{}, src)
}

func mergeRequest(dst *config.RequestConfig, src *config.RequestConfig) *config.RequestConfig {
	if dst == nil {
		dst = &config.RequestConfig{}
	}
	if src == nil {
		return dst
	}

	mergeSamplingParams(dst, src)
	mergePenaltyParams(dst, src)
	mergeResponseParams(dst, src)
	mergeAdvancedParams(dst, src)
	mergeOtherParams(dst, src)

	return dst
}

func mergeSamplingParams(dst, src *config.RequestConfig) {
	if src.CandidateCount != nil {
		dst.CandidateCount = src.CandidateCount
	}
	if src.Temperature != nil {
		dst.Temperature = src.Temperature
	}
	if src.TopP != nil {
		dst.TopP = src.TopP
	}
	if src.TopK != nil {
		dst.TopK = src.TopK
	}
	if src.MaxTokens != nil {
		dst.MaxTokens = src.MaxTokens
	}
}

func mergePenaltyParams(dst, src *config.RequestConfig) {
	if src.FrequencyPenalty != nil {
		dst.FrequencyPenalty = src.FrequencyPenalty
	}
	if src.PresencePenalty != nil {
		dst.PresencePenalty = src.PresencePenalty
	}
	if src.RepetitionPenalty != nil {
		dst.RepetitionPenalty = src.RepetitionPenalty
	}
}

func mergeResponseParams(dst, src *config.RequestConfig) {
	if src.ResponseFormat != nil {
		dst.ResponseFormat = src.ResponseFormat
	}
	if src.ResponseSchema != nil {
		dst.ResponseSchema = src.ResponseSchema
	}
	if src.LogProbs != nil {
		dst.LogProbs = src.LogProbs
	}
	if src.TopLogProbs != nil {
		dst.TopLogProbs = src.TopLogProbs
	}
}

func mergeAdvancedParams(dst, src *config.RequestConfig) {
	if src.MinP != nil {
		dst.MinP = src.MinP
	}
	if src.TopA != nil {
		dst.TopA = src.TopA
	}
	if src.TypicalP != nil {
		dst.TypicalP = src.TypicalP
	}
	if src.Mirostat != nil {
		dst.Mirostat = src.Mirostat
	}
	if src.MirostatTau != nil {
		dst.MirostatTau = src.MirostatTau
	}
	if src.MirostatEta != nil {
		dst.MirostatEta = src.MirostatEta
	}
}

func mergeOtherParams(dst, src *config.RequestConfig) {
	if src.Seed != nil {
		dst.Seed = src.Seed
	}
	if src.Grammar != nil {
		dst.Grammar = src.Grammar
	}
	if src.LogitBias != nil {
		dst.LogitBias = src.LogitBias
	}
	if src.ServiceTier != nil {
		dst.ServiceTier = src.ServiceTier
	}
	if src.User != nil {
		dst.User = src.User
	}
	if src.Stop != nil {
		dst.Stop = src.Stop
	}
}

// validateResolvedPreset validates a resolved preset configuration.
func (s *Service) validateResolvedPreset(resolved *domainpreset.ResolvedPreset) error {
	if s == nil {
		return fmt.Errorf("service is nil")
	}

	if resolved == nil {
		return fmt.Errorf("resolved preset cannot be nil")
	}

	if resolved.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second, got %v", resolved.Timeout)
	}

	if resolved.Model == "" {
		return fmt.Errorf("resolved preset has empty model name")
	}

	if resolved.ModelID == "" {
		return fmt.Errorf("resolved preset has empty model ID")
	}

	return nil
}
