// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"testing"
	"time"

	provider2 "github.com/retran/meowg1k/internal/domain/provider"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("NewService should not return nil")
	}
}

func TestGetValidProviders(t *testing.T) {
	service := NewService()

	// Test all valid providers
	validProviders := []provider2.Provider{
		provider2.Gemini,
		provider2.OpenAI,
		provider2.Anthropic,
		provider2.Llama,
		provider2.OpenRouter,
		provider2.Voyage,
		provider2.OpenAICompatible,
	}

	for _, providerType := range validProviders {
		t.Run(string(providerType), func(t *testing.T) {
			provider, err := service.Get(providerType)
			if err != nil {
				t.Errorf("Expected no error for provider %s, got: %v", providerType, err)
			}

			if provider.Type != providerType {
				t.Errorf("Expected provider type %s, got %s", providerType, provider.Type)
			}

			// Check that name is not empty
			if provider.Name == "" {
				t.Errorf("Provider %s should have a name", providerType)
			}

			// Check timeout is reasonable
			if provider.DefaultTimeout <= 0 {
				t.Errorf("Provider %s should have positive timeout", providerType)
			}
		})
	}
}

func TestGetInvalidProvider(t *testing.T) {
	service := NewService()

	// Test with invalid provider
	invalidProvider := provider2.Provider("invalid-provider")
	_, err := service.Get(invalidProvider)
	if err == nil {
		t.Error("Expected error for invalid provider")
	}

	expectedErrorMsg := "provider not found: invalid-provider"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

func TestGeminiProviderConfiguration(t *testing.T) {
	service := NewService()
	p, err := service.Get(provider2.Gemini)
	if err != nil {
		t.Fatalf("Failed to get Gemini provider: %v", err)
	}

	if p.Name != "Google Gemini" {
		t.Errorf("Expected name 'Google Gemini', got '%s'", p.Name)
	}

	if p.DefaultModel != "gemini-2.5-flash" {
		t.Errorf("Expected default model 'gemini-2.5-flash', got '%s'", p.DefaultModel)
	}

	if p.DefaultEnvVar != "MEOW_GEMINI_API_KEY" {
		t.Errorf("Expected env var 'MEOW_GEMINI_API_KEY', got '%s'", p.DefaultEnvVar)
	}

	if !p.RequiresAPIKey {
		t.Error("Gemini should require API key")
	}

	if p.RequiresBaseURL {
		t.Error("Gemini should not require base URL")
	}

	if p.MaxInputTokens != 1000000 {
		t.Errorf("Expected max input tokens 1000000, got %d", p.MaxInputTokens)
	}

	if p.DefaultTimeout != 5*time.Minute {
		t.Errorf("Expected timeout 5m, got %v", p.DefaultTimeout)
	}
}

func TestOpenAIProviderConfiguration(t *testing.T) {
	service := NewService()
	provider, err := service.Get(provider2.OpenAI)
	if err != nil {
		t.Fatalf("Failed to get OpenAI provider: %v", err)
	}

	if provider.Name != "OpenAI" {
		t.Errorf("Expected name 'OpenAI', got '%s'", provider.Name)
	}

	if provider.DefaultModel != "gpt-4o-mini" {
		t.Errorf("Expected default model 'gpt-4o-mini', got '%s'", provider.DefaultModel)
	}

	if provider.DefaultBaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected base URL 'https://api.openai.com/v1', got '%s'", provider.DefaultBaseURL)
	}

	if !provider.RequiresAPIKey {
		t.Error("OpenAI should require API key")
	}

	if provider.RequiresBaseURL {
		t.Error("OpenAI should not require base URL (has default)")
	}
}

