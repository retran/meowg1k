/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package liststaged

import (
	"context"
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
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}
