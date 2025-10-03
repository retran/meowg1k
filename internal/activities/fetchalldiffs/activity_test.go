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

	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/internal/testutil"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory(nil)
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestActivityNilInput(t *testing.T) {
	factory := NewFactory(nil)
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	_, err := activity(ctx, execCtx, nil)
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}

func TestActivityInvalidInput(t *testing.T) {
	factory := NewFactory(nil)
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	_, err := activity(ctx, execCtx, "invalid")
	if err == nil {
		t.Error("Expected error for invalid input type")
	}
}

func TestActivitySuccess(t *testing.T) {
	mockExec := &testutil.MockExecutor{}
	mockFetchDiffFactory := &testutil.MockActivityFactory{
		ActivityFunc: func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
			return &git.FileChange{Filename: "test.go"}, nil
		},
	}

	factory := &Factory{
		fetchFileDiffActivityFactory: mockFetchDiffFactory,
	}
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, mockExec)

	input := &Input{
		Files: []string{"file1.go", "file2.go"},
	}

	result, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	output, ok := result.(*Output)
	if !ok {
		t.Fatalf("Expected *Output, got %T", result)
	}

	if len(output.Changes) == 0 {
		t.Error("Expected changes, got empty array")
	}
}
