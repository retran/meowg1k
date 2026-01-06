// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package fetchbranchdiff

import (
	"context"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/pkg/executor"
)

// mockBranchDiffReader is a mock implementation of BranchDiffReader for testing.
type mockBranchDiffReader struct {
	GetBranchDiffFunc            func(filePath, targetBranch string) (string, error)
	GetBranchDiffWithOldPathFunc func(filePath, targetBranch, oldPath string) (string, error)
	ReadOriginalFileContentFunc  func(filename string) (string, error)
	ReadStagedFileContentFunc    func(filename string) (string, error)
}

func (m *mockBranchDiffReader) GetBranchDiff(filePath, targetBranch string) (string, error) {
	if m.GetBranchDiffFunc != nil {
		return m.GetBranchDiffFunc(filePath, targetBranch)
	}
	return "", nil
}

func (m *mockBranchDiffReader) GetBranchDiffWithOldPath(filePath, targetBranch, oldPath string) (string, error) {
	if m.GetBranchDiffWithOldPathFunc != nil {
		return m.GetBranchDiffWithOldPathFunc(filePath, targetBranch, oldPath)
	}
	return "", nil
}

func (m *mockBranchDiffReader) ReadOriginalFileContent(filename string) (string, error) {
	if m.ReadOriginalFileContentFunc != nil {
		return m.ReadOriginalFileContentFunc(filename)
	}
	return "", nil
}

func (m *mockBranchDiffReader) ReadStagedFileContent(filename string) (string, error) {
	if m.ReadStagedFileContentFunc != nil {
		return m.ReadStagedFileContentFunc(filename)
	}
	return "", nil
}

func TestNewFactory(t *testing.T) {
	mockReader := &mockBranchDiffReader{}
	factory, err := NewFactory(mockReader)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestNewFactoryNil(t *testing.T) {
	factory, err := NewFactory(nil)
	if err == nil {
		t.Error("Expected error when NewFactory called with nil")
	}
	if factory != nil {
		t.Error("Expected nil factory when error returned")
	}
}

func TestActivityNilInput(t *testing.T) {
	mockReader := &mockBranchDiffReader{}
	factory, err := NewFactory(mockReader)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err = activity(ctx, execCtx, nil)
	if err == nil {
		t.Error("Expected error for nil input, got nil")
	}
}

func TestActivitySuccess(t *testing.T) {
	gitSvc := &mockBranchDiffReader{
		GetBranchDiffFunc: func(filePath, targetBranch string) (string, error) {
			return "diff content", nil
		},
	}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Filename:     "test.go",
		TargetBranch: "main",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	if output.Filename != "test.go" {
		t.Errorf("Expected filename 'test.go', got '%s'", output.Filename)
	}
}

func TestActivity_InitialCommitScenario(t *testing.T) {
	gitSvc := &mockBranchDiffReader{
		GetBranchDiffFunc: func(filePath, targetBranch string) (string, error) {
			return "diff for new file in initial commit", nil
		},
		ReadOriginalFileContentFunc: func(filename string) (string, error) {
			return "", fmt.Errorf("fatal: invalid object name 'HEAD'")
		},
		ReadStagedFileContentFunc: func(filename string) (string, error) {
			return "new file content", nil
		},
	}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{
		Filename:     "initial.go",
		TargetBranch: "main",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error for initial commit: %v", err)
	}

	if output.OriginalFileContent != "" {
		t.Errorf("expected empty original content for initial commit, got %q", output.OriginalFileContent)
	}
	if output.ChangedFileContent != "new file content" {
		t.Errorf("expected staged content, got %q", output.ChangedFileContent)
	}
}
