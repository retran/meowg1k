// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewService tests creating a new config service
func TestNewService(t *testing.T) {
	service, err := NewService()
	require.NoError(t, err)
	require.NotNil(t, service)

	// Verify initial config is not nil
	cfg, err := service.Get()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify maps are initialized
	assert.NotNil(t, cfg.Providers, "Providers map should be initialized")
	assert.NotNil(t, cfg.Models, "Models map should be initialized")
	assert.NotNil(t, cfg.Presets, "Presets map should be initialized")
	assert.Empty(t, cfg.Providers, "Providers should be empty initially")
	assert.Empty(t, cfg.Models, "Models should be empty initially")
	assert.Empty(t, cfg.Presets, "Presets should be empty initially")
}

// TestGet tests retrieving configuration
func TestGet(t *testing.T) {
	t.Run("normal service", func(t *testing.T) {
		service, err := NewService()
		require.NoError(t, err)

		cfg, err := service.Get()
		require.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		cfg, err := service.Get()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "service is nil")
	})
}

// TestOverride tests overriding configuration
func TestOverride(t *testing.T) {
	t.Run("successful override", func(t *testing.T) {
		service, err := NewService()
		require.NoError(t, err)

		// Create new config with some data
		newCfg := &config.Config{
			Providers: map[string]*config.ProviderConfig{
				"test-provider": {
					Type:   "openai",
					APIKey: "test-key",
				},
			},
			Models: map[string]*config.ModelConfig{
				"test-model": {
					Provider: "test-provider",
					Model:    "gpt-4",
				},
			},
			Presets: map[string]*config.PresetConfig{
				"test-preset": {
					Model: "test-model",
				},
			},
		}

		// Override with new config
		err = service.Override(newCfg)
		require.NoError(t, err)

		// Verify config was updated
		retrievedCfg, err := service.Get()
		require.NoError(t, err)
		assert.Equal(t, newCfg, retrievedCfg)
		assert.Len(t, retrievedCfg.Providers, 1)
		assert.Len(t, retrievedCfg.Models, 1)
		assert.Len(t, retrievedCfg.Presets, 1)
		assert.Contains(t, retrievedCfg.Providers, "test-provider")
		assert.Contains(t, retrievedCfg.Models, "test-model")
		assert.Contains(t, retrievedCfg.Presets, "test-preset")
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		newCfg := &config.Config{
			Providers: make(map[string]*config.ProviderConfig),
			Models:    make(map[string]*config.ModelConfig),
			Presets:   make(map[string]*config.PresetConfig),
		}

		err := service.Override(newCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil config", func(t *testing.T) {
		service, err := NewService()
		require.NoError(t, err)

		err = service.Override(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration cannot be nil")
	})
}

// TestOverrideMultipleTimes tests overriding config multiple times
func TestOverrideMultipleTimes(t *testing.T) {
	service, err := NewService()
	require.NoError(t, err)

	// First override
	cfg1 := &config.Config{
		Providers: map[string]*config.ProviderConfig{
			"provider1": {Type: "openai"},
		},
		Models:  make(map[string]*config.ModelConfig),
		Presets: make(map[string]*config.PresetConfig),
	}
	err = service.Override(cfg1)
	require.NoError(t, err)

	retrieved, err := service.Get()
	require.NoError(t, err)
	assert.Len(t, retrieved.Providers, 1)

	// Second override - should replace first
	cfg2 := &config.Config{
		Providers: map[string]*config.ProviderConfig{
			"provider1": {Type: "openai"},
			"provider2": {Type: "anthropic"},
		},
		Models:  make(map[string]*config.ModelConfig),
		Presets: make(map[string]*config.PresetConfig),
	}
	err = service.Override(cfg2)
	require.NoError(t, err)

	retrieved, err = service.Get()
	require.NoError(t, err)
	assert.Len(t, retrieved.Providers, 2)
	assert.Contains(t, retrieved.Providers, "provider1")
	assert.Contains(t, retrieved.Providers, "provider2")
}

// TestOverrideWithComplexConfig tests overriding with full config structure
func TestOverrideWithComplexConfig(t *testing.T) {
	service, err := NewService()
	require.NoError(t, err)

	temp := 0.7
	maxTokens := 1000

	complexCfg := &config.Config{
		Providers: map[string]*config.ProviderConfig{
			"openai": {
				Type:    "openai",
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "sk-test-key",
				Limits: &config.ModelLimits{
					MaxInputTokens:  128000,
					MaxOutputTokens: 16384,
				},
			},
		},
		Models: map[string]*config.ModelConfig{
			"gpt4": {
				Provider: "openai",
				Model:    "gpt-4-turbo",
				Limits: &config.ModelLimits{
					MaxInputTokens:  128000,
					MaxOutputTokens: 4096,
				},
			},
		},
		Presets: map[string]*config.PresetConfig{
			"creative": {
				Model: "gpt4",
				Request: &config.RequestConfig{
					Temperature: &temp,
					MaxTokens:   &maxTokens,
				},
			},
		},
		Filter: &config.FilterConfig{
			Ignore: []string{"*.log", "*.tmp"},
		},
		SchemaVersion: 1,
	}

	err = service.Override(complexCfg)
	require.NoError(t, err)

	retrieved, err := service.Get()
	require.NoError(t, err)

	// Verify all fields
	assert.Len(t, retrieved.Providers, 1)
	assert.Len(t, retrieved.Models, 1)
	assert.Len(t, retrieved.Presets, 1)
	assert.NotNil(t, retrieved.Filter)
	assert.Equal(t, 1, retrieved.SchemaVersion)

	// Verify provider details
	provider := retrieved.Providers["openai"]
	require.NotNil(t, provider)
	assert.Equal(t, "openai", provider.Type)
	assert.Equal(t, "sk-test-key", provider.APIKey)
	assert.NotNil(t, provider.Limits)
	assert.Equal(t, 128000, provider.Limits.MaxInputTokens)

	// Verify model details
	model := retrieved.Models["gpt4"]
	require.NotNil(t, model)
	assert.Equal(t, "openai", model.Provider)
	assert.Equal(t, "gpt-4-turbo", model.Model)

	// Verify preset details
	preset := retrieved.Presets["creative"]
	require.NotNil(t, preset)
	assert.Equal(t, "gpt4", preset.Model)
	require.NotNil(t, preset.Request)
	require.NotNil(t, preset.Request.Temperature)
	assert.Equal(t, 0.7, *preset.Request.Temperature)

	// Verify filter
	assert.Len(t, retrieved.Filter.Ignore, 2)
	assert.Contains(t, retrieved.Filter.Ignore, "*.log")
}

// TestOverrideWithEmptyMaps tests overriding with empty but initialized maps
func TestOverrideWithEmptyMaps(t *testing.T) {
	service, err := NewService()
	require.NoError(t, err)

	// First add some data
	cfg1 := &config.Config{
		Providers: map[string]*config.ProviderConfig{
			"provider1": {Type: "test"},
		},
		Models:  make(map[string]*config.ModelConfig),
		Presets: make(map[string]*config.PresetConfig),
	}
	err = service.Override(cfg1)
	require.NoError(t, err)

	// Override with empty maps (should clear providers)
	cfg2 := &config.Config{
		Providers: make(map[string]*config.ProviderConfig),
		Models:    make(map[string]*config.ModelConfig),
		Presets:   make(map[string]*config.PresetConfig),
	}
	err = service.Override(cfg2)
	require.NoError(t, err)

	retrieved, err := service.Get()
	require.NoError(t, err)
	assert.Empty(t, retrieved.Providers, "Providers should be empty after override with empty map")
}

// TestServiceConcurrency tests that service operations are safe
func TestServiceConcurrency(t *testing.T) {
	service, err := NewService()
	require.NoError(t, err)

	// Note: This is a basic test. For true concurrency testing,
	// you'd need to add mutex/sync mechanisms to the service if needed.

	cfg := &config.Config{
		Providers: make(map[string]*config.ProviderConfig),
		Models:    make(map[string]*config.ModelConfig),
		Presets:   make(map[string]*config.PresetConfig),
	}

	// Multiple operations should work without panic
	err = service.Override(cfg)
	require.NoError(t, err)

	retrieved, err := service.Get()
	require.NoError(t, err)
	assert.NotNil(t, retrieved)

	err = service.Override(cfg)
	require.NoError(t, err)
}
