// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package editfile

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

func TestEditFileActivity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "meow-editfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\ntarget\nline3"
	err = os.WriteFile(filePath, []byte(initialContent), 0o644)
	assert.NoError(t, err)

	mockWS := new(MockWorkspaceService)
	mockWS.On("Get").Return(tmpDir, nil)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test_flow", executor.NoOpFeedbackHandler, exec)

	t.Run("ApplyEdit", func(t *testing.T) {
		factory := NewFactory(mockWS, false)
		act := factory.NewActivity()
		input := &Input{
			Path:      "test.txt",
			OldString: "target",
			NewString: "replaced",
		}
		output, err := act(context.Background(), flowCtx, input)
		if assert.NoError(t, err) {
			assert.True(t, output.Applied)
		}

		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "line1\nreplaced\nline3", string(content))
	})

	t.Run("NotFound", func(t *testing.T) {
		factory := NewFactory(mockWS, false)
		act := factory.NewActivity()
		input := &Input{
			Path:      "test.txt",
			OldString: "missing",
			NewString: "whatever",
		}
		_, err := act(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Ambiguous", func(t *testing.T) {
		ambiguousFile := filepath.Join(tmpDir, "ambiguous.txt")
		err = os.WriteFile(ambiguousFile, []byte("target\ntarget"), 0o644)
		assert.NoError(t, err)

		factory := NewFactory(mockWS, false)
		act := factory.NewActivity()
		input := &Input{
			Path:      "ambiguous.txt",
			OldString: "target",
			NewString: "replaced",
		}
		_, err := act(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ambiguous")
	})

	t.Run("EmptyOldString", func(t *testing.T) {
		factory := NewFactory(mockWS, false)
		act := factory.NewActivity()
		input := &Input{
			Path:      "test.txt",
			OldString: "",
			NewString: "replaced",
		}
		_, err := act(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "old_string")
	})

	t.Run("DryRun", func(t *testing.T) {
		// Reset file
		err = os.WriteFile(filePath, []byte(initialContent), 0o644)
		assert.NoError(t, err)

		factory := NewFactory(mockWS, true)
		act := factory.NewActivity()
		input := &Input{
			Path:      "test.txt",
			OldString: "target",
			NewString: "replaced",
		}
		output, err := act(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.False(t, output.Applied)
		assert.Contains(t, output.Message, "Dry run")

		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, initialContent, string(content))
	})
}
