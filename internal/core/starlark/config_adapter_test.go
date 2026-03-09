// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainConfig "github.com/retran/meowg1k/internal/domain/config"
)

// TestApplyConfigToYAML_EmptyRuntime tests applying empty Starlark config.
func TestApplyConfigToYAML_EmptyRuntime(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	t.Run("nil base config", func(t *testing.T) {
		result, err := runtime.ApplyConfigToYAML(nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Providers)
		assert.Empty(t, result.Models)
		assert.Empty(t, result.Presets)
	})

	t.Run("with existing base config", func(t *testing.T) {
		baseConfig := &domainConfig.Config{
			Providers: map[string]*domainConfig.ProviderConfig{
				"existing": {Type: "anthropic"},
			},
			Models:  make(map[string]*domainConfig.ModelConfig),
			Presets: make(map[string]*domainConfig.PresetConfig),
		}

		result, err := runtime.ApplyConfigToYAML(baseConfig)
		require.NoError(t, err)
		require.NotNil(t, result)
		// Existing provider should remain
		assert.Contains(t, result.Providers, "existing")
	})
}

// TestApplyConfigToYAML_Providers tests provider configuration mapping.
func TestApplyConfigToYAML_Providers(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	// Add providers to runtime
	runtime.providers = map[string]ProviderConfig{
		"anthropic": {
			Type:       "anthropic",
			APIKey:     "test-key",
			BaseURL:    "https://api.anthropic.com",
			Tokenizer:  "cl100k_base",
			RetryCount: 3,
		},
		"openai": {
			Type:    "openai",
			APIKey:  "openai-key",
			BaseURL: "",
		},
	}

	result, err := runtime.ApplyConfigToYAML(nil)
	require.NoError(t, err)

	t.Run("anthropic provider mapped", func(t *testing.T) {
		provider, ok := result.Providers["anthropic"]
		require.True(t, ok)
		assert.Equal(t, "anthropic", provider.Type)
		assert.Equal(t, "test-key", provider.APIKey)
		assert.Equal(t, "https://api.anthropic.com", provider.BaseURL)
		assert.Equal(t, "cl100k_base", provider.Tokenizer)
		assert.Equal(t, 3, provider.RetryCount)
	})

	t.Run("openai provider mapped", func(t *testing.T) {
		provider, ok := result.Providers["openai"]
		require.True(t, ok)
		assert.Equal(t, "openai", provider.Type)
		assert.Equal(t, "openai-key", provider.APIKey)
		assert.Equal(t, "", provider.BaseURL)
	})
}

// TestApplyConfigToYAML_Models tests model configuration mapping.
func TestApplyConfigToYAML_Models(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	runtime.models = map[string]ModelConfig{
		"gpt4": {
			Provider:        "openai",
			Model:           "gpt-4",
			MaxInputTokens:  128000,
			MaxOutputTokens: 4096,
			RateLimitRPM:    500,
			RateLimitTPM:    150000,
			RateLimitRPD:    0,
		},
		"claude": {
			Provider: "anthropic",
			Model:    "claude-3-opus",
			// No limits or rate limits
		},
		"with_rate_limits_only": {
			Provider:     "openai",
			Model:        "gpt-3.5-turbo",
			RateLimitRPM: 3500,
			RateLimitTPM: 0,
			RateLimitRPD: 10000,
		},
	}

	result, err := runtime.ApplyConfigToYAML(nil)
	require.NoError(t, err)

	t.Run("model with limits and rate limits", func(t *testing.T) {
		model, ok := result.Models["gpt4"]
		require.True(t, ok)
		assert.Equal(t, "openai", model.Provider)
		assert.Equal(t, "gpt-4", model.Model)

		require.NotNil(t, model.Limits)
		assert.Equal(t, 128000, model.Limits.MaxInputTokens)
		assert.Equal(t, 4096, model.Limits.MaxOutputTokens)

		require.NotNil(t, model.RateLimit)
		assert.Equal(t, 500, model.RateLimit.RequestsPerMinute)
		assert.Equal(t, 150000, model.RateLimit.TokensPerMinute)
		assert.Equal(t, 0, model.RateLimit.RequestsPerDay)
	})

	t.Run("model without limits or rate limits", func(t *testing.T) {
		model, ok := result.Models["claude"]
		require.True(t, ok)
		assert.Equal(t, "anthropic", model.Provider)
		assert.Equal(t, "claude-3-opus", model.Model)
		assert.Nil(t, model.Limits) // No limits set
		assert.Nil(t, model.RateLimit)
	})

	t.Run("model with only rate limits", func(t *testing.T) {
		model, ok := result.Models["with_rate_limits_only"]
		require.True(t, ok)
		assert.Nil(t, model.Limits) // No token limits
		require.NotNil(t, model.RateLimit)
		assert.Equal(t, 3500, model.RateLimit.RequestsPerMinute)
		assert.Equal(t, 10000, model.RateLimit.RequestsPerDay)
	})
}

