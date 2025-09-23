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

package providers

import (
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/models/config"
	gatewaymodels "github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/services/llm/registry"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Errorf("NewService() returned nil")
	}

	// Verify interface compliance
	var _ Service = service
}

func TestServiceImpl_RegisterProvider(t *testing.T) {
	service := NewService().(*serviceImpl)

	definition := config.ProviderDefinition{
		Type:            gatewaymodels.OpenAI,
		Name:            "test-provider",
		DefaultModel:    "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		DefaultTimeout:  5 * time.Minute,
		TokenizerType:   registry.TokenizerCL100K,
	}

	err := service.RegisterProvider("test", definition)
	if err != nil {
		t.Errorf("RegisterProvider() error = %v", err)
		return
	}

	// Verify it was registered
	if !service.HasProvider("test") {
		t.Errorf("RegisterProvider() failed to register provider")
	}
}

func TestServiceImpl_RegisterProvider_EmptyName(t *testing.T) {
	service := NewService()

	definition := config.ProviderDefinition{
		Type:         gatewaymodels.OpenAI,
		Name:         "test-provider",
		DefaultModel: "gpt-4",
	}

	err := service.RegisterProvider("", definition)
	if err == nil {
		t.Errorf("RegisterProvider() with empty name should return error")
	}
}

func TestServiceImpl_GetProvider(t *testing.T) {
	service := NewService().(*serviceImpl)

	definition := config.ProviderDefinition{
		Type:            gatewaymodels.OpenAI,
		Name:            "test-provider",
		DefaultModel:    "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		DefaultTimeout:  5 * time.Minute,
		TokenizerType:   registry.TokenizerCL100K,
	}

	// Register provider
	err := service.RegisterProvider("test", definition)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Get provider
	result, err := service.GetProvider("test")
	if err != nil {
		t.Errorf("GetProvider() error = %v", err)
		return
	}

	if result.Name != "test-provider" {
		t.Errorf("GetProvider() name = %s, want test-provider", result.Name)
	}
}

func TestServiceImpl_GetProvider_NotFound(t *testing.T) {
	service := NewService()

	_, err := service.GetProvider("nonexistent")
	if err == nil {
		t.Errorf("GetProvider() for nonexistent provider should return error")
	}
}

func TestServiceImpl_HasProvider(t *testing.T) {
	service := NewService().(*serviceImpl)

	definition := config.ProviderDefinition{
		Type:         gatewaymodels.OpenAI,
		Name:         "test-provider",
		DefaultModel: "gpt-4",
	}

	// Initially should not have provider
	if service.HasProvider("test") {
		t.Errorf("HasProvider() should return false for unregistered provider")
	}

	// Register provider
	err := service.RegisterProvider("test", definition)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Now should have provider
	if !service.HasProvider("test") {
		t.Errorf("HasProvider() should return true for registered provider")
	}
}

func TestServiceImpl_ListProviders(t *testing.T) {
	service := NewService().(*serviceImpl)

	// Register a few providers
	definition1 := config.ProviderDefinition{
		Type:         gatewaymodels.OpenAI,
		Name:         "openai-provider",
		DefaultModel: "gpt-4",
	}

	definition2 := config.ProviderDefinition{
		Type:         gatewaymodels.Anthropic,
		Name:         "anthropic-provider",
		DefaultModel: "claude-3",
	}

	err := service.RegisterProvider("openai", definition1)
	if err != nil {
		t.Fatalf("Failed to register openai provider: %v", err)
	}

	err = service.RegisterProvider("anthropic", definition2)
	if err != nil {
		t.Fatalf("Failed to register anthropic provider: %v", err)
	}

	providers := service.ListProviders()

	// Should contain at least our registered providers
	foundOpenAI := false
	foundAnthropic := false
	for _, p := range providers {
		if p == "openai" {
			foundOpenAI = true
		}
		if p == "anthropic" {
			foundAnthropic = true
		}
	}

	if !foundOpenAI {
		t.Errorf("ListProviders() should include 'openai'")
	}
	if !foundAnthropic {
		t.Errorf("ListProviders() should include 'anthropic'")
	}
}

func TestServiceImpl_GetDefaultProfile(t *testing.T) {
	service := NewService()

	// Test with a known provider type
	profile := service.GetDefaultProfile(gatewaymodels.OpenAI)

	if profile.Provider == "" {
		t.Errorf("GetDefaultProfile() should return a profile with provider set")
	}

	if profile.MaxInputTokens == 0 {
		t.Errorf("GetDefaultProfile() should return a profile with MaxInputTokens set")
	}
}

func TestServiceImpl_GetDefaultProfile_UnknownProvider(t *testing.T) {
	service := NewService()

	// Test with an unknown provider type
	profile := service.GetDefaultProfile(gatewaymodels.Provider("unknown"))

	// Should return fallback defaults
	if profile.MaxInputTokens != 128000 {
		t.Errorf("GetDefaultProfile() for unknown provider MaxInputTokens = %d, want 128000", profile.MaxInputTokens)
	}

	if profile.MaxOutputTokens != 4096 {
		t.Errorf("GetDefaultProfile() for unknown provider MaxOutputTokens = %d, want 4096", profile.MaxOutputTokens)
	}
}
