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

package profile

import (
	"fmt"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/llm"
	"github.com/retran/meowg1k/internal/services/provider"
)

// Mock implementations for testing

var errProviderNotFound = fmt.Errorf("provider not found")

type mockConfigService struct {
	config *config.Config
}

func (m *mockConfigService) GetConfig() *config.Config {
	return m.config
}

type mockProviderService struct {
	providers map[provider.Provider]provider.ProviderDefinition
}

func (m *mockProviderService) Get(providerType provider.Provider) (provider.ProviderDefinition, error) {
	if provider, exists := m.providers[providerType]; exists {
		return provider, nil
	}
	return provider.ProviderDefinition{}, fmt.Errorf("%w: '%s'", errProviderNotFound, providerType)
}

func TestNewService(t *testing.T) {
	configService := &mockConfigService{}
	providerService := &mockProviderService{}

	service := NewService(configService, providerService)
	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestGetProfileSuccess(t *testing.T) {
	// Setup mock services
	config := &config.Config{
		Profiles: map[string]*config.ProfileDefinition{
			"test-profile": {
				Provider:        "openai",
				Model:           "gpt-4",
				MaxInputTokens:  128000,
				MaxOutputTokens: 4096,
				Timeout:         5 * time.Minute,
				TokenizerType:   llm.TokenizerCL100K,
			},
		},
	}

	providerDef := provider.ProviderDefinition{
		Type:            provider.OpenAI,
		DefaultModel:    "gpt-3.5-turbo",
		DefaultBaseURL:  "https://api.openai.com/v1",
		DefaultEnvVar:   "OPENAI_API_KEY",
		RequiresAPIKey:  true,
		RequiresBaseURL: false,
		TokenizerType:   llm.TokenizerCL100K,
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		DefaultTimeout:  5 * time.Minute,
	}

	configService := &mockConfigService{config: config}
	providerService := &mockProviderService{
		providers: map[provider.Provider]provider.ProviderDefinition{
			provider.OpenAI: providerDef,
		},
	}

	service := NewService(configService, providerService)

	// Test getting a profile
	profile, err := service.Get(Profile("test-profile"))
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
}

func TestGetProfileNotFound(t *testing.T) {
	config := &config.Config{
		Profiles: map[string]*config.ProfileDefinition{},
	}

	configService := &mockConfigService{config: config}
	providerService := &mockProviderService{providers: map[provider.Provider]provider.ProviderDefinition{}}

	service := NewService(configService, providerService)

	// Test getting a non-existent profile
	_, err := service.Get(Profile("non-existent"))
	if err == nil {
		t.Error("Expected error for non-existent profile")
	}
}

func TestGetProfileNoProfilesConfigured(t *testing.T) {
	config := &config.Config{
		Profiles: nil,
	}

	configService := &mockConfigService{config: config}
	providerService := &mockProviderService{providers: map[provider.Provider]provider.ProviderDefinition{}}

	service := NewService(configService, providerService)

	// Test getting a profile when no profiles are configured
	_, err := service.Get(Profile("test"))
	if err == nil {
		t.Error("Expected error when no profiles are configured")
	}
}

func TestGetProfileWithDefaults(t *testing.T) {
	// Test profile that uses provider defaults
	config := &config.Config{
		Profiles: map[string]*config.ProfileDefinition{
			"minimal-profile": {
				Provider: "openai",
				// No model, tokens, timeout specified - should use defaults
			},
		},
	}

	providerDef := provider.ProviderDefinition{
		Type:            provider.OpenAI,
		DefaultModel:    "gpt-3.5-turbo",
		DefaultBaseURL:  "https://api.openai.com/v1",
		MaxInputTokens:  100000,
		MaxOutputTokens: 2000,
		DefaultTimeout:  3 * time.Minute,
		TokenizerType:   llm.TokenizerCL100K,
	}

	configService := &mockConfigService{config: config}
	providerService := &mockProviderService{
		providers: map[provider.Provider]provider.ProviderDefinition{
			provider.OpenAI: providerDef,
		},
	}

	service := NewService(configService, providerService)

	profile, err := service.Get(Profile("minimal-profile"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should use provider defaults
	if profile.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected default model 'gpt-3.5-turbo', got '%s'", profile.Model)
	}

	if profile.MaxInputTokens != 100000 {
		t.Errorf("Expected default MaxInputTokens 100000, got %d", profile.MaxInputTokens)
	}

	if profile.MaxOutputTokens != 2000 {
		t.Errorf("Expected default MaxOutputTokens 2000, got %d", profile.MaxOutputTokens)
	}

	if profile.Timeout != 3*time.Minute {
		t.Errorf("Expected default timeout 3m, got %v", profile.Timeout)
	}

	if profile.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected default BaseURL 'https://api.openai.com/v1', got '%s'", profile.BaseURL)
	}
}

func TestValidateResolvedProfileSuccess(t *testing.T) {
	configService := &mockConfigService{}
	providerService := &mockProviderService{}
	service := NewService(configService, providerService).(*serviceImpl)

	validProfile := &ResolvedProfile{
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
	}

	err := service.validateResolvedProfile(validProfile)
	if err != nil {
		t.Errorf("Expected no error for valid profile, got %v", err)
	}
}

func TestValidateResolvedProfileErrors(t *testing.T) {
	configService := &mockConfigService{}
	providerService := &mockProviderService{}
	service := NewService(configService, providerService).(*serviceImpl)

	testCases := []struct {
		name    string
		profile *ResolvedProfile
	}{
		{
			name:    "nil profile",
			profile: nil,
		},
		{
			name: "too short timeout",
			profile: &ResolvedProfile{
				Model:   "gpt-4",
				Timeout: 500 * time.Millisecond,
			},
		},
		{
			name: "too many output tokens",
			profile: &ResolvedProfile{
				Model:           "gpt-4",
				MaxOutputTokens: 300000,
				Timeout:         5 * time.Minute,
			},
		},
		{
			name: "too many input tokens",
			profile: &ResolvedProfile{
				Model:          "gpt-4",
				MaxInputTokens: 3000000,
				Timeout:        5 * time.Minute,
			},
		},
		{
			name: "empty model",
			profile: &ResolvedProfile{
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
		Profiles: map[string]*config.ProfileDefinition{
			"cached-profile": {
				Provider: "openai",
				Model:    "gpt-4",
				Timeout:  5 * time.Minute,
			},
		},
	}

	providerDef := provider.ProviderDefinition{
		Type:            provider.OpenAI,
		DefaultModel:    "gpt-3.5-turbo",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		DefaultTimeout:  5 * time.Minute,
		TokenizerType:   llm.TokenizerCL100K,
	}

	configService := &mockConfigService{config: config}
	providerService := &mockProviderService{
		providers: map[provider.Provider]provider.ProviderDefinition{
			provider.OpenAI: providerDef,
		},
	}

	service := NewService(configService, providerService)

	// First call - should resolve and cache
	profile1, err := service.Get(Profile("cached-profile"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second call - should return cached result
	profile2, err := service.Get(Profile("cached-profile"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be the same instance (cached)
	if profile1 != profile2 {
		t.Error("Expected same profile instance from cache")
	}
}
