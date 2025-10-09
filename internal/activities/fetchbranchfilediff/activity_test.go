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

package fetchbranchfilediff

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/pkg/executor"
)

// mockBranchDiffReader is a mock implementation of BranchDiffReader for testing.
type mockBranchDiffReader struct {
	GetBranchDiffFunc           func(filePath, targetBranch string) (string, error)
	ReadOriginalFileContentFunc func(filename string) (string, error)
	ReadStagedFileContentFunc   func(filename string) (string, error)
}

func (m *mockBranchDiffReader) GetBranchDiff(filePath, targetBranch string) (string, error) {
	if m.GetBranchDiffFunc != nil {
		return m.GetBranchDiffFunc(filePath, targetBranch)
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
