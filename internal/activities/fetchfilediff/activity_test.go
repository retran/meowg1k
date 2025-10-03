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

// mockGitService is a mock implementation of git.Service for testing.
type mockGitService struct {
	readStagedChangesFunc       func(filePath string) (string, error)
	readStagedFileContentFunc   func(filePath string) (string, error)
	readOriginalFileContentFunc func(filePath string) (string, error)
}

func (m *mockGitService) ReadStagedFiles() ([]string, error) {
	return nil, nil
}

func (m *mockGitService) ReadStagedChanges(filePath string) (string, error) {
	if m.readStagedChangesFunc != nil {
		return m.readStagedChangesFunc(filePath)
	}
	return "", nil
}

func (m *mockGitService) ReadStagedFileContent(filePath string) (string, error) {
	if m.readStagedFileContentFunc != nil {
		return m.readStagedFileContentFunc(filePath)
	}
	return "", nil
}

func (m *mockGitService) ReadOriginalFileContent(filePath string) (string, error) {
	if m.readOriginalFileContentFunc != nil {
		return m.readOriginalFileContentFunc(filePath)
	}
	return "", nil
}

func (m *mockGitService) GetCurrentBranch() (string, error) {
	return "", nil
}

func (m *mockGitService) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
	return nil, nil
}

func (m *mockGitService) GetBranchDiff(filePath, targetBranch string) (string, error) {
	return "", nil
}

func TestNewFactory(t *testing.T) {
	gitSvc := &mockGitService{}
	factory := NewFactory(gitSvc)

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestActivityNilInput(t *testing.T) {
	gitSvc := &mockGitService{}
	factory := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, nil)
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}

func TestActivityInvalidInput(t *testing.T) {
	gitSvc := &mockGitService{}
	factory := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, "invalid")
	if err == nil {
		t.Error("Expected error for invalid input type")
	}
}
