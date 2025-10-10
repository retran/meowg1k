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
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	coreProfile "github.com/retran/meowg1k/internal/domain/profile"
)

// Mock implementations for testing

// mockConfigResolver is a mock implementation of ConfigResolver for testing.
type mockConfigResolver struct {
	Cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.Cfg, nil
}

// mockProfileResolver is a mock implementation of ProfileResolver for testing.
type mockProfileResolver struct {
	profiles map[coreProfile.Profile]*coreProfile.ResolvedProfile
	err      error
}

func (m *mockProfileResolver) Get(p coreProfile.Profile) (*coreProfile.ResolvedProfile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.profiles[p], nil
}

func TestNewService(t *testing.T) {
	configSvc := &mockConfigResolver{}
	profileSvc := &mockProfileResolver{}
	svc, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestGetSummarizationConfig_NoSummarizeConfig(t *testing.T) {
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{},
	}
	profileSvc := &mockProfileResolver{}
	svc, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	result, err := svc.Get("test.go")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Fatalf("Expected nil result, got %v", result)
	}
}

func TestGetSummarizationConfig_SkipRule(t *testing.T) {
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
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
	profileSvc := &mockProfileResolver{}
	svc, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	result, err := svc.Get("test.go")
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
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
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
	profileSvc := &mockProfileResolver{
		profiles: map[coreProfile.Profile]*coreProfile.ResolvedProfile{
			"default": {
				Name:  "default",
				Model: "gpt-4",
			},
		},
	}
	svc, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	result, err := svc.Get("test.txt")
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
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
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
	profileSvc := &mockProfileResolver{
		profiles: map[coreProfile.Profile]*coreProfile.ResolvedProfile{
			"golang": {
				Name:  "golang",
				Model: "claude-3",
			},
		},
	}
	svc, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	result, err := svc.Get("main.go")
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
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Summarize: &config.SummarizeConfig{
				Default: &config.SummarizeDefault{
					Profile: "nonexistent",
				},
			},
		},
	}
	mockErr := fmt.Errorf("profile not found in configuration")
	profileSvc := &mockProfileResolver{
		err: mockErr,
	}
	svc, err := NewService(configSvc, profileSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = svc.Get("test.txt")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
