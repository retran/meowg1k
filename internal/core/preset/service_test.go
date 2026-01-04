// Copyright © 2025 The meowg1k Authors
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

// Mock implementations for testing

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
	configReader := &mockConfigResolver{}
	modelService := &mockModelService{}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if service == nil {
		t.Fatal("Service should not be nil")
	}
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
					Enabled: true,
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
					Enabled: false,
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
	if resolved.Cache == nil || resolved.Cache.Enabled {
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
