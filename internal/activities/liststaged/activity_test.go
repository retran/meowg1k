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

	"github.com/retran/meowg1k/internal/testutil"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestNewFactory(t *testing.T) {
	gitSvc := &testutil.MockGitService{}
	factory := NewFactory(gitSvc)

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestNewActivity(t *testing.T) {
	gitSvc := &testutil.MockGitService{}
	factory := NewFactory(gitSvc)
	activity := factory.NewActivity()

	if activity == nil {
		t.Error("NewActivity returned nil")
	}
}

func TestActivityExecute(t *testing.T) {
	gitSvc := &testutil.MockGitService{
		ReadStagedFilesFunc: func() ([]string, error) {
			return []string{"file1.txt", "file2.go"}, nil
		},
	}
	factory := NewFactory(gitSvc)
	activity := factory.NewActivity()

	input := &Input{}

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity execution failed: %v", err)
	}

	out, ok := output.(*Output)
	if !ok {
		t.Errorf("Expected output to be *Output, got %T", output)
	}

	expected := []string{"file1.txt", "file2.go"}
	if len(out.Files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(out.Files))
	}

	for i, file := range expected {
		if i >= len(out.Files) || out.Files[i] != file {
			t.Errorf("Expected file %s at position %d, got %v", file, i, out.Files)
		}
	}
}

func TestActivityExecuteNilInput(t *testing.T) {
	gitSvc := &testutil.MockGitService{}
	factory := NewFactory(gitSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, nil)
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}

func TestActivityExecuteInvalidInput(t *testing.T) {
	gitSvc := &testutil.MockGitService{}
	factory := NewFactory(gitSvc)
	activity := factory.NewActivity()

	input := "invalid input"

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, input)
	if err == nil {
		t.Error("Expected error for invalid input type")
	}
}
