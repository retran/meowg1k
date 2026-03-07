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

func TestNewLoaderService(t *testing.T) {
	t.Run("creates loader with runtime", func(t *testing.T) {
		runtime := NewRuntime(t.TempDir())
		loader := NewLoaderService(runtime)

		require.NotNil(t, loader)
		assert.NotNil(t, loader.runtime)
	})
}

func TestLoaderService_LoadAll(t *testing.T) {
	t.Run("loads system init if it exists and no project init", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		projectDir := t.TempDir()

		// Change to project directory (no .meowg1k dir)
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(projectDir)

		// Create system init.star
		systemConfigDir := filepath.Join(homeDir, ".config", "meowg1k")
		err := os.MkdirAll(systemConfigDir, 0o755)
		require.NoError(t, err)

		systemInit := filepath.Join(systemConfigDir, "init.star")
		systemContent := `
# System config
meow.provider("system-provider", type="openai", api_key="system-key")
`
		err = os.WriteFile(systemInit, []byte(systemContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(projectDir)
		loader := NewLoaderService(runtime)

		err = loader.LoadAll()
		assert.NoError(t, err)

		// System provider should be loaded since there is no project config
		assert.Contains(t, runtime.providers, "system-provider")
	})

	t.Run("loads project init if it exists", func(t *testing.T) {
		projectDir := t.TempDir()

		// Change to project directory
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(projectDir)

		// Create project init.star
		projectConfigDir := filepath.Join(projectDir, ".meowg1k")
		err := os.MkdirAll(projectConfigDir, 0o755)
		require.NoError(t, err)

		projectInit := filepath.Join(projectConfigDir, "init.star")
		scriptContent := `
# Project config
y = "project"
`
		err = os.WriteFile(projectInit, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(projectDir)
		loader := NewLoaderService(runtime)

		err = loader.LoadAll()
		assert.NoError(t, err)
	})

	t.Run("loads project init and skips system init when both exist", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		projectDir := t.TempDir()

		// Change to project directory
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(projectDir)

		// Create system init.star that registers a provider
		systemConfigDir := filepath.Join(homeDir, ".config", "meowg1k")
		err := os.MkdirAll(systemConfigDir, 0o755)
		require.NoError(t, err)

		systemInit := filepath.Join(systemConfigDir, "init.star")
		systemContent := `
# System config — should NOT be loaded
meow.provider("system-only-provider", type="openai", api_key="system-key")
`
		err = os.WriteFile(systemInit, []byte(systemContent), 0o644)
		require.NoError(t, err)

		// Create project init.star with a different provider name
		projectConfigDir := filepath.Join(projectDir, ".meowg1k")
		err = os.MkdirAll(projectConfigDir, 0o755)
		require.NoError(t, err)

		projectInit := filepath.Join(projectConfigDir, "init.star")
		projectContent := `
# Project config — should be the only config loaded
meow.provider("project-only-provider", type="anthropic", api_key="project-key")
`
		err = os.WriteFile(projectInit, []byte(projectContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(projectDir)
		loader := NewLoaderService(runtime)

		err = loader.LoadAll()
		assert.NoError(t, err)

		// Project provider must be loaded
		assert.Contains(t, runtime.providers, "project-only-provider")
		// System provider must NOT be loaded
		assert.NotContains(t, runtime.providers, "system-only-provider")
	})

	t.Run("succeeds when no init files exist", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		projectDir := t.TempDir()

		// Change to project directory (no .meowg1k dir)
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(projectDir)

		runtime := NewRuntime(projectDir)
		loader := NewLoaderService(runtime)

		err := loader.LoadAll()
		assert.NoError(t, err)
	})

	t.Run("fails when system init has errors", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		// Create system init.star with syntax error
		systemConfigDir := filepath.Join(homeDir, ".config", "meowg1k")
		err := os.MkdirAll(systemConfigDir, 0o755)
		require.NoError(t, err)

		systemInit := filepath.Join(systemConfigDir, "init.star")
		scriptContent := `
# Bad syntax
x = 
`
		err = os.WriteFile(systemInit, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(t.TempDir())
		loader := NewLoaderService(runtime)

		err = loader.LoadAll()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load system init")
	})

	t.Run("fails when project init has errors", func(t *testing.T) {
		projectDir := t.TempDir()

		// Change to project directory
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(projectDir)

		// Create project init.star with syntax error
		projectConfigDir := filepath.Join(projectDir, ".meowg1k")
		err := os.MkdirAll(projectConfigDir, 0o755)
		require.NoError(t, err)

		projectInit := filepath.Join(projectConfigDir, "init.star")
		scriptContent := `
# Bad syntax
undefined_var + 1
`
		err = os.WriteFile(projectInit, []byte(scriptContent), 0o644)
		require.NoError(t, err)

		runtime := NewRuntime(projectDir)
		loader := NewLoaderService(runtime)

		err = loader.LoadAll()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load project init")
	})
}

func TestLoaderService_GetSystemInitPath(t *testing.T) {
	t.Run("returns correct system init path", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		runtime := NewRuntime(t.TempDir())
		loader := NewLoaderService(runtime)

		path := loader.getSystemInitPath()

		expectedPath := filepath.Join(homeDir, ".config", "meowg1k", "init.star")
		assert.Equal(t, expectedPath, path)
	})
}

func TestLoaderService_GetProjectInitPath(t *testing.T) {
	t.Run("returns correct project init path", func(t *testing.T) {
		projectDir := t.TempDir()

		// Change to project directory
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(projectDir)

		runtime := NewRuntime(projectDir)
		loader := NewLoaderService(runtime)

		path := loader.getProjectInitPath()

		// Just check that the path ends with the expected suffix
		// (actual path may have /private prefix on macOS)
		assert.Contains(t, path, ".meowg1k")
		assert.Contains(t, path, "init.star")
		assert.True(t, filepath.IsAbs(path))
	})
}
