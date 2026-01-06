// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package fetchstageddiff

import (
	"context"
	"fmt"
	"testing"

	"github.com/retran/meowg1k/pkg/executor"
)

const (
	newContentValue      = "new content"
	oldContentValue      = "old content"
	newFileContentValue  = "new file content"
	deletedDiffValue     = "diff for deleted file"
	originalContentValue = "original content"
)

// mockStagedChangesReader is a mock implementation of StagedChangesReader for testing.
type mockStagedChangesReader struct {
	ReadStagedChangesFunc       func(filename string) (string, error)
	ReadOriginalFileContentFunc func(filename string) (string, error)
	ReadStagedFileContentFunc   func(filename string) (string, error)
}

func (m *mockStagedChangesReader) ReadStagedChanges(filename string) (string, error) {
	if m.ReadStagedChangesFunc != nil {
		return m.ReadStagedChangesFunc(filename)
	}
	return "", nil
}

func (m *mockStagedChangesReader) ReadOriginalFileContent(filename string) (string, error) {
	if m.ReadOriginalFileContentFunc != nil {
		return m.ReadOriginalFileContentFunc(filename)
	}
	return "", nil
}

func (m *mockStagedChangesReader) ReadStagedFileContent(filename string) (string, error) {
	if m.ReadStagedFileContentFunc != nil {
		return m.ReadStagedFileContentFunc(filename)
	}
	return "", nil
}

func TestNewFactory(t *testing.T) {
	gitSvc := &mockStagedChangesReader{}
	factory, err := NewFactory(gitSvc)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestActivityNilInput(t *testing.T) {
	gitSvc := &mockStagedChangesReader{}
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

func TestActivitySuccess(t *testing.T) {
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filePath string) (string, error) {
			return "diff content", nil
		},
		ReadStagedFileContentFunc: func(filePath string) (string, error) {
			return newContentValue, nil
		},
		ReadOriginalFileContentFunc: func(filePath string) (string, error) {
			return oldContentValue, nil
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
		Filename: "test.go",
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	if output.Filename != "test.go" {
		t.Errorf("Expected filename 'test.go', got '%s'", output.Filename)
	}

	if output.Change != "diff content" {
		t.Errorf("Expected change 'diff content', got '%s'", output.Change)
	}
}

func TestNewFactory_NilStagedChangesReader(t *testing.T) {
	_, err := NewFactory(nil)
	if err == nil {
		t.Fatal("expected error for nil staged changes reader, got nil")
	}
	expectedMsg := "staged changes reader cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewActivity_NilFactory(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "test.go"}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error for nil factory, got nil")
	}
	expectedMsg := "fetch file diff factory is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestActivity_ReadStagedChangesError(t *testing.T) {
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filename string) (string, error) {
			return "", fmt.Errorf("git error")
		},
	}
	factory, _ := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "test.go"}

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Fatal("expected error when ReadStagedChanges fails, got nil")
	}
}

func TestActivity_NewFileScenario(t *testing.T) {
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filename string) (string, error) {
			return "diff for new file", nil
		},
		ReadOriginalFileContentFunc: func(filename string) (string, error) {
			return "", fmt.Errorf("does not exist in 'HEAD'")
		},
		ReadStagedFileContentFunc: func(filename string) (string, error) {
			return newFileContentValue, nil
		},
	}
	factory, _ := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "newfile.go"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.OriginalFileContent != "" {
		t.Errorf("expected empty original content for new file, got %q", output.OriginalFileContent)
	}
	if output.ChangedFileContent != newFileContentValue {
		t.Errorf("expected staged content, got %q", output.ChangedFileContent)
	}
}

func TestActivity_DeletedFileScenario(t *testing.T) {
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filename string) (string, error) {
			return deletedDiffValue, nil
		},
		ReadOriginalFileContentFunc: func(filename string) (string, error) {
			return originalContentValue, nil
		},
		ReadStagedFileContentFunc: func(filename string) (string, error) {
			return "", fmt.Errorf("does not exist")
		},
	}
	factory, _ := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "deleted.go"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ChangedFileContent != "" {
		t.Errorf("expected empty staged content for deleted file, got %q", output.ChangedFileContent)
	}
	if output.OriginalFileContent != originalContentValue {
		t.Errorf("expected original content, got %q", output.OriginalFileContent)
	}
}

func TestActivity_DeletedFileWithUnknownRevisionError(t *testing.T) {
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filename string) (string, error) {
			return deletedDiffValue, nil
		},
		ReadOriginalFileContentFunc: func(filename string) (string, error) {
			return originalContentValue, nil
		},
		ReadStagedFileContentFunc: func(filename string) (string, error) {
			return "", fmt.Errorf("fatal: ambiguous argument '%s': unknown revision or path not in the working tree", filename)
		},
	}
	factory, _ := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: ".meowg1k/config.yaml"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ChangedFileContent != "" {
		t.Errorf("expected empty staged content for deleted file, got %q", output.ChangedFileContent)
	}
	if output.OriginalFileContent != originalContentValue {
		t.Errorf("expected original content, got %q", output.OriginalFileContent)
	}
	if output.Change != deletedDiffValue {
		t.Errorf("expected diff content, got %q", output.Change)
	}
}

func TestActivity_InitialCommitScenario(t *testing.T) {
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filename string) (string, error) {
			return "diff for new file in initial commit", nil
		},
		ReadOriginalFileContentFunc: func(filename string) (string, error) {
			return "", fmt.Errorf("fatal: invalid object name 'HEAD'")
		},
		ReadStagedFileContentFunc: func(filename string) (string, error) {
			return newFileContentValue, nil
		},
	}
	factory, _ := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "initial.go"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error for initial commit: %v", err)
	}

	if output.OriginalFileContent != "" {
		t.Errorf("expected empty original content for initial commit, got %q", output.OriginalFileContent)
	}
	if output.ChangedFileContent != newFileContentValue {
		t.Errorf("expected staged content, got %q", output.ChangedFileContent)
	}
}

func TestActivity_RenameScenario(t *testing.T) {
	var originalRequested string
	gitSvc := &mockStagedChangesReader{
		ReadStagedChangesFunc: func(filename string) (string, error) {
			return "diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go", nil
		},
		ReadOriginalFileContentFunc: func(filename string) (string, error) {
			originalRequested = filename
			return oldContentValue, nil
		},
		ReadStagedFileContentFunc: func(filename string) (string, error) {
			return newContentValue, nil
		},
	}

	factory, _ := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	input := &Input{Filename: "new.go"}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("unexpected error for rename: %v", err)
	}

	if originalRequested != "old.go" {
		t.Errorf("expected original content to be read from old.go, got %s", originalRequested)
	}
	if output.Filename != "new.go" {
		t.Errorf("expected filename to remain new path, got %s", output.Filename)
	}
	if output.OriginalFileContent != oldContentValue || output.ChangedFileContent != newContentValue {
		t.Errorf("unexpected content fetched: original=%q staged=%q", output.OriginalFileContent, output.ChangedFileContent)
	}
	if output.Change == "" || output.Change == "\n" {
		t.Errorf("expected change to be propagated, got %q", output.Change)
	}
}
