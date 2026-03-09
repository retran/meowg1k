// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package preset

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/model"
	preset2 "github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/domain/provider"
)

// Helper functions for testing.

func boolPtr(b bool) *bool {
	return &b
}

// Mock implementations for testing.

var errModelNotFound = fmt.Errorf("model not found")

type mockConfigResolver struct {
	config *config.Config
	err    error
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.config, nil
}

type mockModelService struct {
	models map[model.Model]*model.ResolvedModel
}

func (m *mockModelService) Get(modelRef model.Model) (*model.ResolvedModel, error) {
	if resolved, exists := m.models[modelRef]; exists {
		return resolved, nil
	}
	return nil, fmt.Errorf("%w: '%s'", errModelNotFound, modelRef)
}

func (m *mockModelService) GetInstanceKey(resolved *model.ResolvedModel) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		resolved.Provider,
		resolved.BaseURL,
		resolved.Model,
		resolved.APIKeyEnv,
	)
}

func TestNewService(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		configReader := &mockConfigResolver{}
		modelService := &mockModelService{}

		service, err := NewService(configReader, modelService)
		if err != nil {
			t.Fatalf("NewService returned error: %v", err)
		}
		if service == nil {
			t.Fatal("Service should not be nil")
		}
	})

	t.Run("nil config resolver", func(t *testing.T) {
		modelService := &mockModelService{}

		service, err := NewService(nil, modelService)
		if err == nil {
			t.Fatal("Expected error for nil config resolver")
		}
		if service != nil {
			t.Fatal("Service should be nil on error")
		}
		if !strings.Contains(err.Error(), "config resolver is nil") {
			t.Errorf("Expected error about config resolver, got: %v", err)
		}
	})

	t.Run("nil model resolver", func(t *testing.T) {
		configReader := &mockConfigResolver{}

		service, err := NewService(configReader, nil)
		if err == nil {
			t.Fatal("Expected error for nil model resolver")
		}
		if service != nil {
			t.Fatal("Service should be nil on error")
		}
		if !strings.Contains(err.Error(), "preset resolver is nil") {
			t.Errorf("Expected error about preset resolver, got: %v", err)
		}
	})
}

