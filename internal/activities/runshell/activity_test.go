// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package runshell

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/retran/meowg1k/pkg/executor"
)

type MockWorkspaceService struct {
	mock.Mock
}

func (m *MockWorkspaceService) Get() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func TestRunShellActivity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "meow-runshell-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mockWS := new(MockWorkspaceService)
	mockWS.On("Get").Return(tmpDir, nil)

	factory := NewFactory(mockWS)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test_flow", executor.NoOpFeedbackHandler, exec)

	t.Run("Echo", func(t *testing.T) {
		input := &Input{
			Command: "echo",
			Args:    []string{"hello", "world"},
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.Equal(t, "hello world", strings.TrimSpace(output.Stdout))
		assert.Equal(t, "", output.Stderr)
		assert.Equal(t, 0, output.ExitCode)
	})

	t.Run("ExitCode", func(t *testing.T) {
		// Use sh -c to simulate exit code
		input := &Input{
			Command: "sh",
			Args:    []string{"-c", "exit 42"},
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.Equal(t, 42, output.ExitCode)
	})

	t.Run("Stderr", func(t *testing.T) {
		input := &Input{
			Command: "sh",
			Args:    []string{"-c", "echo 'error message' >&2"},
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.Equal(t, "error message", strings.TrimSpace(output.Stderr))
	})
}
