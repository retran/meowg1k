// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gitundo

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

func TestGitUndoActivity_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755))

	ws := &stubWorkspaceService{root: tmpDir}
	factory := NewFactory(ws, true)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{Path: "file.txt"})
	require.NoError(t, err)
	assert.False(t, output.Restored)
	assert.Contains(t, output.Message, "DRY RUN")
}

func TestGitUndoActivity_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &stubWorkspaceService{root: tmpDir}
	factory := NewFactory(ws, true)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	_, err := activity(context.Background(), flowCtx, &Input{Path: "file.txt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
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
