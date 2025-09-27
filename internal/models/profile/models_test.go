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
	"testing"
	"time"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	mdLLM "github.com/retran/meowg1k/internal/models/llm"
)

func TestProfile(t *testing.T) {
	// Test Profile type
	profile := Profile("test-profile")
	
	if string(profile) != "test-profile" {
		t.Errorf("Expected profile string 'test-profile', got '%s'", string(profile))
	}
}

func TestProfileComparison(t *testing.T) {
	profile1 := Profile("profile1")
	profile2 := Profile("profile2")
	profile3 := Profile("profile1")

	if profile1 == profile2 {
		t.Error("Different profiles should not be equal")
	}

	if profile1 != profile3 {
		t.Error("Same profiles should be equal")
	}
}

func TestResolvedProfile(t *testing.T) {
	// Test ResolvedProfile struct initialization and field access
	profile := ResolvedProfile{
		Provider:        mdGateway.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "sk-test123",
		TokenizerType:   mdLLM.TokenizerCL100K,
	}

	if profile.Provider != mdGateway.OpenAI {
		t.Errorf("Expected Provider %s, got %s", mdGateway.OpenAI, profile.Provider)
	}

	if profile.Model != "gpt-4" {
		t.Errorf("Expected Model 'gpt-4', got '%s'", profile.Model)
	}

	if profile.MaxInputTokens != 128000 {
		t.Errorf("Expected MaxInputTokens 128000, got %d", profile.MaxInputTokens)
	}

	if profile.MaxOutputTokens != 4096 {
		t.Errorf("Expected MaxOutputTokens 4096, got %d", profile.MaxOutputTokens)
	}

	if profile.Timeout != 5*time.Minute {
		t.Errorf("Expected Timeout 5m, got %v", profile.Timeout)
	}

	if profile.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected BaseURL 'https://api.openai.com/v1', got '%s'", profile.BaseURL)
	}

	if profile.APIKey != "sk-test123" {
		t.Errorf("Expected APIKey 'sk-test123', got '%s'", profile.APIKey)
	}

	if profile.TokenizerType != mdLLM.TokenizerCL100K {
		t.Errorf("Expected TokenizerType %s, got %s", mdLLM.TokenizerCL100K, profile.TokenizerType)
	}
}

func TestResolvedProfileZeroValues(t *testing.T) {
	// Test ResolvedProfile with zero values
	profile := ResolvedProfile{}

	if profile.Provider != "" {
		t.Errorf("Expected empty Provider, got '%s'", profile.Provider)
	}

	if profile.Model != "" {
		t.Errorf("Expected empty Model, got '%s'", profile.Model)
	}

	if profile.MaxInputTokens != 0 {
		t.Errorf("Expected MaxInputTokens 0, got %d", profile.MaxInputTokens)
	}

	if profile.MaxOutputTokens != 0 {
		t.Errorf("Expected MaxOutputTokens 0, got %d", profile.MaxOutputTokens)
	}

	if profile.Timeout != 0 {
		t.Errorf("Expected Timeout 0, got %v", profile.Timeout)
	}

	if profile.BaseURL != "" {
		t.Errorf("Expected empty BaseURL, got '%s'", profile.BaseURL)
	}

	if profile.APIKey != "" {
		t.Errorf("Expected empty APIKey, got '%s'", profile.APIKey)
	}

	if profile.TokenizerType != "" {
		t.Errorf("Expected empty TokenizerType, got %s", profile.TokenizerType)
	}
}

func TestResolvedProfileWithDifferentProviders(t *testing.T) {
	testCases := []struct {
		name     string
		profile  ResolvedProfile
		expected mdGateway.Provider
	}{
		{
			name: "OpenAI",
			profile: ResolvedProfile{
				Provider:      mdGateway.OpenAI,
				Model:         "gpt-3.5-turbo",
				TokenizerType: mdLLM.TokenizerCL100K,
			},
			expected: mdGateway.OpenAI,
		},
		{
			name: "Anthropic",
			profile: ResolvedProfile{
				Provider:      mdGateway.Anthropic,
				Model:         "claude-3-haiku",
				TokenizerType: mdLLM.TokenizerCL100K,
			},
			expected: mdGateway.Anthropic,
		},
		{
			name: "Gemini",
			profile: ResolvedProfile{
				Provider:      mdGateway.Gemini,
				Model:         "gemini-pro",
				TokenizerType: mdLLM.TokenizerGemini,
			},
			expected: mdGateway.Gemini,
		},
		{
			name: "Llama",
			profile: ResolvedProfile{
				Provider:      mdGateway.Llama,
				Model:         "llama3.2:3b",
				TokenizerType: mdLLM.TokenizerLlama,
			},
			expected: mdGateway.Llama,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.profile.Provider != tc.expected {
				t.Errorf("Expected provider %s, got %s", tc.expected, tc.profile.Provider)
			}
		})
	}
}

func TestProfileAsKey(t *testing.T) {
	// Test that Profile can be used as a map key
	profiles := map[Profile]string{
		"openai":    "OpenAI Profile",
		"anthropic": "Anthropic Profile",
		"gemini":    "Gemini Profile",
	}

	if profiles[Profile("openai")] != "OpenAI Profile" {
		t.Error("Profile should work as map key")
	}

	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles in map, got %d", len(profiles))
	}
}