// TestApplyConfigToYAML_Presets tests preset configuration mapping.
func TestApplyConfigToYAML_Presets(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	runtime.presets = map[string]PresetConfig{
		"creative": {
			Model:            "gpt4",
			Temperature:      0.9,
			MaxTokens:        2000,
			TopP:             0.95,
			TopK:             40,
			FrequencyPenalty: 0.5,
			PresencePenalty:  0.3,
			Extends:          "",
		},
		"precise": {
			Model:       "claude",
			Temperature: 0.1,
			MaxTokens:   1000,
			// Other fields zero
		},
		"extended": {
			Model:   "gpt4",
			Extends: "creative",
			// Override only temperature
			Temperature: 0.7,
		},
	}

	result, err := runtime.ApplyConfigToYAML(nil)
	require.NoError(t, err)

	t.Run("preset with all parameters", func(t *testing.T) {
		preset, ok := result.Presets["creative"]
		require.True(t, ok)
		assert.Equal(t, "gpt4", preset.Model)
		assert.Equal(t, "", preset.Extends)

		require.NotNil(t, preset.Request)
		require.NotNil(t, preset.Request.Temperature)
		assert.Equal(t, 0.9, *preset.Request.Temperature)

		require.NotNil(t, preset.Request.MaxTokens)
		assert.Equal(t, 2000, *preset.Request.MaxTokens)

		require.NotNil(t, preset.Request.TopP)
		assert.Equal(t, 0.95, *preset.Request.TopP)

		require.NotNil(t, preset.Request.TopK)
		assert.Equal(t, 40, *preset.Request.TopK)

		require.NotNil(t, preset.Request.FrequencyPenalty)
		assert.Equal(t, 0.5, *preset.Request.FrequencyPenalty)

		require.NotNil(t, preset.Request.PresencePenalty)
		assert.Equal(t, 0.3, *preset.Request.PresencePenalty)
	})

	t.Run("preset with minimal parameters", func(t *testing.T) {
		preset, ok := result.Presets["precise"]
		require.True(t, ok)
		assert.Equal(t, "claude", preset.Model)

		require.NotNil(t, preset.Request)
		require.NotNil(t, preset.Request.Temperature)
		assert.Equal(t, 0.1, *preset.Request.Temperature)

		require.NotNil(t, preset.Request.MaxTokens)
		assert.Equal(t, 1000, *preset.Request.MaxTokens)

		// Zero values should result in nil pointers
		assert.Nil(t, preset.Request.TopP)
		assert.Nil(t, preset.Request.TopK)
		assert.Nil(t, preset.Request.FrequencyPenalty)
		assert.Nil(t, preset.Request.PresencePenalty)
	})

	t.Run("preset with extends", func(t *testing.T) {
		preset, ok := result.Presets["extended"]
		require.True(t, ok)
		assert.Equal(t, "gpt4", preset.Model)
		assert.Equal(t, "creative", preset.Extends)

		require.NotNil(t, preset.Request.Temperature)
		assert.Equal(t, 0.7, *preset.Request.Temperature)
	})
}

// TestHasConfiguration tests configuration detection.
func TestHasConfiguration(t *testing.T) {
	t.Run("empty runtime", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		assert.False(t, runtime.HasConfiguration())
	})

	t.Run("runtime with provider", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.providers = map[string]ProviderConfig{
			"test": {Type: "anthropic"},
		}
		assert.True(t, runtime.HasConfiguration())
	})

	t.Run("runtime with model", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.models = map[string]ModelConfig{
			"test": {Provider: "test", Model: "gpt-4"},
		}
		assert.True(t, runtime.HasConfiguration())
	})

	t.Run("runtime with preset", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.presets = map[string]PresetConfig{
			"test": {Model: "test", Temperature: 0.5},
		}
		assert.True(t, runtime.HasConfiguration())
	})

	t.Run("runtime with all configuration types", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.providers = map[string]ProviderConfig{"p": {Type: "anthropic"}}
		runtime.models = map[string]ModelConfig{"m": {Provider: "p", Model: "gpt-4"}}
		runtime.presets = map[string]PresetConfig{"pr": {Model: "m"}}
		assert.True(t, runtime.HasConfiguration())
	})
}

// TestIntPtr tests the intPtr helper function.
func TestIntPtr(t *testing.T) {
	t.Run("zero returns nil", func(t *testing.T) {
		result := intPtr(0)
		assert.Nil(t, result)
	})

	t.Run("non-zero returns pointer", func(t *testing.T) {
		result := intPtr(42)
		require.NotNil(t, result)
		assert.Equal(t, 42, *result)
	})

	t.Run("negative returns pointer", func(t *testing.T) {
		result := intPtr(-10)
		require.NotNil(t, result)
		assert.Equal(t, -10, *result)
	})
}

