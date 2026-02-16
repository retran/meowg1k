// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntime(t *testing.T) {
	t.Run("creates runtime with valid working directory", func(t *testing.T) {
		workDir := t.TempDir()
		runtime := NewRuntime(workDir)

		require.NotNil(t, runtime)
		assert.Equal(t, workDir, runtime.WorkingDir())
		assert.NotNil(t, runtime.Registry())
		assert.NotNil(t, runtime.predeclared)
		assert.NotNil(t, runtime.providers)
		assert.NotNil(t, runtime.models)
		assert.NotNil(t, runtime.presets)
	})

	t.Run("initializes predeclared modules", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		// Check that meow and env modules are registered
		assert.Contains(t, runtime.predeclared, "meow")
		assert.Contains(t, runtime.predeclared, "env")
		assert.NotNil(t, runtime.predeclared["meow"])
		assert.NotNil(t, runtime.predeclared["env"])
	})
}

func TestRuntime_LoadScript_Success(t *testing.T) {
	t.Run("loads and executes simple script", func(t *testing.T) {
		tempDir := t.TempDir()
		scriptPath := filepath.Join(tempDir, "test.star")

		scriptContent := `
# Simple variable assignment
x = 42
y = "hello"
`
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(scriptPath)
		assert.NoError(t, err)
	})

	t.Run("loads script with meow module usage", func(t *testing.T) {
		tempDir := t.TempDir()
		scriptPath := filepath.Join(tempDir, "test.star")

		scriptContent := `
# Access meow module and register a tool
def handler(ctx):
    return None

meow.tool(
    name="test-tool",
    description="A test tool",
    handler=handler
)
`
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(scriptPath)
		assert.NoError(t, err)

		// Verify tool was registered
		tools := runtime.Registry().ListTools()
		require.Len(t, tools, 1)
		assert.Equal(t, "test-tool", tools[0].Name)
		assert.Equal(t, "A test tool", tools[0].Description)
	})

	t.Run("loads script with env module usage", func(t *testing.T) {
		tempDir := t.TempDir()
		scriptPath := filepath.Join(tempDir, "test.star")

		// Set a test environment variable
		t.Setenv("TEST_VAR", "test_value")

		scriptContent := `
# Access env module
api_key = env.get("TEST_VAR", "default")
`
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(scriptPath)
		assert.NoError(t, err)
	})
}

func TestRuntime_LoadScript_Errors(t *testing.T) {
	t.Run("fails on non-existent file", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		err := runtime.LoadScript("/nonexistent/file.star")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read script")
	})

	t.Run("fails on syntax error", func(t *testing.T) {
		tempDir := t.TempDir()
		scriptPath := filepath.Join(tempDir, "bad.star")

		scriptContent := `
# Invalid syntax
x = 
`
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(scriptPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "script execution failed")
	})

	t.Run("fails on undefined variable", func(t *testing.T) {
		tempDir := t.TempDir()
		scriptPath := filepath.Join(tempDir, "undefined.star")

		scriptContent := `
# Reference undefined variable
result = undefined_var + 1
`
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(scriptPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "script execution failed")
	})
}

