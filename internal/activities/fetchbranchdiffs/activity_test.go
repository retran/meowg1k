// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package fetchbranchdiffs

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/fetchbranchdiff"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockBranchFileDiffActivityFactory is a mock implementation of BranchFileDiffActivityFactory for testing.
type mockBranchFileDiffActivityFactory struct {
	activity executor.Activity[*fetchbranchdiff.Input, *git.FileChange]
}

func (m *mockBranchFileDiffActivityFactory) NewActivity() executor.Activity[*fetchbranchdiff.Input, *git.FileChange] {
	return m.activity
}

func TestNewFactory(t *testing.T) {
	mockFactory := &mockBranchFileDiffActivityFactory{}
	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestActivityNilInput(t *testing.T) {
	mockFactory := &mockBranchFileDiffActivityFactory{}
	factory, err := NewFactory(mockFactory)
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
	mockActivity := func(ctx context.Context, executorCtx *executor.Context, input *fetchbranchdiff.Input) (*git.FileChange, error) {
		return &git.FileChange{Filename: input.Filename}, nil
	}

	mockFactory := &mockBranchFileDiffActivityFactory{
		activity: mockActivity,
	}

	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Files:        []string{"file1.go", "file2.go"},
		TargetBranch: "main",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(output.Changes) == 0 {
		t.Error("Expected changes, got empty array")
	}
}
