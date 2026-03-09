// Copyright © 2025 The meowg1k Authors.
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
	providers map[provider.Provider]provider.Definition
	err       error
}

func (m *mockServiceProviderResolver) Get(p provider.Provider) (provider.Definition, error) {
	if m.err != nil {
		return provider.Definition{}, m.err
	}
	if def, ok := m.providers[p]; ok {
		return def, nil
	}
	return provider.Definition{}, fmt.Errorf("provider not found: %s", p)
}

func TestNewService_Success(t *testing.T) {
	configResolver := &mockServiceConfigResolver{
		config: &config.Config{},
	}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.Definition),
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
		Models: map[string]*config.ModelConfig{
			"test-model": {
				Provider: "openai",
				Model:    "gpt-4",
				BaseURL:  "https://api.openai.com/v1",
				APIKey:   "test-key",
				Limits: &config.ModelLimits{
					MaxInputTokens:  8000,
					MaxOutputTokens: 2000,
				},
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.Definition{
			"openai": {
				Type:            "openai",
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
		Models: map[string]*config.ModelConfig{
			"test-model": {
				Provider: "openai",
				Model:    "gpt-4",
				Limits: &config.ModelLimits{
					MaxInputTokens:  8000,
					MaxOutputTokens: 2000,
				},
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.Definition{
			"openai": {
				Type:            "openai",
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
		Models: map[string]*config.ModelConfig{},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.Definition),
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

func TestService_ResolveModelInternal_ModelNameRequired(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelConfig{
			"test-model": {
				Provider: "openai",
				Model:    "",
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.Definition{
			"openai": {
				Type:            "openai",
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

	_, err = service.Get("test-model")
	if err == nil {
		t.Fatal("Expected error for missing model name")
	}
}

func TestService_ResolveModelInternal_WithRateLimit(t *testing.T) {
	cfg := &config.Config{
		Models: map[string]*config.ModelConfig{
			"test-model": {
				Provider: "openai",
				Model:    "gpt-4",
				Limits: &config.ModelLimits{
					MaxInputTokens:  8000,
					MaxOutputTokens: 2000,
				},
				RateLimit: &config.RateLimitConfig{
					RequestsPerMinute: 10,
					TokensPerMinute:   100000,
					RequestsPerDay:    1000,
				},
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: map[provider.Provider]provider.Definition{
			"openai": {
				Type:            "openai",
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
		Models: map[string]*config.ModelConfig{
			"test-model": {
				Provider: "unknown-provider",
				Model:    "some-model",
			},
		},
	}

	configResolver := &mockServiceConfigResolver{config: cfg}
	providerResolver := &mockServiceProviderResolver{
		providers: make(map[provider.Provider]provider.Definition),
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
		providers: make(map[provider.Provider]provider.Definition),
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

// Test mergeProviderConfig.

func TestService_MergeProviderConfig_NilProvider(t *testing.T) {
	service, err := NewService(
		&mockServiceConfigResolver{config: &config.Config{}},
		&mockServiceProviderResolver{providers: make(map[provider.Provider]provider.Definition)},
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved := &model.ResolvedModel{}
	service.mergeProviderConfig(resolved, nil)
	// Should not panic, resolved should be unchanged
}

func TestService_MergeProviderConfig_WithBaseURL(t *testing.T) {
	service, err := NewService(
		&mockServiceConfigResolver{config: &config.Config{}},
		&mockServiceProviderResolver{providers: make(map[provider.Provider]provider.Definition)},
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved := &model.ResolvedModel{
		BaseURL: "", // Empty, should be filled from provider
	}

	providerCfg := &config.ProviderConfig{
		BaseURL: "https://api.example.com",
	}

	service.mergeProviderConfig(resolved, providerCfg)

	if resolved.BaseURL != "https://api.example.com" {
		t.Errorf("Expected BaseURL to be set, got %s", resolved.BaseURL)
	}
}

func TestService_MergeProviderConfig_WithTokenizer(t *testing.T) {
	service, err := NewService(
		&mockServiceConfigResolver{config: &config.Config{}},
		&mockServiceProviderResolver{providers: make(map[provider.Provider]provider.Definition)},
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved := &model.ResolvedModel{
		Tokenizer: "", // Empty, should be filled from provider
	}

	providerCfg := &config.ProviderConfig{
		Tokenizer: "cl100k",
	}

	service.mergeProviderConfig(resolved, providerCfg)

	if resolved.Tokenizer != "cl100k" {
		t.Errorf("Expected Tokenizer to be set, got %s", resolved.Tokenizer)
	}
}

func TestService_MergeProviderConfig_WithLimits(t *testing.T) {
	service, err := NewService(
		&mockServiceConfigResolver{config: &config.Config{}},
		&mockServiceProviderResolver{providers: make(map[provider.Provider]provider.Definition)},
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved := &model.ResolvedModel{}

	providerCfg := &config.ProviderConfig{
		Limits: &config.ModelLimits{
			MaxInputTokens:  100000,
			MaxOutputTokens: 4000,
		},
	}

	service.mergeProviderConfig(resolved, providerCfg)

	if resolved.MaxInputTokens != 100000 {
		t.Errorf("Expected MaxInputTokens 100000, got %d", resolved.MaxInputTokens)
	}
	if resolved.MaxOutputTokens != 4000 {
		t.Errorf("Expected MaxOutputTokens 4000, got %d", resolved.MaxOutputTokens)
	}
}

func TestService_MergeProviderConfig_WithRateLimit(t *testing.T) {
	service, err := NewService(
		&mockServiceConfigResolver{config: &config.Config{}},
		&mockServiceProviderResolver{providers: make(map[provider.Provider]provider.Definition)},
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved := &model.ResolvedModel{}

	providerCfg := &config.ProviderConfig{
		RateLimit: &config.RateLimitConfig{
			RequestsPerMinute: 10,
			TokensPerMinute:   100000,
			RequestsPerDay:    1000,
		},
	}

	service.mergeProviderConfig(resolved, providerCfg)

	if resolved.RateLimit.RequestsPerMinute != 10 {
		t.Errorf("Expected RequestsPerMinute 10, got %d", resolved.RateLimit.RequestsPerMinute)
	}
	if resolved.RateLimit.TokensPerMinute != 100000 {
		t.Errorf("Expected TokensPerMinute 100000, got %d", resolved.RateLimit.TokensPerMinute)
	}
	if resolved.RateLimit.RequestsPerDay != 1000 {
		t.Errorf("Expected RequestsPerDay 1000, got %d", resolved.RateLimit.RequestsPerDay)
	}
}

func TestService_MergeProviderConfig_DoesNotOverrideExisting(t *testing.T) {
	service, err := NewService(
		&mockServiceConfigResolver{config: &config.Config{}},
		&mockServiceProviderResolver{providers: make(map[provider.Provider]provider.Definition)},
	)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	resolved := &model.ResolvedModel{
		BaseURL:   "https://existing.com",
		Tokenizer: "existing-tokenizer",
	}

	providerCfg := &config.ProviderConfig{
		BaseURL:   "https://new.com",
		Tokenizer: "new-tokenizer",
	}

	service.mergeProviderConfig(resolved, providerCfg)

	// Should not override existing values
	if resolved.BaseURL != "https://existing.com" {
		t.Errorf("Expected BaseURL to remain unchanged, got %s", resolved.BaseURL)
	}
	if resolved.Tokenizer != "existing-tokenizer" {
		t.Errorf("Expected Tokenizer to remain unchanged, got %s", resolved.Tokenizer)
	}
}

// Test registry methods.

func TestRegistry_Get_WithNilRegistry(t *testing.T) {
	var reg *Registry
	info := reg.Get("test-model")
	if info.Provider != "unknown" {
		t.Errorf("Expected unknown provider for nil registry, got %s", info.Provider)
	}
}

func TestRegistry_ListKnownModels_WithNilRegistry(t *testing.T) {
	var reg *Registry
	models := reg.ListKnownModels()
	if models != nil {
		t.Error("Expected nil for nil registry")
	}
}

func TestRegistry_ListKnownModels_WithModels(t *testing.T) {
	reg := &Registry{
		models: map[string]model.Info{
			"gpt-4": {
				Provider:         "openai",
				MaxContextTokens: 8192,
				MaxOutputTokens:  4096,
			},
			"claude": {
				Provider:         "anthropic",
				MaxContextTokens: 100000,
				MaxOutputTokens:  4096,
			},
		},
	}

	modelList := reg.ListKnownModels()
	if len(modelList) != 2 {
		t.Errorf("Expected 2 models, got %d", len(modelList))
	}

	// Check that both models are present
	found := make(map[string]bool)
	for _, m := range modelList {
		found[m] = true
	}
	if !found["gpt-4"] || !found["claude"] {
		t.Error("Expected both gpt-4 and claude in list")
	}
}
