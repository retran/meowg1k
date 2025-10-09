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

package fetchfilediff

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/pkg/executor"
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
			return "new content", nil
		},
		ReadOriginalFileContentFunc: func(filePath string) (string, error) {
			return "old content", nil
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
