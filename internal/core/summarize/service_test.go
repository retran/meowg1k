// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package summarize

import (
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
)

// Mock implementations for testing

// mockConfigResolver is a mock implementation of ConfigResolver for testing.
type mockConfigResolver struct {
	Cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.Cfg, nil
}

// mockPresetResolver is a mock implementation of PresetResolver for testing.
type mockPresetResolver struct {
	presets map[preset.Preset]*preset.ResolvedPreset
	err     error
}

func (m *mockPresetResolver) Get(p preset.Preset) (*preset.ResolvedPreset, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.presets[p], nil
}

func TestNewService(t *testing.T) {
	configSvc := &mockConfigResolver{}
	presetSvc := &mockPresetResolver{}
	svc, err := NewService(configSvc, presetSvc)
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
	presetSvc := &mockPresetResolver{}
	svc, err := NewService(configSvc, presetSvc)
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
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{},
			},
			Activities: &config.ActivitiesConfig{
				Summarize: &config.SummarizeActivityConfig{
					Rules: []*config.SummarizeRule{
						{
							Match: "*.go",
							Skip:  true,
						},
					},
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{}
	svc, err := NewService(configSvc, presetSvc)
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
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{},
			},
			Activities: &config.ActivitiesConfig{
				Summarize: &config.SummarizeActivityConfig{
					Preset:       "default",
					SystemPrompt: "Default prompt",
					Strategy: &config.StrategyConfig{
						Type:                "plain",
						IncludeOriginalFile: true,
						IncludeChangedFile:  false,
					},
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"default": {
				Name:  "default",
				Model: "gpt-4",
			},
		},
	}
	svc, err := NewService(configSvc, presetSvc)
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
	if result.Preset.Name != "default" {
		t.Errorf("Expected preset name 'default', got '%s'", result.Preset.Name)
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
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{},
			},
			Activities: &config.ActivitiesConfig{
				Summarize: &config.SummarizeActivityConfig{
					Rules: []*config.SummarizeRule{
						{
							Match:        "*.go",
							Preset:       "golang",
							SystemPrompt: "Go specific prompt",
							Strategy: &config.StrategyConfig{
								Type:                "diff",
								IncludeOriginalFile: false,
								IncludeChangedFile:  true,
							},
						},
					},
					Preset:       "default",
					SystemPrompt: "Default prompt",
					Strategy: &config.StrategyConfig{
						Type:                "plain",
						IncludeOriginalFile: true,
						IncludeChangedFile:  false,
					},
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"golang": {
				Name:  "golang",
				Model: "claude-3",
			},
		},
	}
	svc, err := NewService(configSvc, presetSvc)
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
	if result.Preset.Name != "golang" {
		t.Errorf("Expected preset name 'golang', got '%s'", result.Preset.Name)
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

func TestGetSummarizationConfig_PresetError(t *testing.T) {
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{},
			},
			Activities: &config.ActivitiesConfig{
				Summarize: &config.SummarizeActivityConfig{
					Preset: "nonexistent",
				},
			},
		},
	}
	mockErr := fmt.Errorf("preset not found in configuration")
	presetSvc := &mockPresetResolver{
		err: mockErr,
	}
	svc, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = svc.Get("test.txt")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestNewServiceWithNilConfigResolver(t *testing.T) {
	presetSvc := &mockPresetResolver{}
	service, err := NewService(nil, presetSvc)
	if err == nil {
		t.Error("Expected error when config resolver is nil")
	}
	if service != nil {
		t.Error("Expected nil service when config resolver is nil")
	}
}

func TestNewServiceWithNilPresetResolver(t *testing.T) {
	configSvc := &mockConfigResolver{}
	service, err := NewService(configSvc, nil)
	if err == nil {
		t.Error("Expected error when preset resolver is nil")
	}
	if service != nil {
		t.Error("Expected nil service when preset resolver is nil")
	}
}

func TestGetWithNilService(t *testing.T) {
	var service *Service
	_, err := service.Get("test.txt")
	if err == nil {
		t.Error("Expected error when service is nil")
	}
}

func TestGetWithConfigError(t *testing.T) {
	configSvc := &mockConfigResolverWithError{}
	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"default": {
				Name:  "default",
				Model: "gpt-4",
			},
		},
	}

	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get("test.txt")
	if err == nil {
		t.Error("Expected error when config resolver returns error")
	}
}

func TestGetWithDefaultStrategy(t *testing.T) {
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{},
			},
			Activities: &config.ActivitiesConfig{
				Summarize: &config.SummarizeActivityConfig{
					Preset:       "default",
					SystemPrompt: "Default prompt",
					Strategy:     nil, // No strategy specified
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{
		presets: map[preset.Preset]*preset.ResolvedPreset{
			"default": {
				Name:  "default",
				Model: "gpt-4",
			},
		},
	}
	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	result, err := service.Get("test.txt")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Should use default strategy
	if result.Strategy == nil {
		t.Fatal("Expected strategy to be set")
	}

	if result.Strategy.Type != "plain" {
		t.Errorf("Expected strategy type 'plain', got '%s'", result.Strategy.Type)
	}

	if result.IncludeOriginalFile {
		t.Error("Expected IncludeOriginalFile to be false by default")
	}

	if result.IncludeChangedFile {
		t.Error("Expected IncludeChangedFile to be false by default")
	}
}

type mockConfigResolverWithError struct{}

func (m *mockConfigResolverWithError) Get() (*config.Config, error) {
	return nil, fmt.Errorf("config error")
}
