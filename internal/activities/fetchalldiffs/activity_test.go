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

package fetchalldiffs

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/fetchfilediff"
	"github.com/retran/meowg1k/internal/core/git"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockFileDiffActivityFactory is a mock implementation of FileDiffActivityFactory for testing.
type mockFileDiffActivityFactory struct {
	activity executor.Activity[*fetchfilediff.Input, *git.FileChange]
}

func (m *mockFileDiffActivityFactory) NewActivity() executor.Activity[*fetchfilediff.Input, *git.FileChange] {
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
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}

func TestActivitySuccess(t *testing.T) {
	mockActivity := func(ctx context.Context, executorCtx *executor.Context, input *fetchfilediff.Input) (*git.FileChange, error) {
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
	mockExec := executor.NewExecutor()
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
