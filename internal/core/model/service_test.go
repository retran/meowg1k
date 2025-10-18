// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/provider"
)

type mockServiceConfigResolver struct {
	config *config.Config
	err    error
}

func (m *mockServiceConfigResolver) Get() (*config.Config, error) {
	return m.config, m.err
}

type mockServiceProviderResolver struct {
	providers map[provider.Provider]provider.ProviderDefinition
	err       error
}

func (m *mockServiceProviderResolver) Get(p provider.Provider) (provider.ProviderDefinition, error) {
	if m.err != nil {
		return provider.ProviderDefinition{}, m.err
	}
	if def, ok := m.providers[p]; ok {
		return def, nil
	}
	return provider.ProviderDefinition{}, fmt.Errorf("provider not found: %s", p)
}

func TestNewService_Success(t *testing.T) {
	configResolver := &mockServiceConfigResolver{
		config: &config.Config{},
	}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.ProviderDefinition),
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestNewService_NilConfigResolver(t *testing.T) {
	providerResolver := &mockServiceProviderResolver{}

	service, err := NewService(nil, providerResolver)
	if err == nil {
		t.Fatal("Expected error for nil config resolver")
	}
	if service != nil {
		t.Fatal("Service should be nil on error")
	}
}

func TestNewService_NilProviderResolver(t *testing.T) {
	configResolver := &mockServiceConfigResolver{}

	service, err := NewService(configResolver, nil)
	if err == nil {
		t.Fatal("Expected error for nil provider registry")
	}
	if service != nil {
		t.Fatal("Service should be nil on error")
	}
}

func TestService_Get_Success(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"test-model": {
				Provider:        "openai",
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				BaseURL:         "https://api.openai.com/v1",
				APIKeyEnv:       "TEST_API_KEY",
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.ProviderDefinition{
			"openai": {
				Type:            "openai",
				DefaultModel:    "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				DefaultBaseURL:  "https://api.openai.com/v1",
				DefaultEnvVar:   "OPENAI_API_KEY",
			},
		},
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved, err := service.Get("test-model")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if resolved == nil {
		t.Fatal("Resolved model should not be nil")
	}
	if resolved.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", resolved.Model)
	}
}

func TestService_Get_Cached(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"test-model": {
				Provider:        "openai",
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.ProviderDefinition{
			"openai": {
				Type:            "openai",
				DefaultModel:    "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
			},
		},
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	// First call
	resolved1, err := service.Get("test-model")
	if err != nil {
		t.Fatalf("First Get returned error: %v", err)
	}

	// Second call - should be cached
	resolved2, err := service.Get("test-model")
	if err != nil {
		t.Fatalf("Second Get returned error: %v", err)
	}

	if resolved1 != resolved2 {
		t.Error("Expected cached result to be the same instance")
	}
}

func TestService_Get_ModelNotFound(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.ProviderDefinition),
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent model")
	}
}

func TestService_Get_NilService(t *testing.T) {
	var service *Service
	_, err := service.Get("test")
	if err == nil {
		t.Fatal("Expected error for nil service")
	}
}

func TestService_GetInstanceKey_Success(t *testing.T) {
	service := &Service{}
	resolved := &model.ResolvedModel{
		Provider:  "openai",
		BaseURL:   "https://api.openai.com/v1",
		Model:     "gpt-4",
		APIKeyEnv: "OPENAI_API_KEY",
	}

	key, err := service.GetInstanceKey(resolved)
	if err != nil {
		t.Fatalf("GetInstanceKey returned error: %v", err)
	}

	expected := "openai:https://api.openai.com/v1:gpt-4:OPENAI_API_KEY"
	if key != expected {
		t.Errorf("Expected key '%s', got '%s'", expected, key)
	}
}

func TestService_GetInstanceKey_NilResolved(t *testing.T) {
	service := &Service{}
	_, err := service.GetInstanceKey(nil)
	if err == nil {
		t.Fatal("Expected error for nil resolved model")
	}
}

func TestService_ValidateResolvedModel_Success(t *testing.T) {
	service := &Service{}
	resolved := &model.ResolvedModel{
		Model:           "gpt-4",
		MaxInputTokens:  8000,
		MaxOutputTokens: 2000,
	}

	err := service.validateResolvedModel(resolved)
	if err != nil {
		t.Errorf("validateResolvedModel returned error: %v", err)
	}
}

func TestService_ValidateResolvedModel_NilModel(t *testing.T) {
	service := &Service{}
	err := service.validateResolvedModel(nil)
	if err == nil {
		t.Fatal("Expected error for nil model")
	}
}

