// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package movefile

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
	root string
	err  error
}

func (s *stubWorkspaceService) Get() (string, error) {
	return s.root, s.err
}

func TestMovefileActivity_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &stubWorkspaceService{root: tmpDir}
	factory := NewFactory(ws, true)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{
		SourcePath: "a.txt",
		DestPath:   "b.txt",
	})
	require.NoError(t, err)
	assert.False(t, output.Moved)
	assert.Contains(t, output.Message, "DRY RUN")
}

func TestMovefileActivity_RenamesFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "a.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("data"), 0o644))

	ws := &stubWorkspaceService{root: tmpDir}
	factory := NewFactory(ws, false)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{
		SourcePath: "a.txt",
		DestPath:   "dest/b.txt",
	})
	require.NoError(t, err)
	assert.True(t, output.Moved)
	assert.Contains(t, output.Message, "Moved")

	_, statErr := os.Stat(sourcePath)
	assert.Error(t, statErr)
	assert.True(t, os.IsNotExist(statErr))
	_, statErr = os.Stat(filepath.Join(tmpDir, "dest", "b.txt"))
	assert.NoError(t, statErr)
}

func TestMovefileActivity_MissingSource(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &stubWorkspaceService{root: tmpDir}
	factory := NewFactory(ws, false)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	_, err := activity(context.Background(), flowCtx, &Input{
		SourcePath: "missing.txt",
		DestPath:   "dest/b.txt",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source file not found")
}

func TestResolveAndValidatePath(t *testing.T) {
	tmpDir := t.TempDir()

	full, clean, err := resolveAndValidatePath(tmpDir, "a/b.txt")
	require.NoError(t, err)
	assert.Equal(t, "a/b.txt", clean)
	assert.True(t, filepath.IsAbs(full))

	_, _, err = resolveAndValidatePath(tmpDir, "../outside.txt")
	assert.Error(t, err)
}
