// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	domainConfig "github.com/retran/meowg1k/internal/domain/config"
)

// ApplyConfigToYAML converts Starlark configuration to domain YAML config format.
// This allows Starlark scripts to override or extend YAML-based configuration.
func (r *Runtime) ApplyConfigToYAML(baseConfig *domainConfig.Config) (*domainConfig.Config, error) {
	if baseConfig == nil {
		baseConfig = &domainConfig.Config{
			Providers: make(map[string]*domainConfig.ProviderConfig),
			Models:    make(map[string]*domainConfig.ModelConfig),
			Presets:   make(map[string]*domainConfig.PresetConfig),
		}
	}

	for name, providerCfg := range r.providers {
		domainProvider := &domainConfig.ProviderConfig{
			Type:       providerCfg.Type,
			BaseURL:    providerCfg.BaseURL,
			APIKey:     providerCfg.APIKey,
			Tokenizer:  providerCfg.Tokenizer,
			RetryCount: providerCfg.RetryCount,
		}

		baseConfig.Providers[name] = domainProvider
	}

	for name, modelCfg := range r.models {
		domainModel := &domainConfig.ModelConfig{
			Provider: modelCfg.Provider,
			Model:    modelCfg.Model,
		}

		if modelCfg.MaxInputTokens > 0 || modelCfg.MaxOutputTokens > 0 {
			domainModel.Limits = &domainConfig.ModelLimits{
				MaxInputTokens:  modelCfg.MaxInputTokens,
				MaxOutputTokens: modelCfg.MaxOutputTokens,
			}
		}

		if modelCfg.RateLimitRPM > 0 || modelCfg.RateLimitTPM > 0 || modelCfg.RateLimitRPD > 0 {
			domainModel.RateLimit = &domainConfig.RateLimitConfig{
				RequestsPerMinute: modelCfg.RateLimitRPM,
				TokensPerMinute:   modelCfg.RateLimitTPM,
				RequestsPerDay:    modelCfg.RateLimitRPD,
			}
		}

		baseConfig.Models[name] = domainModel
	}

	for name, presetCfg := range r.presets {
		requestCfg := &domainConfig.RequestConfig{}

		if presetCfg.Temperature != 0 {
			requestCfg.Temperature = float64Ptr(presetCfg.Temperature)
		}

		if presetCfg.MaxTokens != 0 {
			requestCfg.MaxTokens = intPtr(presetCfg.MaxTokens)
		}

		if presetCfg.TopP != 0 {
			requestCfg.TopP = float64Ptr(presetCfg.TopP)
		}

		if presetCfg.TopK != 0 {
			requestCfg.TopK = intPtr(presetCfg.TopK)
		}

		if presetCfg.FrequencyPenalty != 0 {
			requestCfg.FrequencyPenalty = float64Ptr(presetCfg.FrequencyPenalty)
		}

		if presetCfg.PresencePenalty != 0 {
			requestCfg.PresencePenalty = float64Ptr(presetCfg.PresencePenalty)
		}

		domainPreset := &domainConfig.PresetConfig{
			Model:   presetCfg.Model,
			Extends: presetCfg.Extends,
			Request: requestCfg,
		}

		baseConfig.Presets[name] = domainPreset
	}

	return baseConfig, nil
}

// HasConfiguration returns true if any Starlark configuration has been defined.
func (r *Runtime) HasConfiguration() bool {
	return len(r.providers) > 0 || len(r.models) > 0 || len(r.presets) > 0
}

// Helper functions for pointer conversion
func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func float64Ptr(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

// ValidateConfiguration checks if Starlark configuration is valid.
func (r *Runtime) ValidateConfiguration() error {
	for modelName, modelCfg := range r.models {
		if _, exists := r.providers[modelCfg.Provider]; !exists {
			return fmt.Errorf("model %q references unknown provider %q", modelName, modelCfg.Provider)
		}
	}

	for presetName, presetCfg := range r.presets {
		if _, exists := r.models[presetCfg.Model]; !exists {
			return fmt.Errorf("preset %q references unknown model %q", presetName, presetCfg.Model)
		}
	}

	return nil
}