func TestService_ValidateResolvedModel_OutputTokensTooLarge(t *testing.T) {
	service := &Service{}
	resolved := &model.ResolvedModel{
		Model:           "gpt-4",
		MaxInputTokens:  8000,
		MaxOutputTokens: 300000,
	}

	err := service.validateResolvedModel(resolved)
	if err == nil {
		t.Fatal("Expected error for output tokens too large")
	}
}

func TestService_ValidateResolvedModel_InputTokensTooLarge(t *testing.T) {
	service := &Service{}
	resolved := &model.ResolvedModel{
		Model:           "gpt-4",
		MaxInputTokens:  3000000,
		MaxOutputTokens: 2000,
	}

	err := service.validateResolvedModel(resolved)
	if err == nil {
		t.Fatal("Expected error for input tokens too large")
	}
}

func TestService_ValidateResolvedModel_EmptyModelName(t *testing.T) {
	service := &Service{}
	resolved := &model.ResolvedModel{
		Model:           "",
		MaxInputTokens:  8000,
		MaxOutputTokens: 2000,
	}

	err := service.validateResolvedModel(resolved)
	if err == nil {
		t.Fatal("Expected error for empty model name")
	}
}

func TestService_ResolveModelInternal_WithDefaults(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"test-model": {
				Provider: "openai",
				// Model not specified - should use provider default
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.ProviderDefinition{
			"openai": {
				Type:            "openai",
				DefaultModel:    "gpt-3.5-turbo",
				MaxInputTokens:  4000,
				MaxOutputTokens: 2000,
				DefaultBaseURL:  "https://api.openai.com/v1",
				DefaultEnvVar:   "OPENAI_API_KEY",
			},
		},
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved, err := service.Get("test-model")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	// Should use provider defaults
	if resolved.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected default model 'gpt-3.5-turbo', got '%s'", resolved.Model)
	}
	if resolved.MaxInputTokens != 4000 {
		t.Errorf("Expected default max input tokens 4000, got %d", resolved.MaxInputTokens)
	}
}

func TestService_ResolveModelInternal_WithRateLimit(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"test-model": {
				Provider:        "openai",
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				RateLimit: &config.ModelRateLimitConfig{
					RequestsPerMinute: 10,
					TokensPerMinute:   100000,
					RequestsPerDay:    1000,
				},
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.ProviderDefinition{
			"openai": {
				Type:            "openai",
				DefaultModel:    "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
			},
		},
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved, err := service.Get("test-model")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if resolved.RateLimit.RequestsPerMinute != 10 {
		t.Errorf("Expected rate limit 10 requests per minute, got %d", resolved.RateLimit.RequestsPerMinute)
	}
	if resolved.RateLimit.TokensPerMinute != 100000 {
		t.Errorf("Expected rate limit 100000 tokens per minute, got %d", resolved.RateLimit.TokensPerMinute)
	}
}

func TestService_ResolveModelInternal_UnknownProvider(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"test-model": {
				Provider: "unknown-provider",
				Model:    "some-model",
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.ProviderDefinition),
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get("test-model")
	if err == nil {
		t.Fatal("Expected error for unknown provider")
	}
}

func TestService_ResolveModelInternal_NilModels(t *testing.T) {
	cfg := &config.Config{
		Models: nil,
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.ProviderDefinition),
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = service.Get("test-model")
	if err == nil {
		t.Fatal("Expected error for nil models config")
	}
}

func TestService_ResolveModelInternal_APIKeyFromEnv(t *testing.T) {
	os.Setenv("CUSTOM_API_KEY", "my-secret-key")
	defer os.Unsetenv("CUSTOM_API_KEY")

	cfg := &config.Config{
		Models: map[string]*config.ModelDefinition{
			"test-model": {
				Provider:        "openai",
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				APIKeyEnv:       "CUSTOM_API_KEY",
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.ProviderDefinition{
			"openai": {
				Type:            "openai",
				DefaultModel:    "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
			},
		},
	}

	service, err := NewService(configResolver, providerResolver)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved, err := service.Get("test-model")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if resolved.APIKey != "my-secret-key" {
		t.Errorf("Expected API key 'my-secret-key', got '%s'", resolved.APIKey)
	}
	if resolved.APIKeyEnv != "CUSTOM_API_KEY" {
		t.Errorf("Expected API key env 'CUSTOM_API_KEY', got '%s'", resolved.APIKeyEnv)
	}
}
