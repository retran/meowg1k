// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package getdiff

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/pkg/executor"
)

type mockGitToolingService struct {
	err      error
	lastRef  string
	lastPath string
	diff     string
}

func (m *mockGitToolingService) Status() (string, error) { return "", nil }
func (m *mockGitToolingService) Diff(ref, path string) (string, error) {
	m.lastRef, m.lastPath = ref, path
	return m.diff, m.err
}
func (m *mockGitToolingService) Show(ref string) (string, error)            { return "", nil }
func (m *mockGitToolingService) Log(limit int, path string) (string, error) { return "", nil }
func (m *mockGitToolingService) Branches() ([]string, error)                { return nil, nil }
func (m *mockGitToolingService) CurrentBranch() (string, error)             { return "", nil }
func (m *mockGitToolingService) Stage(paths []string) (string, error)       { return "", nil }
func (m *mockGitToolingService) Commit(message string) (string, error)      { return "", nil }
func (m *mockGitToolingService) HeadHash() (string, error)                  { return "", nil }

func TestGetDiffActivity_Workdir(t *testing.T) {
	mockGit := &mockGitToolingService{diff: "diff content"}
	factory := NewFactory(mockGit)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{Staged: false})
	require.NoError(t, err)
	assert.Equal(t, "diff content", output.Diff)
	assert.Equal(t, "", mockGit.lastRef)
	assert.Equal(t, "", mockGit.lastPath)
}

func TestGetDiffActivity_Staged(t *testing.T) {
	mockGit := &mockGitToolingService{diff: "staged diff"}
	factory := NewFactory(mockGit)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{Staged: true})
	require.NoError(t, err)
	assert.Equal(t, "staged diff", output.Diff)
	assert.Equal(t, "--staged", mockGit.lastRef)
	assert.Equal(t, "", mockGit.lastPath)
}

func TestGetDiffActivity_Error(t *testing.T) {
	mockGit := &mockGitToolingService{err: errors.New("diff failed")}
	factory := NewFactory(mockGit)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	_, err := activity(context.Background(), flowCtx, &Input{Staged: false})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get diff")
}