func TestRuntime_LoadScript_WithLoad(t *testing.T) {
	t.Run("loads script with load() statement", func(t *testing.T) {
		tempDir := t.TempDir()
		libPath := filepath.Join(tempDir, "lib.star")
		mainPath := filepath.Join(tempDir, "main.star")

		// Create library file
		libContent := `
def add(a, b):
    return a + b

constant = 42
`
		err := os.WriteFile(libPath, []byte(libContent), 0o644)
		require.NoError(t, err)

		// Create main file that loads the library
		mainContent := `
load("lib.star", "add", "constant")

result = add(10, 20)
`
		err = os.WriteFile(mainPath, []byte(mainContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(mainPath)
		assert.NoError(t, err)
	})

	t.Run("loads script with nested load() statements", func(t *testing.T) {
		tempDir := t.TempDir()
		utilsPath := filepath.Join(tempDir, "utils.star")
		helperPath := filepath.Join(tempDir, "helper.star")
		mainPath := filepath.Join(tempDir, "main.star")

		// Create utils file
		utilsContent := `
def multiply(a, b):
    return a * b
`
		err := os.WriteFile(utilsPath, []byte(utilsContent), 0o644)
		require.NoError(t, err)

		// Create helper that loads utils
		helperContent := `
load("utils.star", "multiply")

def calculate(x):
    return multiply(x, 2)
`
		err = os.WriteFile(helperPath, []byte(helperContent), 0o644)
		require.NoError(t, err)

		// Create main that loads helper
		mainContent := `
load("helper.star", "calculate")

result = calculate(5)
`
		err = os.WriteFile(mainPath, []byte(mainContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(mainPath)
		assert.NoError(t, err)
	})

	t.Run("fails when loaded file does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		mainPath := filepath.Join(tempDir, "main.star")

		mainContent := `
load("nonexistent.star", "foo")
`
		err := os.WriteFile(mainPath, []byte(mainContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(tempDir)
		err = runtime.LoadScript(mainPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "script execution failed")
	})
}

func TestRuntime_RegisterProvider(t *testing.T) {
	t.Run("registers provider configuration", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		config := ProviderConfig{
			Type:    "openai",
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "test-key",
		}

		runtime.RegisterProvider("test-provider", config)

		assert.Len(t, runtime.providers, 1)
		assert.Contains(t, runtime.providers, "test-provider")
		assert.Equal(t, config.Type, runtime.providers["test-provider"].Type)
		assert.Equal(t, config.BaseURL, runtime.providers["test-provider"].BaseURL)
	})

	t.Run("overwrites existing provider", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		config1 := ProviderConfig{Type: "openai", APIKey: "key1"}
		config2 := ProviderConfig{Type: "anthropic", APIKey: "key2"}

		runtime.RegisterProvider("provider", config1)
		runtime.RegisterProvider("provider", config2)

		assert.Len(t, runtime.providers, 1)
		assert.Equal(t, "anthropic", runtime.providers["provider"].Type)
		assert.Equal(t, "key2", runtime.providers["provider"].APIKey)
	})
}

func TestRuntime_RegisterModel(t *testing.T) {
	t.Run("registers model configuration", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		config := ModelConfig{
			Provider:        "openai",
			Model:           "gpt-4",
			MaxInputTokens:  8000,
			MaxOutputTokens: 2000,
		}

		runtime.RegisterModel("test-model", config)

		assert.Len(t, runtime.models, 1)
		assert.Contains(t, runtime.models, "test-model")
		assert.Equal(t, config.Provider, runtime.models["test-model"].Provider)
		assert.Equal(t, config.Model, runtime.models["test-model"].Model)
	})

	t.Run("registers multiple models", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		runtime.RegisterModel("model1", ModelConfig{Provider: "openai", Model: "gpt-4"})
		runtime.RegisterModel("model2", ModelConfig{Provider: "anthropic", Model: "claude-3"})

		assert.Len(t, runtime.models, 2)
		assert.Contains(t, runtime.models, "model1")
		assert.Contains(t, runtime.models, "model2")
	})
}

func TestRuntime_RegisterPreset(t *testing.T) {
	t.Run("registers preset configuration", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		config := PresetConfig{
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
		}

		runtime.RegisterPreset("test-preset", config)

		assert.Len(t, runtime.presets, 1)
		assert.Contains(t, runtime.presets, "test-preset")
		assert.Equal(t, config.Model, runtime.presets["test-preset"].Model)
		assert.Equal(t, config.Temperature, runtime.presets["test-preset"].Temperature)
	})

	t.Run("registers multiple presets", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		runtime.RegisterPreset("fast", PresetConfig{Model: "gpt-3.5", Temperature: 0.2})
		runtime.RegisterPreset("creative", PresetConfig{Model: "gpt-4", Temperature: 0.9})

		assert.Len(t, runtime.presets, 2)
		assert.Contains(t, runtime.presets, "fast")
		assert.Contains(t, runtime.presets, "creative")
	})
}

func TestRuntime_Registry(t *testing.T) {
	t.Run("returns registry instance", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		registry := runtime.Registry()

		assert.NotNil(t, registry)
		assert.IsType(t, &Registry{}, registry)
	})

	t.Run("returns same registry on multiple calls", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		registry1 := runtime.Registry()
		registry2 := runtime.Registry()

		assert.Same(t, registry1, registry2)
	})
}

func TestRuntime_WorkingDir(t *testing.T) {
	t.Run("returns working directory", func(t *testing.T) {
		workDir := t.TempDir()
		runtime := NewRuntime(workDir)

		assert.Equal(t, workDir, runtime.WorkingDir())
	})
}

