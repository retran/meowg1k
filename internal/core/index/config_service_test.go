// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
)

// Mock implementations for testing

type mockConfigResolver struct {
	GetFunc func() (*config.Config, error)
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	if m.GetFunc != nil {
		return m.GetFunc()
	}
	return &config.Config{
		Flows: &config.FlowsConfig{
			Index: &config.IndexFlowConfig{
				Preset: "test-preset",
				Chunker: &config.ChunkerConfig{
					MaxRunes:     1000,
					OverlapRunes: 100,
				},
				BatchSize: 32,
			},
		},
	}, nil
}

type mockPresetResolver struct {
	GetFunc func(prof preset.Preset) (*preset.ResolvedPreset, error)
}

func (m *mockPresetResolver) Get(prof preset.Preset) (*preset.ResolvedPreset, error) {
	if m.GetFunc != nil {
		return m.GetFunc(prof)
	}
	return &preset.ResolvedPreset{
		Name: string(prof),
	}, nil
}

func TestNewConfigService(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		configResolver := &mockConfigResolver{}
		presetResolver := &mockPresetResolver{}

		service, err := NewConfigService(configResolver, presetResolver)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if service == nil {
			t.Fatal("Expected service to be non-nil")
		}
	})

	t.Run("Nil configResolver", func(t *testing.T) {
		presetResolver := &mockPresetResolver{}

		service, err := NewConfigService(nil, presetResolver)
		if err == nil {
			t.Fatal("Expected error for nil configResolver")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "config resolver is nil") {
			t.Errorf("Expected config resolver error, got: %v", err)
		}
	})

	t.Run("Nil presetResolver", func(t *testing.T) {
		configResolver := &mockConfigResolver{}

		service, err := NewConfigService(configResolver, nil)
		if err == nil {
			t.Fatal("Expected error for nil presetResolver")
		}
		if service != nil {
			t.Fatal("Expected service to be nil when error occurs")
		}
		if !strings.Contains(err.Error(), "preset resolver is nil") {
			t.Errorf("Expected preset resolver error, got: %v", err)
		}
	})
}

