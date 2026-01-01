// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package readfile

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

func TestReadfileActivity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "meow-readfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	err = os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err)

	mockWS := new(MockWorkspaceService)
	mockWS.On("Get").Return(tmpDir, nil)

	factory := NewFactory(mockWS)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test_flow", executor.NoOpFeedbackHandler, exec)

	t.Run("ReadAll", func(t *testing.T) {
		input := &Input{
			Path: "test.txt",
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.Equal(t, content, output.Content)
		assert.False(t, output.IsTruncated)
		assert.Equal(t, 5, output.TotalLines)
	})

	t.Run("ReadRange", func(t *testing.T) {
		input := &Input{
			Path:      "test.txt",
			StartLine: 2,
			EndLine:   4,
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		expected := "line2\nline3\nline4"
		assert.Equal(t, expected, output.Content)
		assert.True(t, output.IsTruncated)
		assert.Equal(t, 5, output.TotalLines)
	})

	t.Run("ReadOutOfBounds", func(t *testing.T) {
		input := &Input{
			Path:      "test.txt",
			StartLine: 10,
		}
		output, err := activity(context.Background(), flowCtx, input)
		assert.NoError(t, err)
		assert.Equal(t, "", output.Content)
		assert.True(t, output.IsTruncated)
	})

	t.Run("PathTraversal", func(t *testing.T) {
		input := &Input{
			Path: "../outside.txt",
		}
		_, err := activity(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})

	t.Run("PathTraversalPrefixBypass", func(t *testing.T) {
		// Old strings.HasPrefix(root) checks can be bypassed by joining a path that
		// escapes to a sibling directory whose name shares the same prefix.
		rootBase := filepath.Base(tmpDir)
		outsideDir := filepath.Join(filepath.Dir(tmpDir), rootBase+"_evil")
		err := os.MkdirAll(outsideDir, 0755)
		assert.NoError(t, err)
		outsideFile := filepath.Join(outsideDir, "outside.txt")
		err = os.WriteFile(outsideFile, []byte("nope"), 0644)
		assert.NoError(t, err)

		input := &Input{Path: "../" + rootBase + "_evil/outside.txt"}
		_, err = activity(context.Background(), flowCtx, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})
}
