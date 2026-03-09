// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCommand creates a cobra command with all the expected flags for testing.
func createTestCommand(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use: name,
		Run: func(cmd *cobra.Command, args []string) {},
	}

	cmd.Flags().String("workspace", "/default/workspace", "workspace path")
	cmd.Flags().String("task", "default-task", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("no-cache", false, "disable cache")
	cmd.Flags().Bool("update-cache", false, "update cache")
	cmd.Flags().Bool("no-tui", false, "disable TUI")

	return cmd
}

// TestNewService tests creating a new command service.
func TestNewService(t *testing.T) {
	t.Run("with valid command", func(t *testing.T) {
		cmd := createTestCommand("test")
		service, err := NewService(cmd)

		// Note: This may fail if stdin is piped in test environment
		// In a real CI/CD environment, stdin.Stat() should work
		if err != nil {
			// If we get an error, it should be about stdin
			assert.Contains(t, err.Error(), "stdin")
			return
		}

		require.NotNil(t, service)
		assert.NotNil(t, service.cmd)
	})

	t.Run("with nil command", func(t *testing.T) {
		service, err := NewService(nil)
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "command cannot be nil")
	})
}

// TestGetCommandName tests retrieving command name.
func TestGetCommandName(t *testing.T) {
	t.Run("valid service", func(t *testing.T) {
		cmd := createTestCommand("my-command")
		service := &Service{cmd: cmd}

		name, err := service.GetCommandName()
		require.NoError(t, err)
		assert.Equal(t, "my-command", name)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		name, err := service.GetCommandName()
		assert.Error(t, err)
		assert.Empty(t, name)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		name, err := service.GetCommandName()
		assert.Error(t, err)
		assert.Empty(t, name)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestGetWorkspacePath tests retrieving workspace path.
func TestGetWorkspacePath(t *testing.T) {
	t.Run("default workspace", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		path, err := service.GetWorkspacePath()
		require.NoError(t, err)
		assert.Equal(t, "/default/workspace", path)
	})

	t.Run("custom workspace", func(t *testing.T) {
		cmd := createTestCommand("test")
		cmd.Flags().Set("workspace", "/custom/path")
		service := &Service{cmd: cmd}

		path, err := service.GetWorkspacePath()
		require.NoError(t, err)
		assert.Equal(t, "/custom/path", path)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		path, err := service.GetWorkspacePath()
		assert.Error(t, err)
		assert.Empty(t, path)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		path, err := service.GetWorkspacePath()
		assert.Error(t, err)
		assert.Empty(t, path)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestGetTaskName tests retrieving task name.
func TestGetTaskName(t *testing.T) {
	t.Run("default task", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		task, err := service.GetTaskName()
		require.NoError(t, err)
		assert.Equal(t, "default-task", task)
	})

	t.Run("custom task", func(t *testing.T) {
		cmd := createTestCommand("test")
		cmd.Flags().Set("task", "custom-task")
		service := &Service{cmd: cmd}

		task, err := service.GetTaskName()
		require.NoError(t, err)
		assert.Equal(t, "custom-task", task)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		task, err := service.GetTaskName()
		assert.Error(t, err)
		assert.Empty(t, task)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		task, err := service.GetTaskName()
		assert.Error(t, err)
		assert.Empty(t, task)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestGetUserPrompt tests retrieving user prompt.
func TestGetUserPrompt(t *testing.T) {
	t.Run("default empty prompt", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		prompt, err := service.GetUserPrompt()
		require.NoError(t, err)
		assert.Empty(t, prompt)
	})

	t.Run("custom prompt", func(t *testing.T) {
		cmd := createTestCommand("test")
		cmd.Flags().Set("user-prompt", "Write a test")
		service := &Service{cmd: cmd}

		prompt, err := service.GetUserPrompt()
		require.NoError(t, err)
		assert.Equal(t, "Write a test", prompt)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		prompt, err := service.GetUserPrompt()
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		prompt, err := service.GetUserPrompt()
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestGetNoTUIFlag tests retrieving no-tui flag.
func TestGetNoTUIFlag(t *testing.T) {
	t.Run("default false", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		noTUI, err := service.GetNoTUIFlag()
		require.NoError(t, err)
		assert.False(t, noTUI)
	})

	t.Run("set to true", func(t *testing.T) {
		cmd := createTestCommand("test")
		cmd.Flags().Set("no-tui", "true")
		service := &Service{cmd: cmd}

		noTUI, err := service.GetNoTUIFlag()
		require.NoError(t, err)
		assert.True(t, noTUI)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		noTUI, err := service.GetNoTUIFlag()
		assert.Error(t, err)
		assert.False(t, noTUI)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		noTUI, err := service.GetNoTUIFlag()
		assert.Error(t, err)
		assert.False(t, noTUI)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestMultipleFlagRetrievals tests getting multiple flags in sequence.
func TestGetStdIn(t *testing.T) {
	t.Run("empty stdin when not piped", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		// When stdin is a TTY (not piped), GetStdIn should return empty string.
		// In the test environment stdin is typically not a pipe, so stdinCached
		// stays "" and no error is returned.
		stdin, err := service.GetStdIn()
		require.NoError(t, err)
		assert.Empty(t, stdin)
	})

	t.Run("with piped stdin content", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)

		_, err = w.WriteString("some input")
		require.NoError(t, err)
		w.Close()

		old := os.Stdin
		os.Stdin = r
		defer func() {
			r.Close()
			os.Stdin = old
		}()

		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		stdin, err := service.GetStdIn()
		require.NoError(t, err)
		assert.Equal(t, "some input", stdin)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		stdin, err := service.GetStdIn()
		assert.Error(t, err)
		assert.Empty(t, stdin)
		assert.Contains(t, err.Error(), "service is nil")
	})
}

// TestGetNoCacheFlag tests retrieving no-cache flag.
func TestGetNoCacheFlag(t *testing.T) {
	t.Run("default false", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		noCache, err := service.GetNoCacheFlag()
		require.NoError(t, err)
		assert.False(t, noCache)
	})

	t.Run("set to true", func(t *testing.T) {
		cmd := createTestCommand("test")
		cmd.Flags().Set("no-cache", "true")
		service := &Service{cmd: cmd}

		noCache, err := service.GetNoCacheFlag()
		require.NoError(t, err)
		assert.True(t, noCache)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		noCache, err := service.GetNoCacheFlag()
		assert.Error(t, err)
		assert.False(t, noCache)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		noCache, err := service.GetNoCacheFlag()
		assert.Error(t, err)
		assert.False(t, noCache)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestGetUpdateCacheFlag tests retrieving update-cache flag.
func TestGetUpdateCacheFlag(t *testing.T) {
	t.Run("default false", func(t *testing.T) {
		cmd := createTestCommand("test")
		service := &Service{cmd: cmd}

		updateCache, err := service.GetUpdateCacheFlag()
		require.NoError(t, err)
		assert.False(t, updateCache)
	})

	t.Run("set to true", func(t *testing.T) {
		cmd := createTestCommand("test")
		cmd.Flags().Set("update-cache", "true")
		service := &Service{cmd: cmd}

		updateCache, err := service.GetUpdateCacheFlag()
		require.NoError(t, err)
		assert.True(t, updateCache)
	})

	t.Run("nil service", func(t *testing.T) {
		var service *Service
		updateCache, err := service.GetUpdateCacheFlag()
		assert.Error(t, err)
		assert.False(t, updateCache)
		assert.Contains(t, err.Error(), "service is nil")
	})

	t.Run("nil command", func(t *testing.T) {
		service := &Service{cmd: nil}
		updateCache, err := service.GetUpdateCacheFlag()
		assert.Error(t, err)
		assert.False(t, updateCache)
		assert.Contains(t, err.Error(), "command is nil")
	})
}

// TestMultipleFlagRetrievals tests getting multiple flags in sequence.
func TestMultipleFlagRetrievals(t *testing.T) {
	cmd := createTestCommand("test")
	cmd.Flags().Set("workspace", "/my/workspace")
	cmd.Flags().Set("task", "my-task")
	cmd.Flags().Set("no-tui", "true")
	cmd.Flags().Set("no-cache", "true")

	service := &Service{cmd: cmd}

	// Get workspace
	workspace, err := service.GetWorkspacePath()
	require.NoError(t, err)
	assert.Equal(t, "/my/workspace", workspace)

	// Get task
	task, err := service.GetTaskName()
	require.NoError(t, err)
	assert.Equal(t, "my-task", task)

	// Get no-tui
	noTUI, err := service.GetNoTUIFlag()
	require.NoError(t, err)
	assert.True(t, noTUI)

	// Get no-cache
	noCache, err := service.GetNoCacheFlag()
	require.NoError(t, err)
	assert.True(t, noCache)

	// GetStdIn returns empty when stdin is not piped (TTY in test environment)
	stdin, err := service.GetStdIn()
	require.NoError(t, err)
	assert.Empty(t, stdin)

	// Get command name
	name, err := service.GetCommandName()
	require.NoError(t, err)
	assert.Equal(t, "test", name)
}
