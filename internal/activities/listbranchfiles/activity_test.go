// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package listbranchfiles

import (
	"context"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/pkg/executor"
)

// mockBranchFileListReader is a mock implementation of BranchFileListReader for testing.
type mockBranchFileListReader struct {
	GetChangedFilesInBranchFunc func(targetBranch string) ([]string, error)
}

func (m *mockBranchFileListReader) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
	if m.GetChangedFilesInBranchFunc != nil {
		return m.GetChangedFilesInBranchFunc(targetBranch)
	}
	return nil, nil
}

func TestNewFactory(t *testing.T) {
	mockReader := &mockBranchFileListReader{}
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
	mockReader := &mockBranchFileListReader{}
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
	gitSvc := &mockBranchFileListReader{
		GetChangedFilesInBranchFunc: func(targetBranch string) ([]string, error) {
			return []string{"file1.go", "file2.go"}, nil
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
		TargetBranch: "main",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	if len(output.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(output.Files))
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{TargetBranch: "main"}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "list branch files factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_EmptyTargetBranch(t *testing.T) {
	mockReader := &mockBranchFileListReader{}
	factory, _ := NewFactory(mockReader)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{TargetBranch: ""}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for empty target branch, got nil")
	}
	expectedMsg := "target branch cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_GetChangedFilesError(t *testing.T) {
	mockReader := &mockBranchFileListReader{
		GetChangedFilesInBranchFunc: func(targetBranch string) ([]string, error) {
			return nil, fmt.Errorf("git error")
		},
	}
	factory, _ := NewFactory(mockReader)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{TargetBranch: "main"}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error when GetChangedFilesInBranch fails, got nil")
	}
}

func TestActivity_EmptyFileList(t *testing.T) {
	mockReader := &mockBranchFileListReader{
		GetChangedFilesInBranchFunc: func(targetBranch string) ([]string, error) {
			return []string{}, nil
		},
	}
	factory, _ := NewFactory(mockReader)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{TargetBranch: "main"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(output.Files))
	}
}
