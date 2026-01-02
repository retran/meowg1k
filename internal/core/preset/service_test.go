// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package preset

import (
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
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
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
