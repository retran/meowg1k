// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package writefile

import (
	"context"
	"os"
	"path/filepath"
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

func TestWriteFileActivity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "meow-writefile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mockWS := new(MockWorkspaceService)
	mockWS.On("Get").Return(tmpDir, nil)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test_flow", executor.NoOpFeedbackHandler, exec)

	t.Run("Write", func(t *testing.T) {
		factory := NewFactory(mockWS, false)
		activity := factory.NewActivity()
		input := &Input{
			Path:    "newfile.txt",
			Content: "hello world",
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.True(t, output.Written)

		content, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("DryRun", func(t *testing.T) {
		factory := NewFactory(mockWS, true)
		activity := factory.NewActivity()
		input := &Input{
			Path:    "dryrun.txt",
			Content: "should not exist",
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.False(t, output.Written)
		assert.Contains(t, output.Message, "Dry run")

		_, err = os.Stat(filepath.Join(tmpDir, "dryrun.txt"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("PathTraversal", func(t *testing.T) {
		factory := NewFactory(mockWS, false)
		activity := factory.NewActivity()
		input := &Input{
			Path: "../outside.txt",
		}
		_, err := activity(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})

	t.Run("PathTraversalPrefixBypass", func(t *testing.T) {
		rootBase := filepath.Base(tmpDir)
		outsideDir := filepath.Join(filepath.Dir(tmpDir), rootBase+"_evil")
		err := os.MkdirAll(outsideDir, 0o755)
		assert.NoError(t, err)

		factory := NewFactory(mockWS, false)
		activity := factory.NewActivity()
		input := &Input{Path: "../" + rootBase + "_evil/outside.txt", Content: "nope"}
		_, err = activity(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})
}
