// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// TestMeowModuleCreation tests createMeowModule function
func TestMeowModuleCreation(t *testing.T) {
	runtime := NewRuntime("/tmp")
	module := runtime.createMeowModule()

	require.NotNil(t, module)

	moduleStruct, ok := module.(*starlarkstruct.Struct)
	require.True(t, ok)

	// Verify all expected functions exist
	provider, err := moduleStruct.Attr("provider")
	require.NoError(t, err)
	require.NotNil(t, provider)

	model, err := moduleStruct.Attr("model")
	require.NoError(t, err)
	require.NotNil(t, model)

	preset, err := moduleStruct.Attr("preset")
	require.NoError(t, err)
	require.NotNil(t, preset)

	presets, err := moduleStruct.Attr("presets")
	require.NoError(t, err)
	require.NotNil(t, presets)

	param, err := moduleStruct.Attr("param")
	require.NoError(t, err)
	require.NotNil(t, param)

	tool, err := moduleStruct.Attr("tool")
	require.NoError(t, err)
	require.NotNil(t, tool)

	command, err := moduleStruct.Attr("command")
	require.NoError(t, err)
	require.NotNil(t, command)
}

// TestMeowProvider tests provider registration
func TestMeowProvider(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("register basic provider", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("test-provider")}
		kwargs := []starlark.Tuple{
			{starlark.String("type"), starlark.String("anthropic")},
			{starlark.String("api_key"), starlark.String("test-key")},
		}

		result, err := runtime.meowProvider(thread, starlark.NewBuiltin("provider", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		// Verify provider was registered
		providerConfig, exists := runtime.providers["test-provider"]
		assert.True(t, exists)
		assert.Equal(t, "anthropic", providerConfig.Type)
		assert.Equal(t, "test-key", providerConfig.APIKey)
	})

	t.Run("register provider with all fields", func(t *testing.T) {
		runtime := NewRuntime("/tmp") // Fresh runtime
		args := starlark.Tuple{starlark.String("full-provider")}
		kwargs := []starlark.Tuple{
			{starlark.String("type"), starlark.String("openai")},
			{starlark.String("base_url"), starlark.String("https://api.example.com")},
			{starlark.String("api_key"), starlark.String("key123")},
			{starlark.String("tokenizer"), starlark.String("cl100k_base")},
			{starlark.String("retry_count"), starlark.MakeInt(5)},
		}

		result, err := runtime.meowProvider(thread, starlark.NewBuiltin("provider", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		providerConfig := runtime.providers["full-provider"]
		assert.Equal(t, "openai", providerConfig.Type)
		assert.Equal(t, "https://api.example.com", providerConfig.BaseURL)
		assert.Equal(t, "key123", providerConfig.APIKey)
		assert.Equal(t, "cl100k_base", providerConfig.Tokenizer)
		assert.Equal(t, 5, providerConfig.RetryCount)
	})

	t.Run("register provider with extra options", func(t *testing.T) {
		runtime := NewRuntime("/tmp")
		args := starlark.Tuple{starlark.String("extra-provider")}
		kwargs := []starlark.Tuple{
			{starlark.String("type"), starlark.String("custom")},
			{starlark.String("custom_field"), starlark.String("custom_value")},
		}

		result, err := runtime.meowProvider(thread, starlark.NewBuiltin("provider", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		providerConfig := runtime.providers["extra-provider"]
		assert.Equal(t, "custom", providerConfig.Type)
		assert.Equal(t, "custom_value", providerConfig.ExtraOpts["custom_field"])
	})
}

// TestMeowProviderErrors tests error cases for provider registration
func TestMeowProviderErrors(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing name argument", func(t *testing.T) {
		_, err := runtime.meowProvider(thread, starlark.NewBuiltin("provider", nil),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 positional argument")
	})

	t.Run("wrong argument type for name", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.meowProvider(thread, starlark.NewBuiltin("provider", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be a string")
	})

	t.Run("too many positional arguments", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("name1"), starlark.String("name2")}
		_, err := runtime.meowProvider(thread, starlark.NewBuiltin("provider", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 positional argument")
	})
}

// TestMeowModel tests model registration
func TestMeowModel(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("register basic model", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("test-model")}
		kwargs := []starlark.Tuple{
			{starlark.String("provider"), starlark.String("test-provider")},
			{starlark.String("model"), starlark.String("gpt-4")},
		}

		result, err := runtime.meowModel(thread, starlark.NewBuiltin("model", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		modelConfig := runtime.models["test-model"]
		assert.Equal(t, "test-provider", modelConfig.Provider)
		assert.Equal(t, "gpt-4", modelConfig.Model)
	})

	t.Run("register model with all fields", func(t *testing.T) {
		runtime := NewRuntime("/tmp")
		args := starlark.Tuple{starlark.String("full-model")}
		kwargs := []starlark.Tuple{
			{starlark.String("provider"), starlark.String("openai")},
			{starlark.String("model"), starlark.String("gpt-4-turbo")},
			{starlark.String("max_input_tokens"), starlark.MakeInt(128000)},
			{starlark.String("max_output_tokens"), starlark.MakeInt(4096)},
			{starlark.String("rate_limit_rpm"), starlark.MakeInt(500)},
			{starlark.String("rate_limit_tpm"), starlark.MakeInt(200000)},
			{starlark.String("rate_limit_rpd"), starlark.MakeInt(10000)},
		}

		result, err := runtime.meowModel(thread, starlark.NewBuiltin("model", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		modelConfig := runtime.models["full-model"]
		assert.Equal(t, "openai", modelConfig.Provider)
		assert.Equal(t, "gpt-4-turbo", modelConfig.Model)
		assert.Equal(t, 128000, modelConfig.MaxInputTokens)
		assert.Equal(t, 4096, modelConfig.MaxOutputTokens)
		assert.Equal(t, 500, modelConfig.RateLimitRPM)
		assert.Equal(t, 200000, modelConfig.RateLimitTPM)
		assert.Equal(t, 10000, modelConfig.RateLimitRPD)
	})

	t.Run("register model with extra options", func(t *testing.T) {
		runtime := NewRuntime("/tmp")
		args := starlark.Tuple{starlark.String("extra-model")}
		kwargs := []starlark.Tuple{
			{starlark.String("provider"), starlark.String("custom")},
			{starlark.String("model"), starlark.String("custom-model")},
			{starlark.String("custom_param"), starlark.String("custom_value")},
		}

		result, err := runtime.meowModel(thread, starlark.NewBuiltin("model", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		modelConfig := runtime.models["extra-model"]
		assert.Equal(t, "custom", modelConfig.Provider)
		assert.Equal(t, "custom_value", modelConfig.ExtraOpts["custom_param"])
	})
}

// TestMeowModelErrors tests error cases for model registration
func TestMeowModelErrors(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing name argument", func(t *testing.T) {
		_, err := runtime.meowModel(thread, starlark.NewBuiltin("model", nil),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 positional argument")
	})

	t.Run("wrong argument type for name", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.meowModel(thread, starlark.NewBuiltin("model", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be a string")
	})

	t.Run("too many positional arguments", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("name1"), starlark.String("name2")}
		_, err := runtime.meowModel(thread, starlark.NewBuiltin("model", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 positional argument")
	})
}

// TestMeowPreset tests preset registration
func TestMeowPreset(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("register basic preset", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("test-preset")}
		kwargs := []starlark.Tuple{
			{starlark.String("model"), starlark.String("test-model")},
			{starlark.String("temperature"), starlark.Float(0.7)},
		}

		result, err := runtime.meowPreset(thread, starlark.NewBuiltin("preset", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		presetConfig := runtime.presets["test-preset"]
		assert.Equal(t, "test-model", presetConfig.Model)
		assert.Equal(t, 0.7, presetConfig.Temperature)
	})

	t.Run("register preset with all fields", func(t *testing.T) {
		runtime := NewRuntime("/tmp")
		args := starlark.Tuple{starlark.String("full-preset")}
		kwargs := []starlark.Tuple{
			{starlark.String("model"), starlark.String("gpt-4")},
			{starlark.String("extends"), starlark.String("base-preset")},
			{starlark.String("temperature"), starlark.Float(0.8)},
			{starlark.String("max_tokens"), starlark.MakeInt(2000)},
			{starlark.String("top_p"), starlark.Float(0.9)},
			{starlark.String("top_k"), starlark.MakeInt(40)},
			{starlark.String("frequency_penalty"), starlark.Float(0.5)},
			{starlark.String("presence_penalty"), starlark.Float(0.6)},
		}

		result, err := runtime.meowPreset(thread, starlark.NewBuiltin("preset", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		presetConfig := runtime.presets["full-preset"]
		assert.Equal(t, "gpt-4", presetConfig.Model)
		assert.Equal(t, "base-preset", presetConfig.Extends)
		assert.Equal(t, 0.8, presetConfig.Temperature)
		assert.Equal(t, 2000, presetConfig.MaxTokens)
		assert.Equal(t, 0.9, presetConfig.TopP)
		assert.Equal(t, 40, presetConfig.TopK)
		assert.Equal(t, 0.5, presetConfig.FrequencyPenalty)
		assert.Equal(t, 0.6, presetConfig.PresencePenalty)
	})

	t.Run("register preset with extra options", func(t *testing.T) {
		runtime := NewRuntime("/tmp")
		args := starlark.Tuple{starlark.String("extra-preset")}
		kwargs := []starlark.Tuple{
			{starlark.String("model"), starlark.String("custom-model")},
			{starlark.String("custom_option"), starlark.String("custom_value")},
		}

		result, err := runtime.meowPreset(thread, starlark.NewBuiltin("preset", nil), args, kwargs)
		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)

		presetConfig := runtime.presets["extra-preset"]
		assert.Equal(t, "custom-model", presetConfig.Model)
		assert.Equal(t, "custom_value", presetConfig.ExtraOpts["custom_option"])
	})
}

// TestMeowPresetErrors tests error cases for preset registration
func TestMeowPresetErrors(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing name argument", func(t *testing.T) {
		_, err := runtime.meowPreset(thread, starlark.NewBuiltin("preset", nil),
			starlark.Tuple{},
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 positional argument")
	})

	t.Run("wrong argument type for name", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.meowPreset(thread, starlark.NewBuiltin("preset", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be a string")
	})

	t.Run("too many positional arguments", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("name1"), starlark.String("name2")}
		_, err := runtime.meowPreset(thread, starlark.NewBuiltin("preset", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 positional argument")
	})
}

// TestMeowPresets tests listing registered presets
func TestMeowPresets(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	// Register some presets
	runtime.RegisterPreset("preset1", PresetConfig{Model: "model1"})
	runtime.RegisterPreset("preset2", PresetConfig{Model: "model2"})
	runtime.RegisterPreset("preset3", PresetConfig{Model: "model3"})

	result, err := runtime.meowPresets(thread, starlark.NewBuiltin("presets", nil),
		starlark.Tuple{},
		nil)
	require.NoError(t, err)

	list, ok := result.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 3, list.Len())

	// Check that preset names are in the list
	presetNames := make(map[string]bool)
	for i := 0; i < list.Len(); i++ {
		name, ok := list.Index(i).(starlark.String)
		require.True(t, ok)
		presetNames[string(name)] = true
	}
	assert.True(t, presetNames["preset1"])
	assert.True(t, presetNames["preset2"])
	assert.True(t, presetNames["preset3"])
}

// TestMeowPresetsErrors tests error cases for presets listing
func TestMeowPresetsErrors(t *testing.T) {
	runtime := NewRuntime("/tmp")
	thread := &starlark.Thread{Name: "test"}

	t.Run("unexpected arguments", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("unexpected")}
		_, err := runtime.meowPresets(thread, starlark.NewBuiltin("presets", nil),
			args,
			nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 0 arguments")
	})
}

// TestStarlarkToGo tests the starlarkToGo conversion function
func TestStarlarkToGo(t *testing.T) {
	tests := []struct {
		name     string
		input    starlark.Value
		expected interface{}
	}{
		{
			name:     "string",
			input:    starlark.String("hello"),
			expected: "hello",
		},
		{
			name:     "int",
			input:    starlark.MakeInt(42),
			expected: 42,
		},
		{
			name:     "bool true",
			input:    starlark.True,
			expected: true,
		},
		{
			name:     "bool false",
			input:    starlark.False,
			expected: false,
		},
		{
			name:     "float",
			input:    starlark.Float(3.14),
			expected: float64(3.14),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := starlarkToGo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStarlarkToGoComplexTypes tests complex type conversions
func TestStarlarkToGoComplexTypes(t *testing.T) {
	t.Run("list conversion", func(t *testing.T) {
		list := starlark.NewList([]starlark.Value{
			starlark.String("a"),
			starlark.MakeInt(1),
			starlark.True,
		})
		result := starlarkToGo(list)
		slice, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, slice, 3)
		assert.Equal(t, "a", slice[0])
		assert.Equal(t, 1, slice[1])
		assert.Equal(t, true, slice[2])
	})

	t.Run("dict conversion", func(t *testing.T) {
		dict := starlark.NewDict(2)
		dict.SetKey(starlark.String("name"), starlark.String("Alice"))
		dict.SetKey(starlark.String("age"), starlark.MakeInt(30))

		result := starlarkToGo(dict)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Alice", resultMap["name"])
		assert.Equal(t, 30, resultMap["age"])
	})

	t.Run("nested structures", func(t *testing.T) {
		innerDict := starlark.NewDict(1)
		innerDict.SetKey(starlark.String("inner"), starlark.String("value"))

		outerDict := starlark.NewDict(1)
		outerDict.SetKey(starlark.String("outer"), innerDict)

		result := starlarkToGo(outerDict)
		outerMap, ok := result.(map[string]interface{})
		require.True(t, ok)

		innerMap, ok := outerMap["outer"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", innerMap["inner"])
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		// Use a tuple which is not handled
		tuple := starlark.Tuple{starlark.String("test")}
		result := starlarkToGo(tuple)
		assert.Nil(t, result)
	})

	t.Run("dict with non-string keys", func(t *testing.T) {
		dict := starlark.NewDict(1)
		dict.SetKey(starlark.MakeInt(123), starlark.String("value"))

		result := starlarkToGo(dict)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		// Non-string keys should be ignored
		assert.Len(t, resultMap, 0)
	})
}
