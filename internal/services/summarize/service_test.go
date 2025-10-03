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

package summarize

import (
	"testing"

	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/profile"
)

// Mock implementations for testing

type mockConfigService struct {
	config *config.Config
}

func (m *mockConfigService) GetConfig() *config.Config {
	return m.config
}

type mockProfileService struct {
	profiles map[profile.Profile]*profile.ResolvedProfile
	err      error
}

func (m *mockProfileService) Get(p profile.Profile) (*profile.ResolvedProfile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.profiles[p], nil
}

func TestNewService(t *testing.T) {
	configSvc := &mockConfigService{}
	profileSvc := &mockProfileService{}
	svc := NewService(configSvc, profileSvc)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestGetSummarizationConfig_NoSummarizeConfig(t *testing.T) {
	configSvc := &mockConfigService{
		config: &config.Config{},
	}
	profileSvc := &mockProfileService{}
	svc := NewService(configSvc, profileSvc)

	result, err := svc.GetSummarizationConfig("test.go")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Fatalf("Expected nil result, got %v", result)
	}
}

func TestGetSummarizationConfig_SkipRule(t *testing.T) {
	configSvc := &mockConfigService{
		config: &config.Config{
			Summarize: &config.SummarizeConfig{
				Rules: []*config.SummarizeRule{
					{
						Match: "*.go",
						Skip:  true,
					},
				},
			},
		},
	}
	profileSvc := &mockProfileService{}
	svc := NewService(configSvc, profileSvc)

	result, err := svc.GetSummarizationConfig("test.go")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if !result.Skip {
		t.Fatal("Expected Skip to be true")
	}
}

func TestGetSummarizationConfig_WithDefaults(t *testing.T) {
	configSvc := &mockConfigService{
		config: &config.Config{
			Summarize: &config.SummarizeConfig{
				Default: &config.SummarizeDefault{
					Profile:      "default",
					SystemPrompt: "Default prompt",
					Strategy: &config.Strategy{
						Type:                "plain",
						IncludeOriginalFile: true,
						IncludeChangedFile:  false,
					},
				},
			},
		},
	}
	profileSvc := &mockProfileService{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"default": {
				Name:     "default",
				Provider: "openai",
				Model:    "gpt-4",
			},
		},
	}
	svc := NewService(configSvc, profileSvc)

	result, err := svc.GetSummarizationConfig("test.txt")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Skip {
		t.Fatal("Expected Skip to be false")
	}
	if result.Profile.Name != "default" {
		t.Errorf("Expected profile name 'default', got '%s'", result.Profile.Name)
	}
	if result.SystemPrompt != "Default prompt" {
		t.Errorf("Expected system prompt 'Default prompt', got '%s'", result.SystemPrompt)
	}
	if !result.IncludeOriginalFile {
		t.Fatal("Expected IncludeOriginalFile to be true")
	}
	if result.IncludeChangedFile {
		t.Fatal("Expected IncludeChangedFile to be false")
	}
}

func TestGetSummarizationConfig_WithRuleOverride(t *testing.T) {
	configSvc := &mockConfigService{
		config: &config.Config{
			Summarize: &config.SummarizeConfig{
				Rules: []*config.SummarizeRule{
					{
						Match:        "*.go",
						Profile:      "golang",
						SystemPrompt: "Go specific prompt",
						Strategy: &config.Strategy{
							Type:                "diff",
							IncludeOriginalFile: false,
							IncludeChangedFile:  true,
						},
					},
				},
				Default: &config.SummarizeDefault{
					Profile:      "default",
					SystemPrompt: "Default prompt",
					Strategy: &config.Strategy{
						Type:                "plain",
						IncludeOriginalFile: true,
						IncludeChangedFile:  false,
					},
				},
			},
		},
	}
	profileSvc := &mockProfileService{
		profiles: map[profile.Profile]*profile.ResolvedProfile{
			"golang": {
				Name:     "golang",
				Provider: "anthropic",
				Model:    "claude-3",
			},
		},
	}
	svc := NewService(configSvc, profileSvc)

	result, err := svc.GetSummarizationConfig("main.go")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Profile.Name != "golang" {
		t.Errorf("Expected profile name 'golang', got '%s'", result.Profile.Name)
	}
	if result.SystemPrompt != "Go specific prompt" {
		t.Errorf("Expected system prompt 'Go specific prompt', got '%s'", result.SystemPrompt)
	}
	if result.IncludeOriginalFile {
		t.Fatal("Expected IncludeOriginalFile to be false")
	}
	if !result.IncludeChangedFile {
		t.Fatal("Expected IncludeChangedFile to be true")
	}
}

func TestGetSummarizationConfig_ProfileError(t *testing.T) {
	configSvc := &mockConfigService{
		config: &config.Config{
			Summarize: &config.SummarizeConfig{
				Default: &config.SummarizeDefault{
					Profile: "nonexistent",
				},
			},
		},
	}
	profileSvc := &mockProfileService{
		err: profile.ErrProfileNotFound,
	}
	svc := NewService(configSvc, profileSvc)

	_, err := svc.GetSummarizationConfig("test.txt")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