func TestAnthropicProviderConfiguration(t *testing.T) {
	service := NewService()
	provider, err := service.Get(provider2.Anthropic)
	if err != nil {
		t.Fatalf("Failed to get Anthropic provider: %v", err)
	}

	if provider.Name != "Anthropic Claude" {
		t.Errorf("Expected name 'Anthropic Claude', got '%s'", provider.Name)
	}

	if provider.DefaultModel != "claude-3-5-haiku-20241022" {
		t.Errorf("Expected default model 'claude-3-5-haiku-20241022', got '%s'", provider.DefaultModel)
	}

	if provider.MaxInputTokens != 200000 {
		t.Errorf("Expected max input tokens 200000, got %d", provider.MaxInputTokens)
	}
}

func TestLlamaProviderConfiguration(t *testing.T) {
	service := NewService()
	provider, err := service.Get(provider2.Llama)
	if err != nil {
		t.Fatalf("Failed to get Llama provider: %v", err)
	}

	if provider.Name != "Meta Llama" {
		t.Errorf("Expected name 'Meta Llama', got '%s'", provider.Name)
	}

	if provider.DefaultModel != "llama3.2:3b" {
		t.Errorf("Expected default model 'llama3.2:3b', got '%s'", provider.DefaultModel)
	}

	if provider.RequiresAPIKey {
		t.Error("Llama should not require API key")
	}

	if !provider.RequiresBaseURL {
		t.Error("Llama should require base URL")
	}

	if provider.DefaultTimeout != 10*time.Minute {
		t.Errorf("Expected timeout 10m, got %v", provider.DefaultTimeout)
	}
}

func TestVoyageProviderConfiguration(t *testing.T) {
	service := NewService()
	provider, err := service.Get(provider2.Voyage)
	if err != nil {
		t.Fatalf("Failed to get Voyage provider: %v", err)
	}

	if provider.Name != "Voyage AI" {
		t.Errorf("Expected name 'Voyage AI', got '%s'", provider.Name)
	}

	if provider.DefaultModel != "voyage-3" {
		t.Errorf("Expected default model 'voyage-3', got '%s'", provider.DefaultModel)
	}

	if provider.MaxOutputTokens != 0 {
		t.Errorf("Expected max output tokens 0 (embeddings), got %d", provider.MaxOutputTokens)
	}
}

func TestOpenAICompatibleProviderConfiguration(t *testing.T) {
	service := NewService()
	provider, err := service.Get(provider2.OpenAICompatible)
	if err != nil {
		t.Fatalf("Failed to get OpenAI Compatible provider: %v", err)
	}

	if provider.Name != "OpenAI Compatible" {
		t.Errorf("Expected name 'OpenAI Compatible', got '%s'", provider.Name)
	}

	if provider.DefaultModel != "" {
		t.Errorf("Expected empty default model (must be user specified), got '%s'", provider.DefaultModel)
	}

	if !provider.RequiresBaseURL {
		t.Error("OpenAI Compatible should require base URL")
	}

	if provider.RequiresAPIKey {
		t.Error("OpenAI Compatible should not require API key by default")
	}
}

func TestGetWithNilService(t *testing.T) {
	var service *Service
	_, err := service.Get(provider2.OpenAI)
	if err == nil {
		t.Error("Expected error when service is nil")
	}

	expectedErrorMsg := "provider service is nil"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

func TestOpenRouterProviderConfiguration(t *testing.T) {
	service := NewService()
	provider, err := service.Get(provider2.OpenRouter)
	if err != nil {
		t.Fatalf("Failed to get OpenRouter provider: %v", err)
	}

	if provider.Name != "OpenRouter" {
		t.Errorf("Expected name 'OpenRouter', got '%s'", provider.Name)
	}

	if provider.DefaultModel != "anthropic/claude-3.5-haiku" {
		t.Errorf("Expected default model 'anthropic/claude-3.5-haiku', got '%s'", provider.DefaultModel)
	}

	if provider.DefaultBaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected base URL 'https://openrouter.ai/api/v1', got '%s'", provider.DefaultBaseURL)
	}

	if !provider.RequiresAPIKey {
		t.Error("OpenRouter should require API key")
	}

	if provider.RequiresBaseURL {
		t.Error("OpenRouter should not require base URL (has default)")
	}
}
