// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package pullrequest

import (
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
)

// mockConfigResolver is a mock implementation of ConfigReader for testing.
type mockConfigResolver struct {
	Cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.Cfg, nil
}

// mockPresetResolver is a mock implementation of PresetResolver for testing.
type mockPresetResolver struct {
	Preset *preset.ResolvedPreset
	Err    error
}

func (m *mockPresetResolver) Get(p preset.Preset) (*preset.ResolvedPreset, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Preset, nil
}

func TestNewService(t *testing.T) {
	configSvc := &mockConfigResolver{}
	presetSvc := &mockPresetResolver{}
	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Errorf("NewService returned error: %v", err)
	}

	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetPRConfig(t *testing.T) {
	resolvedPreset := &preset.ResolvedPreset{
		Model: "gpt-4",
	}

	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{
					Pr: &config.CommandFlowConfig{
						Preset:       "test",
						SystemPrompt: "Test PR prompt",
					},
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{
		Preset: resolvedPreset,
	}

	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Errorf("NewService returned error: %v", err)
	}

	result, err := service.Get()
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if result.Preset != resolvedPreset {
		t.Error("Preset not set correctly")
	}

	if result.SystemPrompt != "Test PR prompt" {
		t.Errorf("Expected 'Test PR prompt', got '%s'", result.SystemPrompt)
	}
}

func TestGetPRConfigDefault(t *testing.T) {
	resolvedPreset := &preset.ResolvedPreset{
		Model: "gpt-4",
	}

	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{
					Pr: nil,
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{
		Preset: resolvedPreset,
	}

	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Errorf("NewService returned error: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when PR config is nil, got nil")
	}
}

func TestGetPRConfigPresetError(t *testing.T) {
	configSvc := &mockConfigResolver{
		Cfg: &config.Config{},
	}
	mockErr := fmt.Errorf("preset not found in configuration")
	presetSvc := &mockPresetResolver{
		Err: mockErr,
	}

	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Errorf("NewService returned error: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected preset error, got nil")
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
	_, err := service.Get()
	if err == nil {
		t.Error("Expected error when service is nil")
	}
}

func TestGetWithConfigError(t *testing.T) {
	configSvc := &mockConfigResolverWithError{}
	presetSvc := &mockPresetResolver{
		Preset: &preset.ResolvedPreset{
			Provider: "openai",
			Model:    "gpt-4",
		},
	}

	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when config resolver returns error")
	}
}

func TestGetWithEmptySystemPrompt(t *testing.T) {
	resolvedPreset := &preset.ResolvedPreset{
		Provider: "openai",
		Model:    "gpt-4",
	}

	configSvc := &mockConfigResolver{
		Cfg: &config.Config{
			Flows: &config.FlowsConfig{
				Draft: &config.DraftFlowConfig{
					Pr: &config.CommandFlowConfig{
						Preset:       "test",
						SystemPrompt: "", // Empty system prompt
					},
				},
			},
		},
	}
	presetSvc := &mockPresetResolver{
		Preset: resolvedPreset,
	}

	service, err := NewService(configSvc, presetSvc)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Error("Expected error when system prompt is empty")
	}
}

type mockConfigResolverWithError struct{}

func (m *mockConfigResolverWithError) Get() (*config.Config, error) {
	return nil, fmt.Errorf("config error")
}
