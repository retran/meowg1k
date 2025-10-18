// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package liststaged

import (
	"context"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/pkg/executor"
)

// mockStagedFileListReader is a mock implementation of StagedFileListReader for testing.
type mockStagedFileListReader struct {
	ReadStagedFilesFunc func() ([]string, error)
}

func (m *mockStagedFileListReader) ReadStagedFiles() ([]string, error) {
	if m.ReadStagedFilesFunc != nil {
		return m.ReadStagedFilesFunc()
	}
	return nil, nil
}

func TestNewFactory(t *testing.T) {
	gitSvc := &mockStagedFileListReader{}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestNewActivity(t *testing.T) {
	gitSvc := &mockStagedFileListReader{}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	if activity == nil {
		t.Error("NewActivity returned nil")
	}
}

func TestActivityExecute(t *testing.T) {
	gitSvc := &mockStagedFileListReader{
		ReadStagedFilesFunc: func() ([]string, error) {
			return []string{"file1.txt", "file2.go"}, nil
		},
	}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	input := &Input{}

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity execution failed: %v", err)
	}

	expected := []string{"file1.txt", "file2.go"}
	if len(output.Files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(output.Files))
	}

	for i, file := range expected {
		if i >= len(output.Files) || output.Files[i] != file {
			t.Errorf("Expected file %s at position %d, got %v", file, i, output.Files)
		}
	}
}

func TestActivityExecuteNilInput(t *testing.T) {
	gitSvc := &mockStagedFileListReader{}
	factory, err := NewFactory(gitSvc)
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

func TestNewFactory_NilStagedFileListReader(t *testing.T) {
	_, err := NewFactory(nil)
	if err == nil {
		t.Fatal("expected error for nil staged file list reader, got nil")
	}
	expectedMsg := "staged file list reader cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "list staged factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivityExecute_ReadStagedFilesError(t *testing.T) {
	gitSvc := &mockStagedFileListReader{
		ReadStagedFilesFunc: func() ([]string, error) {
			return nil, fmt.Errorf("git error")
		},
	}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	input := &Input{}
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err = activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error when ReadStagedFiles fails, got nil")
	}
}

func TestActivityExecute_EmptyFileList(t *testing.T) {
	gitSvc := &mockStagedFileListReader{
		ReadStagedFilesFunc: func() ([]string, error) {
			return []string{}, nil
		},
	}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()

	input := &Input{}
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(output.Files))
	}
}
