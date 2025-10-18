// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/model"
	profile2 "github.com/retran/meowg1k/internal/domain/profile"
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

func TestGetProfileSuccess(t *testing.T) {
	// Setup mock adapters
	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Profiles: map[string]*config.ProfileDefinition{
			"test-profile": {
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

	// Test getting a profile
	profile, err := service.Get(profile2.Profile("test-profile"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if profile == nil {
		t.Fatal("Profile should not be nil")
	}

	if profile.Provider != provider.OpenAI {
		t.Errorf("Expected provider %s, got %s", provider.OpenAI, profile.Provider)
	}

	if profile.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", profile.Model)
	}

	if profile.ModelID != "gpt4" {
		t.Errorf("Expected model ID 'gpt4', got '%s'", profile.ModelID)
	}

	if profile.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("Expected APIKeyEnv 'OPENAI_API_KEY', got '%s'", profile.APIKeyEnv)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	config := &config.Config{
		Profiles: map[string]*config.ProfileDefinition{},
	}

	configReader := &mockConfigResolver{config: config}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// Test getting a non-existent profile
	_, err = service.Get(profile2.Profile("non-existent"))
	if err == nil {
		t.Error("Expected error for non-existent profile")
	}
}

func TestGetProfileNoProfilesConfigured(t *testing.T) {
	config := &config.Config{
		Profiles: nil,
	}

	configReader := &mockConfigResolver{config: config}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// Test getting a profile when no profiles are configured
	_, err = service.Get(profile2.Profile("test"))
	if err == nil {
		t.Error("Expected error when no profiles are configured")
	}
}

func TestGetProfileModelNotFound(t *testing.T) {
	// Test profile that references non-existent model
	config := &config.Config{
		Profiles: map[string]*config.ProfileDefinition{
			"test": {
				Model: "non-existent-model",
			},
		},
	}

	configReader := &mockConfigResolver{config: config}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get(profile2.Profile("test"))
	if err == nil {
		t.Error("Expected error for non-existent model")
	}
}

func TestGetProfileEmptyModelReference(t *testing.T) {
	// Test profile with empty model reference - should return ErrModelReferenceRequired
	config := &config.Config{
		Profiles: map[string]*config.ProfileDefinition{
			"test": {
				Model: "", // Empty model reference
			},
		},
	}

	configReader := &mockConfigResolver{config: config}
	modelService := &mockModelService{models: map[model.Model]*model.ResolvedModel{}}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get(profile2.Profile("test"))
	if err == nil {
		t.Error("Expected error for empty model reference")
	}
	if err != nil && !strings.Contains(err.Error(), "profile must reference a model") {
		t.Errorf("Expected error to mention 'profile must reference a model', got: %v", err)
	}
}

func TestGetProfileWithMaxTokensOverride(t *testing.T) {
	maxTokens := 2000
	config := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Profiles: map[string]*config.ProfileDefinition{
			"test": {
				Model:     "gpt4",
				MaxTokens: &maxTokens,
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

	configReader := &mockConfigResolver{config: config}
	modelService := &mockModelService{
		models: map[model.Model]*model.ResolvedModel{
			"gpt4": resolvedModel,
		},
	}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	profile, err := service.Get(profile2.Profile("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Profile should override max output tokens
	if profile.MaxOutputTokens != maxTokens {
		t.Errorf("Expected MaxOutputTokens %d, got %d", maxTokens, profile.MaxOutputTokens)
	}
}

func TestGetProfileWithTemperature(t *testing.T) {
	temp := 0.7
	config := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Profiles: map[string]*config.ProfileDefinition{
			"test": {
				Model:       "gpt4",
				Temperature: &temp,
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

	configReader := &mockConfigResolver{config: config}
	modelService := &mockModelService{
		models: map[model.Model]*model.ResolvedModel{
			"gpt4": resolvedModel,
		},
	}

	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	profile, err := service.Get(profile2.Profile("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if profile.Temperature == nil {
		t.Fatal("Expected temperature to be set")
	}

	if *profile.Temperature != temp {
		t.Errorf("Expected temperature %f, got %f", temp, *profile.Temperature)
	}
}

func TestValidateResolvedProfileSuccess(t *testing.T) {
	configReader := &mockConfigResolver{}
	modelService := &mockModelService{}
	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	validProfile := &profile2.ResolvedProfile{
		ModelID:         "gpt4",
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
	}

	err = service.validateResolvedProfile(validProfile)
	if err != nil {
		t.Errorf("Expected no error for valid profile, got %v", err)
	}
}

func TestValidateResolvedProfileErrors(t *testing.T) {
	configReader := &mockConfigResolver{}
	modelService := &mockModelService{}
	service, err := NewService(configReader, modelService)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	testCases := []struct {
		name    string
		profile *profile2.ResolvedProfile
	}{
		{
			name:    "nil profile",
			profile: nil,
		},
		{
			name: "too short timeout",
			profile: &profile2.ResolvedProfile{
				Model:   "gpt-4",
				Timeout: 500 * time.Millisecond,
			},
		},
		{
			name: "too many output tokens",
			profile: &profile2.ResolvedProfile{
				Model:           "gpt-4",
				MaxOutputTokens: 300000,
				Timeout:         5 * time.Minute,
			},
		},
		{
			name: "too many input tokens",
			profile: &profile2.ResolvedProfile{
				Model:          "gpt-4",
				MaxInputTokens: 3000000,
				Timeout:        5 * time.Minute,
			},
		},
		{
			name: "empty model",
			profile: &profile2.ResolvedProfile{
				Model:   "",
				Timeout: 5 * time.Minute,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.validateResolvedProfile(tc.profile)
			if err == nil {
				t.Errorf("Expected error for %s", tc.name)
			}
		})
	}
}

func TestCaching(t *testing.T) {
	config := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
		Profiles: map[string]*config.ProfileDefinition{
			"cached-profile": {
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

	configReader := &mockConfigResolver{config: config}
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
	profile1, err := service.Get(profile2.Profile("cached-profile"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second call - should return cached result
	profile2, err := service.Get(profile2.Profile("cached-profile"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be the same instance (cached)
	if profile1 != profile2 {
		t.Error("Expected same profile instance from cache")
	}
}