// TestFloat64Ptr tests the float64Ptr helper function.
func TestFloat64Ptr(t *testing.T) {
	t.Run("zero returns nil", func(t *testing.T) {
		result := float64Ptr(0.0)
		assert.Nil(t, result)
	})

	t.Run("non-zero returns pointer", func(t *testing.T) {
		result := float64Ptr(3.14)
		require.NotNil(t, result)
		assert.Equal(t, 3.14, *result)
	})

	t.Run("negative returns pointer", func(t *testing.T) {
		result := float64Ptr(-2.5)
		require.NotNil(t, result)
		assert.Equal(t, -2.5, *result)
	})

	t.Run("very small non-zero returns pointer", func(t *testing.T) {
		result := float64Ptr(0.001)
		require.NotNil(t, result)
		assert.Equal(t, 0.001, *result)
	})
}

// TestValidateConfiguration tests configuration validation.
func TestValidateConfiguration(t *testing.T) {
	t.Run("empty configuration is valid", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		err := runtime.ValidateConfiguration()
		assert.NoError(t, err)
	})

	t.Run("valid provider-model-preset chain", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.providers = map[string]ProviderConfig{
			"anthropic": {Type: "anthropic", APIKey: "key"},
		}
		runtime.models = map[string]ModelConfig{
			"claude": {Provider: "anthropic", Model: "claude-3-opus"},
		}
		runtime.presets = map[string]PresetConfig{
			"creative": {Model: "claude", Temperature: 0.9},
		}

		err := runtime.ValidateConfiguration()
		assert.NoError(t, err)
	})

	t.Run("model references unknown provider", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.models = map[string]ModelConfig{
			"gpt4": {Provider: "openai", Model: "gpt-4"},
		}
		// No providers defined

		err := runtime.ValidateConfiguration()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gpt4")
		assert.Contains(t, err.Error(), "unknown provider")
		assert.Contains(t, err.Error(), "openai")
	})

	t.Run("preset references unknown model", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.providers = map[string]ProviderConfig{
			"anthropic": {Type: "anthropic"},
		}
		runtime.presets = map[string]PresetConfig{
			"fast": {Model: "claude", Temperature: 0.2},
		}
		// No models defined

		err := runtime.ValidateConfiguration()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fast")
		assert.Contains(t, err.Error(), "unknown model")
		assert.Contains(t, err.Error(), "claude")
	})

	t.Run("multiple validation errors", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.models = map[string]ModelConfig{
			"model1": {Provider: "provider1", Model: "m1"},
			"model2": {Provider: "provider2", Model: "m2"},
		}

		// First model references unknown provider - should fail on first error
		err := runtime.ValidateConfiguration()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})

	t.Run("only providers and models without presets", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		runtime.providers = map[string]ProviderConfig{
			"openai": {Type: "openai"},
		}
		runtime.models = map[string]ModelConfig{
			"gpt4": {Provider: "openai", Model: "gpt-4"},
		}

		err := runtime.ValidateConfiguration()
		assert.NoError(t, err)
	})
}

// TestApplyConfigToYAML_Integration tests end-to-end configuration mapping.
func TestApplyConfigToYAML_Integration(t *testing.T) {
	runtime := NewRuntime(t.TempDir())

	// Set up complete configuration
	runtime.providers = map[string]ProviderConfig{
		"anthropic": {
			Type:       "anthropic",
			APIKey:     "anthropic-key",
			BaseURL:    "https://api.anthropic.com",
			RetryCount: 3,
		},
		"openai": {
			Type:    "openai",
			APIKey:  "openai-key",
			BaseURL: "https://api.openai.com/v1",
		},
	}

	runtime.models = map[string]ModelConfig{
		"claude": {
			Provider:        "anthropic",
			Model:           "claude-3-opus",
			MaxInputTokens:  200000,
			MaxOutputTokens: 4096,
		},
		"gpt4": {
			Provider:       "openai",
			Model:          "gpt-4-turbo",
			RateLimitRPM:   500,
			RateLimitTPM:   150000,
			RateLimitRPD:   10000,
			MaxInputTokens: 128000,
		},
	}

	runtime.presets = map[string]PresetConfig{
		"creative": {
			Model:            "claude",
			Temperature:      0.9,
			MaxTokens:        2000,
			TopP:             0.95,
			FrequencyPenalty: 0.5,
		},
		"precise": {
			Model:       "gpt4",
			Temperature: 0.1,
			MaxTokens:   1000,
		},
	}

	// Apply configuration
	result, err := runtime.ApplyConfigToYAML(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify all components are present
	assert.Len(t, result.Providers, 2)
	assert.Len(t, result.Models, 2)
	assert.Len(t, result.Presets, 2)

	// Validate configuration
	err = runtime.ValidateConfiguration()
	assert.NoError(t, err)
}