func TestConfigService_Get(t *testing.T) {
	t.Run("Successful configuration retrieval", func(t *testing.T) {
		configResolver := &mockConfigResolver{}
		presetResolver := &mockPresetResolver{}

		service, _ := NewConfigService(configResolver, presetResolver)
		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg == nil {
			t.Fatal("Expected config to be non-nil")
		}
		if cfg.Preset == nil {
			t.Fatal("Expected preset to be non-nil")
		}
		if cfg.ChunkerMaxRunes != 1000 {
			t.Errorf("Expected ChunkerMaxRunes=1000, got %d", cfg.ChunkerMaxRunes)
		}
		if cfg.ChunkerOverlapRunes != 100 {
			t.Errorf("Expected ChunkerOverlapRunes=100, got %d", cfg.ChunkerOverlapRunes)
		}
		if cfg.BatchSize != 32 {
			t.Errorf("Expected BatchSize=32, got %d", cfg.BatchSize)
		}
	})

	t.Run("Nil service", func(t *testing.T) {
		var service *ConfigService = nil

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for nil service")
		}
		if !strings.Contains(err.Error(), "index config service is nil") {
			t.Errorf("Expected service nil error, got: %v", err)
		}
	})

	t.Run("ConfigResolver returns error", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return nil, errors.New("config error")
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error from configResolver")
		}
		if !strings.Contains(err.Error(), "failed to get application config") {
			t.Errorf("Expected config error, got: %v", err)
		}
	})

	t.Run("Missing index configuration", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: nil,
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for missing index config")
		}
		if !strings.Contains(err.Error(), "index configuration is missing") {
			t.Errorf("Expected missing index config error, got: %v", err)
		}
	})

	t.Run("Empty preset name", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: &config.FlowsConfig{
						Index: &config.IndexFlowConfig{
							Preset: "",
							Chunker: &config.ChunkerConfig{
								MaxRunes:     1000,
								OverlapRunes: 100,
							},
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for empty preset")
		}
		if !strings.Contains(err.Error(), "index.preset is required") {
			t.Errorf("Expected preset required error, got: %v", err)
		}
	})

	t.Run("Missing chunker configuration", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: &config.FlowsConfig{
						Index: &config.IndexFlowConfig{
							Preset:  "test-preset",
							Chunker: nil,
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error for missing chunker config")
		}
		if !strings.Contains(err.Error(), "index.chunker configuration is missing") {
			t.Errorf("Expected chunker config error, got: %v", err)
		}
	})

	t.Run("PresetResolver returns error", func(t *testing.T) {
		presetResolver := &mockPresetResolver{
			GetFunc: func(prof preset.Preset) (*preset.ResolvedPreset, error) {
				return nil, errors.New("preset error")
			},
		}
		service, _ := NewConfigService(&mockConfigResolver{}, presetResolver)

		_, err := service.Get()
		if err == nil {
			t.Fatal("Expected error from presetResolver")
		}
		if !strings.Contains(err.Error(), "failed to resolve preset") {
			t.Errorf("Expected preset resolve error, got: %v", err)
		}
	})

	t.Run("Default batch size when zero", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: &config.FlowsConfig{
						Index: &config.IndexFlowConfig{
							Preset: "test-preset",
							Chunker: &config.ChunkerConfig{
								MaxRunes:     1000,
								OverlapRunes: 100,
							},
							BatchSize: 0, // Should default to 32
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.BatchSize != 32 {
			t.Errorf("Expected default BatchSize=32, got %d", cfg.BatchSize)
		}
	})

	t.Run("Default batch size when negative", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: &config.FlowsConfig{
						Index: &config.IndexFlowConfig{
							Preset: "test-preset",
							Chunker: &config.ChunkerConfig{
								MaxRunes:     1000,
								OverlapRunes: 100,
							},
							BatchSize: -10, // Should default to 32
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.BatchSize != 32 {
			t.Errorf("Expected default BatchSize=32, got %d", cfg.BatchSize)
		}
	})

	t.Run("Custom batch size", func(t *testing.T) {
		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: &config.FlowsConfig{
						Index: &config.IndexFlowConfig{
							Preset: "test-preset",
							Chunker: &config.ChunkerConfig{
								MaxRunes:     1000,
								OverlapRunes: 100,
							},
							BatchSize: 64,
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.BatchSize != 64 {
			t.Errorf("Expected BatchSize=64, got %d", cfg.BatchSize)
		}
	})

	t.Run("Preserves chunker configuration", func(t *testing.T) {
		expectedMaxRunes := 2000
		expectedOverlapRunes := 200

		configResolver := &mockConfigResolver{
			GetFunc: func() (*config.Config, error) {
				return &config.Config{
					Flows: &config.FlowsConfig{
						Index: &config.IndexFlowConfig{
							Preset: "test-preset",
							Chunker: &config.ChunkerConfig{
								MaxRunes:     expectedMaxRunes,
								OverlapRunes: expectedOverlapRunes,
							},
							BatchSize: 32,
						},
					},
				}, nil
			},
		}
		service, _ := NewConfigService(configResolver, &mockPresetResolver{})

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.ChunkerMaxRunes != expectedMaxRunes {
			t.Errorf("Expected ChunkerMaxRunes=%d, got %d", expectedMaxRunes, cfg.ChunkerMaxRunes)
		}
		if cfg.ChunkerOverlapRunes != expectedOverlapRunes {
			t.Errorf("Expected ChunkerOverlapRunes=%d, got %d", expectedOverlapRunes, cfg.ChunkerOverlapRunes)
		}
	})
}

func TestConfigService_Get_ReturnsResolvedConfig(t *testing.T) {
	t.Run("Returns valid ResolvedConfig structure", func(t *testing.T) {
		expectedPreset := &preset.ResolvedPreset{
			Name: "test-preset",
		}
		presetResolver := &mockPresetResolver{
			GetFunc: func(prof preset.Preset) (*preset.ResolvedPreset, error) {
				return expectedPreset, nil
			},
		}
		service, _ := NewConfigService(&mockConfigResolver{}, presetResolver)

		cfg, err := service.Get()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cfg == nil {
			t.Fatal("Expected ResolvedConfig to be non-nil")
		}

		if cfg.Preset != expectedPreset {
			t.Error("Expected preset to match")
		}
	})
}
