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

package listbranchfiles

import (
	"context"
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