func TestRuntime_SetOutputService(t *testing.T) {
	t.Run("sets output service", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		mockOutput := &mockOutputWriter{}
		runtime.SetOutputService(mockOutput)

		assert.Same(t, mockOutput, runtime.outputService)
	})

	t.Run("replaces existing output service", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())

		mock1 := &mockOutputWriter{}
		mock2 := &mockOutputWriter{}

		runtime.SetOutputService(mock1)
		runtime.SetOutputService(mock2)

		assert.Same(t, mock2, runtime.outputService)
	})
}

func TestRuntime_CreateModulesForCtx(t *testing.T) {
	tests := []struct {
		name   string
		create func(*Runtime) interface{}
	}{
		{"CreateFSModuleForCtx", func(r *Runtime) interface{} { return r.CreateFSModuleForCtx() }},
		{"CreateGitModuleForCtx", func(r *Runtime) interface{} { return r.CreateGitModuleForCtx() }},
		{"CreateLLMModuleForCtx", func(r *Runtime) interface{} { return r.CreateLLMModuleForCtx(nil) }},
		{"CreateShellModuleForCtx", func(r *Runtime) interface{} { return r.CreateShellModuleForCtx() }},
		{"CreateIndexModuleForCtx", func(r *Runtime) interface{} { return r.CreateIndexModuleForCtx() }},
		{"CreateStdinModuleForCtx", func(r *Runtime) interface{} { return r.CreateStdinModuleForCtx() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := NewRuntime(t.TempDir())
			module := tt.create(runtime)

			assert.NotNil(t, module)
		})
	}
}

func TestRuntime_CreateOutputModuleForCtx(t *testing.T) {
	t.Run("returns noop output module when outputService is nil", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		// outputService is nil by default

		module := runtime.CreateOutputModuleForCtx()

		assert.NotNil(t, module)
	})

	t.Run("returns output module with set outputService", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		mockOutput := &mockOutputWriter{}
		runtime.SetOutputService(mockOutput)

		module := runtime.CreateOutputModuleForCtx()

		assert.NotNil(t, module)
	})
}

func TestNoopOutputWriter(t *testing.T) {
	t.Run("Print returns no error", func(t *testing.T) {
		writer := &noopOutputWriter{}
		err := writer.Print("test content")
		assert.NoError(t, err)
	})

	t.Run("PrintLine returns no error", func(t *testing.T) {
		writer := &noopOutputWriter{}
		err := writer.PrintLine("test line")
		assert.NoError(t, err)
	})

	t.Run("Printf returns no error", func(t *testing.T) {
		writer := &noopOutputWriter{}
		err := writer.Printf("format %s %d", "string", 42)
		assert.NoError(t, err)
	})

	t.Run("PrintMarkdown returns no error", func(t *testing.T) {
		writer := &noopOutputWriter{}
		err := writer.PrintMarkdown("# Markdown\n**bold**")
		assert.NoError(t, err)
	})

	t.Run("StreamMarkdown returns no error", func(t *testing.T) {
		writer := &noopOutputWriter{}
		err := writer.StreamMarkdown("streaming content", false)
		assert.NoError(t, err)

		err = writer.StreamMarkdown("final content", true)
		assert.NoError(t, err)
	})

	t.Run("all methods can be called multiple times", func(t *testing.T) {
		writer := &noopOutputWriter{}

		for i := 0; i < 10; i++ {
			assert.NoError(t, writer.Print("test"))
			assert.NoError(t, writer.PrintLine("test"))
			assert.NoError(t, writer.Printf("test %d", i))
			assert.NoError(t, writer.PrintMarkdown("test"))
			assert.NoError(t, writer.StreamMarkdown("test", i%2 == 0))
		}
	})
}

// mockOutputWriter is a test implementation of OutputWriter
type mockOutputWriter struct {
	printed []string
}

func (m *mockOutputWriter) Print(content string) error {
	m.printed = append(m.printed, content)
	return nil
}

func (m *mockOutputWriter) PrintLine(content string) error {
	m.printed = append(m.printed, content+"\n")
	return nil
}

func (m *mockOutputWriter) Printf(format string, args ...any) error {
	return nil
}

func (m *mockOutputWriter) PrintMarkdown(content string) error {
	m.printed = append(m.printed, content)
	return nil
}

func (m *mockOutputWriter) StreamMarkdown(content string, done bool) error {
	m.printed = append(m.printed, content)
	return nil
}