func TestGetPresetSuccess(t *testing.T) {
	// Setup mock adapters
	cfg := &config.Config{
		Models: map[string]*config.ModelConfig{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Presets: map[string]*config.PresetConfig{
			"test-preset": {
				Model:   "gpt4",
				Timeout: 5 * time.Minute,
			},
		},
	}

	resolvedModel := &model.ResolvedModel{
		ID:              "gpt4",
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "test-key",
		APIKeyEnv:       "OPENAI_API_KEY",
		Tokenizer:       model.TokenizerCL100K,
		RateLimit: model.RateLimitConfig{
			RequestsPerMinute: 10,
			TokensPerMinute:   100000,
			RequestsPerDay:    1000,
		},
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{
		models: map[model.Model]*model.ResolvedModel{
			"gpt4": resolvedModel,
		},
	}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// Test getting a preset
	preset, err := service.Get(preset2.Preset("test-preset"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if preset == nil {
		t.Fatal("Preset should not be nil")
	}

	if preset.Provider != provider.OpenAI {
		t.Errorf("Expected provider %s, got %s", provider.OpenAI, preset.Provider)
	}

	if preset.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", preset.Model)
	}

	if preset.ModelID != "gpt4" {
		t.Errorf("Expected model ID 'gpt4', got '%s'", preset.ModelID)
	}

	if preset.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("Expected APIKeyEnv 'OPENAI_API_KEY', got '%s'", preset.APIKeyEnv)
	}
}

func TestGetPresetNotFound(t *testing.T) {
	cfg := &config.Config{
		Presets: map[string]*config.PresetConfig{},
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// Test getting a non-existent preset
	_, err = service.Get(preset2.Preset("non-existent"))
	if err == nil {
		t.Error("Expected error for non-existent preset")
	}
}

func TestGetPresetNoPresetsConfigured(t *testing.T) {
	cfg := &config.Config{
		Presets: nil,
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// Test getting a preset when no presets are configured
	_, err = service.Get(preset2.Preset("test"))
	if err == nil {
		t.Error("Expected error when no presets are configured")
	}
}

func TestGetPresetModelNotFound(t *testing.T) {
	// Test preset that references non-existent model
	cfg := &config.Config{
		Presets: map[string]*config.PresetConfig{
			"test": {
				Model: "non-existent-model",
			},
		},
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get(preset2.Preset("test"))
	if err == nil {
		t.Error("Expected error for non-existent model")
	}
}

func TestGetPresetEmptyModelReference(t *testing.T) {
	// Test preset with empty model reference - should return ErrModelReferenceRequired
	cfg := &config.Config{
		Presets: map[string]*config.PresetConfig{
			"test": {
				Model: "", // Empty model reference
			},
		},
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get(preset2.Preset("test"))
	if err == nil {
		t.Error("Expected error for empty model reference")
	}
	if err != nil && !strings.Contains(err.Error(), "preset must reference a model") {
		t.Errorf("Expected error to mention 'preset must reference a model', got: %v", err)
	}
}

func TestGetPresetWithMaxTokensOverride(t *testing.T) {
	maxTokens := 2000
	cfg := &config.Config{
		Models: map[string]*config.ModelConfig{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Presets: map[string]*config.PresetConfig{
			"test": {
				Model: "gpt4",
				Request: &config.RequestConfig{
					MaxTokens: &maxTokens,
				},
			},
		},
	}

	resolvedModel := &model.ResolvedModel{
		ID:              "gpt4",
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxOutputTokens: 4096,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "test-key",
		APIKeyEnv:       "OPENAI_API_KEY",
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{
		models: map[model.Model]*model.ResolvedModel{
			"gpt4": resolvedModel,
		},
	}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	preset, err := service.Get(preset2.Preset("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Preset should override max output tokens
	if preset.MaxOutputTokens != maxTokens {
		t.Errorf("Expected MaxOutputTokens %d, got %d", maxTokens, preset.MaxOutputTokens)
	}
}

func TestGetPresetWithTemperature(t *testing.T) {
	temp := 0.7
	cfg := &config.Config{
		Models: map[string]*config.ModelConfig{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Presets: map[string]*config.PresetConfig{
			"test": {
				Model: "gpt4",
				Request: &config.RequestConfig{
					Temperature: &temp,
				},
			},
		},
	}

	resolvedModel := &model.ResolvedModel{
		ID:        "gpt4",
		Provider:  provider.OpenAI,
		Model:     "gpt-4",
		BaseURL:   "https://api.openai.com/v1",
		APIKey:    "test-key",
		APIKeyEnv: "OPENAI_API_KEY",
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{
		models: map[model.Model]*model.ResolvedModel{
			"gpt4": resolvedModel,
		},
	}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	preset, err := service.Get(preset2.Preset("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if preset.Temperature == nil {
		t.Fatal("Expected temperature to be set")
	}

	if *preset.Temperature != temp {
		t.Errorf("Expected temperature %f, got %f", temp, *preset.Temperature)
	}
}

func TestValidateResolvedPresetSuccess(t *testing.T) {
	configReader := &mockConfigResolver{}
	modelService := &mockModelService{}
	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	validPreset := &preset2.ResolvedPreset{
		ModelID:         "gpt4",
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
	}

	err = service.validateResolvedPreset(validPreset)
	if err != nil {
		t.Errorf("Expected no error for valid preset, got %v", err)
	}
}

func TestValidateResolvedPresetErrors(t *testing.T) {
	configReader := &mockConfigResolver{}
	modelService := &mockModelService{}
	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	testCases := []struct {
		preset *preset2.ResolvedPreset
		name   string
	}{
		{
			name:   "nil preset",
			preset: nil,
		},
		{
			name: "too short timeout",
			preset: &preset2.ResolvedPreset{
				Model:   "gpt-4",
				Timeout: 500 * time.Millisecond,
			},
		},
		{
			name: "too many output tokens",
			preset: &preset2.ResolvedPreset{
				Model:           "gpt-4",
				MaxOutputTokens: 300000,
				Timeout:         5 * time.Minute,
			},
		},
		{
			name: "too many input tokens",
			preset: &preset2.ResolvedPreset{
				Model:          "gpt-4",
				MaxInputTokens: 3000000,
				Timeout:        5 * time.Minute,
			},
		},
		{
			name: "empty model",
			preset: &preset2.ResolvedPreset{
				Model:   "",
				Timeout: 5 * time.Minute,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.validateResolvedPreset(tc.preset)
			if err == nil {
				t.Errorf("Expected error for %s", tc.name)
			}
		})
	}
}

func TestCaching(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelConfig{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Presets: map[string]*config.PresetConfig{
			"cached-preset": {
				Model:   "gpt4",
				Timeout: 5 * time.Minute,
			},
		},
	}

	resolvedModel := &model.ResolvedModel{
		ID:              "gpt4",
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "test-key",
		APIKeyEnv:       "OPENAI_API_KEY",
		Tokenizer:       model.TokenizerCL100K,
	}

	configReader := &mockConfigResolver{config: cfg}
	modelService := &mockModelService{
		models: map[model.Model]*model.ResolvedModel{
			"gpt4": resolvedModel,
		},
	}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// First call - should resolve and cache
	preset1, err := service.Get(preset2.Preset("cached-preset"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second call - should return cached result
	cachedPreset, err := service.Get(preset2.Preset("cached-preset"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be the same instance (cached)
	if preset1 != cachedPreset {
		t.Error("Expected same preset instance from cache")
	}
}

func TestGetPresetConfigError(t *testing.T) {
	configReader := &mockConfigResolver{err: errors.New("config read failed")}
	modelService := &mockModelService{}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get(preset2.Preset("any"))
	if err == nil {
		t.Fatal("expected error from config resolver")
	}
}

func TestResolvePresetInheritance(t *testing.T) {
	temperature := 0.5
	topK := 10
	cfg := &config.Config{
		Presets: map[string]*config.PresetConfig{
			"base": {
				Model:   "gpt4",
				Timeout: 2 * time.Minute,
				Cache: &config.CacheConfig{
					Enabled: boolPtr(true),
					TTL:     10 * time.Minute,
				},
				Request: &config.RequestConfig{
					Temperature: &temperature,
				},
			},
			"child": {
				Extends: "base",
				Model:   "gpt4-turbo",
				Cache: &config.CacheConfig{
					Enabled: boolPtr(false),
				},
				Request: &config.RequestConfig{
					TopK: &topK,
				},
			},
		},
	}

	resolved, err := resolvePreset("child", cfg)
	if err != nil {
		t.Fatalf("resolvePreset error: %v", err)
	}

	if resolved.Model != "gpt4-turbo" {
		t.Errorf("expected model override, got %s", resolved.Model)
	}
	if resolved.Cache == nil {
		t.Fatal("expected cache config to be set")
	}
	if resolved.Cache.Enabled == nil || *resolved.Cache.Enabled {
		t.Fatal("expected child cache to override parent and be disabled")
	}
	if resolved.Request == nil || resolved.Request.Temperature == nil || resolved.Request.TopK == nil {
		t.Fatal("expected merged request fields from parent and child")
	}
	if *resolved.Request.Temperature != temperature {
		t.Errorf("expected temperature %f, got %f", temperature, *resolved.Request.Temperature)
	}
	if *resolved.Request.TopK != topK {
		t.Errorf("expected TopK %d, got %d", topK, *resolved.Request.TopK)
	}
}

func TestResolvePresetCycle(t *testing.T) {
	cfg := &config.Config{
		Presets: map[string]*config.PresetConfig{
			"a": {Extends: "b"},
			"b": {Extends: "a"},
		},
	}

	_, err := resolvePreset("a", cfg)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestResolvePresetNilConfig(t *testing.T) {
	_, err := resolvePreset("any", nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestResolvePresetRequestInitialization(t *testing.T) {
	cfg := &config.Config{
		Presets: map[string]*config.PresetConfig{
			"plain": {
				Model: "gpt4",
			},
		},
	}

	resolved, err := resolvePreset("plain", cfg)
	if err != nil {
		t.Fatalf("resolvePreset error: %v", err)
	}
	if resolved.Request == nil {
		t.Fatal("expected request to be initialized")
	}
}

func TestMergeRequestNilHandling(t *testing.T) {
	t.Run("nil dst and src returns empty request", func(t *testing.T) {
		merged := mergeRequest(nil, nil)
		if merged == nil {
			t.Fatal("expected non-nil request")
		}
	})

	t.Run("nil dst merges from src", func(t *testing.T) {
		count := 2
		merged := mergeRequest(nil, &config.RequestConfig{CandidateCount: &count})
		if merged == nil || merged.CandidateCount == nil || *merged.CandidateCount != count {
			t.Fatal("expected candidate count to be merged")
		}
	})
}

func TestServiceNilReceiver(t *testing.T) {
	var service *Service
	if _, err := service.Get("preset"); err == nil {
		t.Fatal("expected error when service is nil")
	}
}

func TestResolvePresetInternalNilService(t *testing.T) {
	var service *Service
	if _, err := service.resolvePresetInternal("preset", &config.Config{}); err == nil {
		t.Fatal("expected error when service is nil")
	}
}

// Test merge functions for all parameter types.

func TestMergePenaltyParams(t *testing.T) {
	freq := 0.5
	pres := 0.3
	rep := 1.2

	src := &config.RequestConfig{
		FrequencyPenalty:  &freq,
		PresencePenalty:   &pres,
		RepetitionPenalty: &rep,
	}

	dst := &config.RequestConfig{}
	mergePenaltyParams(dst, src)

	if dst.FrequencyPenalty == nil || *dst.FrequencyPenalty != freq {
		t.Errorf("expected FrequencyPenalty %f, got %v", freq, dst.FrequencyPenalty)
	}
	if dst.PresencePenalty == nil || *dst.PresencePenalty != pres {
		t.Errorf("expected PresencePenalty %f, got %v", pres, dst.PresencePenalty)
	}
	if dst.RepetitionPenalty == nil || *dst.RepetitionPenalty != rep {
		t.Errorf("expected RepetitionPenalty %f, got %v", rep, dst.RepetitionPenalty)
	}
}

func TestMergeResponseParams(t *testing.T) {
	logProbs := true
	topLogProbs := 5

	src := &config.RequestConfig{
		LogProbs:    &logProbs,
		TopLogProbs: &topLogProbs,
	}

	dst := &config.RequestConfig{}
	mergeResponseParams(dst, src)

	if dst.LogProbs == nil || *dst.LogProbs != logProbs {
		t.Errorf("expected LogProbs %v, got %v", logProbs, dst.LogProbs)
	}
	if dst.TopLogProbs == nil || *dst.TopLogProbs != topLogProbs {
		t.Errorf("expected TopLogProbs %d, got %v", topLogProbs, dst.TopLogProbs)
	}
}

func TestMergeAdvancedParams(t *testing.T) {
	minP := 0.1
	topA := 0.9
	typicalP := 0.8
	mirostat := 2
	mirostatTau := 5.0
	mirostatEta := 0.1

	src := &config.RequestConfig{
		MinP:        &minP,
		TopA:        &topA,
		TypicalP:    &typicalP,
		Mirostat:    &mirostat,
		MirostatTau: &mirostatTau,
		MirostatEta: &mirostatEta,
	}

	dst := &config.RequestConfig{}
	mergeAdvancedParams(dst, src)

	if dst.MinP == nil || *dst.MinP != minP {
		t.Errorf("expected MinP %f, got %v", minP, dst.MinP)
	}
	if dst.TopA == nil || *dst.TopA != topA {
		t.Errorf("expected TopA %f, got %v", topA, dst.TopA)
	}
	if dst.TypicalP == nil || *dst.TypicalP != typicalP {
		t.Errorf("expected TypicalP %f, got %v", typicalP, dst.TypicalP)
	}
	if dst.Mirostat == nil || *dst.Mirostat != mirostat {
		t.Errorf("expected Mirostat %d, got %v", mirostat, dst.Mirostat)
	}
	if dst.MirostatTau == nil || *dst.MirostatTau != mirostatTau {
		t.Errorf("expected MirostatTau %f, got %v", mirostatTau, dst.MirostatTau)
	}
	if dst.MirostatEta == nil || *dst.MirostatEta != mirostatEta {
		t.Errorf("expected MirostatEta %f, got %v", mirostatEta, dst.MirostatEta)
	}
}

func TestMergeOtherParams(t *testing.T) {
	seed := 12345
	grammar := "test-grammar"
	logitBias := map[string]int{"token1": 1}
	serviceTier := "premium"
	user := "test-user"
	stop := []string{"stop1", "stop2"}

	src := &config.RequestConfig{
		Seed:        &seed,
		Grammar:     &grammar,
		LogitBias:   logitBias,
		ServiceTier: &serviceTier,
		User:        &user,
		Stop:        stop,
	}

	dst := &config.RequestConfig{}
	mergeOtherParams(dst, src)

	if dst.Seed == nil || *dst.Seed != seed {
		t.Errorf("expected Seed %d, got %v", seed, dst.Seed)
	}
	if dst.Grammar == nil || *dst.Grammar != grammar {
		t.Errorf("expected Grammar %s, got %v", grammar, dst.Grammar)
	}
	if dst.LogitBias == nil {
		t.Error("expected LogitBias to be set")
	}
	if dst.ServiceTier == nil || *dst.ServiceTier != serviceTier {
		t.Errorf("expected ServiceTier %s, got %v", serviceTier, dst.ServiceTier)
	}
	if dst.User == nil || *dst.User != user {
		t.Errorf("expected User %s, got %v", user, dst.User)
	}
	if dst.Stop == nil {
		t.Error("expected Stop to be set")
	}
}

// Test clone functions.

func TestClonePreset(t *testing.T) {
	t.Run("clone non-nil preset", func(t *testing.T) {
		enabled := true
		temp := 0.7

		original := &config.PresetConfig{
			Extends: "base",
			Model:   "gpt4",
			Timeout: 5 * time.Minute,
			Cache: &config.CacheConfig{
				Enabled: &enabled,
				TTL:     10 * time.Minute,
			},
			Request: &config.RequestConfig{
				Temperature: &temp,
			},
			Labels: map[string]any{"env": "prod"},
		}

		cloned := clonePreset(original)
		if cloned == nil {
			t.Fatal("expected non-nil cloned preset")
		}
		if cloned.Model != original.Model {
			t.Errorf("expected model %s, got %s", original.Model, cloned.Model)
		}
		if cloned.Timeout != original.Timeout {
			t.Errorf("expected timeout %v, got %v", original.Timeout, cloned.Timeout)
		}
		// Verify it's a deep clone by modifying original
		if cloned.Cache == original.Cache {
			t.Error("expected cache to be cloned, not same instance")
		}
	})

	t.Run("clone nil preset", func(t *testing.T) {
		cloned := clonePreset(nil)
		if cloned == nil {
			t.Fatal("expected non-nil result for nil preset")
		}
		if cloned.Request == nil {
			t.Fatal("expected request to be initialized")
		}
	})
}

func TestCloneCache(t *testing.T) {
	t.Run("clone non-nil cache", func(t *testing.T) {
		enabled := true
		original := &config.CacheConfig{
			Enabled: &enabled,
			TTL:     10 * time.Minute,
		}

		cloned := cloneCache(original)
		if cloned == nil {
			t.Fatal("expected non-nil cloned cache")
		}
		if cloned.Enabled == nil || *cloned.Enabled != *original.Enabled {
			t.Error("expected enabled to be cloned")
		}
		if cloned.TTL != original.TTL {
			t.Errorf("expected TTL %v, got %v", original.TTL, cloned.TTL)
		}
		// Verify it's a separate instance
		if cloned == original {
			t.Error("expected different cache instance")
		}
	})

	t.Run("clone nil cache", func(t *testing.T) {
		cloned := cloneCache(nil)
		if cloned != nil {
			t.Error("expected nil result for nil cache")
		}
	})
}

func TestCloneRequest(t *testing.T) {
	t.Run("clone non-nil request", func(t *testing.T) {
		temp := 0.8
		original := &config.RequestConfig{
			Temperature: &temp,
		}

		cloned := cloneRequest(original)
		if cloned == nil {
			t.Fatal("expected non-nil cloned request")
		}
		if cloned.Temperature == nil || *cloned.Temperature != temp {
			t.Errorf("expected temperature %f, got %v", temp, cloned.Temperature)
		}
	})

	t.Run("clone nil request", func(t *testing.T) {
		cloned := cloneRequest(nil)
		if cloned == nil {
			t.Fatal("expected non-nil result for nil request")
		}
	})
}

// Test applyPreset edge cases.

func TestApplyPreset(t *testing.T) {
	t.Run("apply with both nil", func(t *testing.T) {
		applyPreset(nil, nil) // Should not panic
	})

	t.Run("apply with nil dst", func(t *testing.T) {
		src := &config.PresetConfig{Model: "gpt4"}
		applyPreset(nil, src) // Should not panic
	})

	t.Run("apply with nil src", func(t *testing.T) {
		dst := &config.PresetConfig{Model: "gpt3"}
		applyPreset(dst, nil) // Should not panic
		if dst.Model != "gpt3" {
			t.Error("dst should not be modified")
		}
	})

	t.Run("apply full config", func(t *testing.T) {
		enabled := true
		temp := 0.5

		dst := &config.PresetConfig{
			Model: "gpt3",
		}

		src := &config.PresetConfig{
			Model:   "gpt4",
			Timeout: 3 * time.Minute,
			Cache: &config.CacheConfig{
				Enabled: &enabled,
				TTL:     5 * time.Minute,
			},
			Request: &config.RequestConfig{
				Temperature: &temp,
			},
		}

		applyPreset(dst, src)

		if dst.Model != "gpt4" {
			t.Errorf("expected model to be overridden to gpt4, got %s", dst.Model)
		}
		if dst.Timeout != 3*time.Minute {
			t.Errorf("expected timeout to be set to 3m, got %v", dst.Timeout)
		}
		if dst.Cache == nil {
			t.Fatal("expected cache to be set")
		}
		if dst.Request == nil || dst.Request.Temperature == nil {
			t.Fatal("expected request with temperature to be set")
		}
	})
}
