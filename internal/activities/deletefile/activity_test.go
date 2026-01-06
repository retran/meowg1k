// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package deletefile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/pkg/executor"
)

type stubWorkspaceService struct {
	err  error
	root string
}

func (s *stubWorkspaceService) Get() (string, error) {
	return s.root, s.err
}

func TestDeletefileActivity_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &stubWorkspaceService{root: tmpDir}

	factory := NewFactory(ws, true)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{Path: "file.txt"})
	require.NoError(t, err)
	assert.False(t, output.Deleted)
	assert.Contains(t, output.Message, "DRY RUN")
}

func TestDeletefileActivity_DeletesFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o644))

	ws := &stubWorkspaceService{root: tmpDir}
	factory := NewFactory(ws, false)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{Path: "file.txt"})
	require.NoError(t, err)
	assert.True(t, output.Deleted)
	assert.Contains(t, output.Message, "Deleted")

	_, statErr := os.Stat(filePath)
	assert.Error(t, statErr)
	assert.True(t, os.IsNotExist(statErr))
}

func TestResolveAndValidatePath(t *testing.T) {
	tmpDir := t.TempDir()

	full, clean, err := resolveAndValidatePath(tmpDir, "a/b.txt")
	require.NoError(t, err)
	assert.Equal(t, "a/b.txt", clean)
	assert.True(t, filepath.IsAbs(full))

	_, _, err = resolveAndValidatePath(tmpDir, "")
	assert.Error(t, err)

	_, _, err = resolveAndValidatePath(tmpDir, "/abs.txt")
	assert.Error(t, err)

	_, _, err = resolveAndValidatePath(tmpDir, "../outside.txt")
	assert.Error(t, err)
}
