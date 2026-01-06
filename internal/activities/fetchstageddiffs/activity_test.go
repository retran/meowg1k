// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package fetchstageddiffs

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/fetchstageddiff"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockFileDiffActivityFactory is a mock implementation of FileDiffActivityFactory for testing.
type mockFileDiffActivityFactory struct {
	activity executor.Activity[*fetchstageddiff.Input, *git.FileChange]
}

func (m *mockFileDiffActivityFactory) NewActivity() executor.Activity[*fetchstageddiff.Input, *git.FileChange] {
	return m.activity
}

func TestNewFactory(t *testing.T) {
	mockFactory := &mockFileDiffActivityFactory{}
	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestActivityNilInput(t *testing.T) {
	mockFactory := &mockFileDiffActivityFactory{}
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
	mockActivity := func(ctx context.Context, executorCtx *executor.Context, input *fetchstageddiff.Input) (*git.FileChange, error) {
		return &git.FileChange{Filename: input.Filename}, nil
	}

	mockFactory := &mockFileDiffActivityFactory{
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
		Files: []string{"file1.go", "file2.go"},
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(output.Changes) == 0 {
		t.Error("Expected changes, got empty array")
	}
}
