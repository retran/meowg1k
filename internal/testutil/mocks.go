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

// Package testutil provides common test utilities and mocks for testing.
package testutil

import (
	"context"

	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// MockExecutor is a mock implementation of executor.Executor for testing.
type MockExecutor struct{}

// RunActivity implements executor.Executor.
func (m *MockExecutor) RunActivity(ctx context.Context, executorCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any] {
	f := future.NewFuture[any]()
	go func() {
		result, err := activity(ctx, executorCtx, input)
		if err != nil {
			f.CompleteWithError(err)
		} else {
			f.Complete(result)
		}
	}()
	return f
}

// RunFlow implements executor.Executor.
func (m *MockExecutor) RunFlow(ctx context.Context, name string, flow executor.Flow, retryPolicy *executor.RetryPolicy) error {
	return nil
}

// MockGitService is a mock implementation of git.Service for testing.
type MockGitService struct {
	ReadStagedFilesFunc         func() ([]string, error)
	ReadStagedChangesFunc       func(filePath string) (string, error)
	ReadStagedFileContentFunc   func(filePath string) (string, error)
	ReadOriginalFileContentFunc func(filePath string) (string, error)
	GetCurrentBranchFunc        func() (string, error)
	GetChangedFilesInBranchFunc func(targetBranch string) ([]string, error)
	GetBranchDiffFunc           func(filePath, targetBranch string) (string, error)
}

// ReadStagedFiles implements git.Service.
func (m *MockGitService) ReadStagedFiles() ([]string, error) {
	if m.ReadStagedFilesFunc != nil {
		return m.ReadStagedFilesFunc()
	}
	return []string{"file1.go", "file2.go"}, nil
}

// ReadStagedChanges implements git.Service.
func (m *MockGitService) ReadStagedChanges(filePath string) (string, error) {
	if m.ReadStagedChangesFunc != nil {
		return m.ReadStagedChangesFunc(filePath)
	}
	return "test changes", nil
}

// ReadStagedFileContent implements git.Service.
func (m *MockGitService) ReadStagedFileContent(filePath string) (string, error) {
	if m.ReadStagedFileContentFunc != nil {
		return m.ReadStagedFileContentFunc(filePath)
	}
	return "test content", nil
}

// ReadOriginalFileContent implements git.Service.
func (m *MockGitService) ReadOriginalFileContent(filePath string) (string, error) {
	if m.ReadOriginalFileContentFunc != nil {
		return m.ReadOriginalFileContentFunc(filePath)
	}
	return "original content", nil
}

// GetCurrentBranch implements git.Service.
func (m *MockGitService) GetCurrentBranch() (string, error) {
	if m.GetCurrentBranchFunc != nil {
		return m.GetCurrentBranchFunc()
	}
	return "main", nil
}

// GetChangedFilesInBranch implements git.Service.
func (m *MockGitService) GetChangedFilesInBranch(targetBranch string) ([]string, error) {
	if m.GetChangedFilesInBranchFunc != nil {
		return m.GetChangedFilesInBranchFunc(targetBranch)
	}
	return []string{"file1.go", "file2.go"}, nil
}

// GetBranchDiff implements git.Service.
func (m *MockGitService) GetBranchDiff(filePath, targetBranch string) (string, error) {
	if m.GetBranchDiffFunc != nil {
		return m.GetBranchDiffFunc(filePath, targetBranch)
	}
	return "test diff", nil
}

// MockActivityFactory is a mock implementation of executor.ActivityFactory for testing.
type MockActivityFactory struct {
	ActivityFunc func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error)
}

// NewActivity implements executor.ActivityFactory.
func (m *MockActivityFactory) NewActivity() executor.Activity[any, any] {
	if m.ActivityFunc != nil {
		return m.ActivityFunc
	}
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		return nil, nil
	}
}
