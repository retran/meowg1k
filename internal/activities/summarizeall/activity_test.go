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

package summarizeall

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// mockExecutor is a mock implementation of the executor for testing.
type mockExecutor struct{}

func (m *mockExecutor) RunActivity(ctx context.Context, executorCtx *executor.Context, name string, activity executor.Activity[any, any], input any) *future.Future[any] {
	f := future.NewFuture[any]()
	result, err := activity(ctx, executorCtx, input)
	if err != nil {
		f.CompleteWithError(err)
	} else {
		f.Complete(result)
	}
	return f
}

func (m *mockExecutor) RunFlow(ctx context.Context, name string, flow executor.Flow, retryPolicy *executor.RetryPolicy) error {
	flowCtx := executor.NewContext(name, nil, m)
	return flow(ctx, flowCtx)
}

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

type mockSummarizeFactory struct{}

func (m *mockSummarizeFactory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, input any) (any, error) {
		return &summarizefile.Output{
			Filename: "test.go",
			Summary:  "test summary",
			Skipped:  false,
		}, nil
	}
}

func TestActivitySuccess(t *testing.T) {
	mockFactory := &mockSummarizeFactory{}
	factory := NewFactory((*summarizefile.Factory)(nil))
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, &mockExecutor{})

	input := &Input{
		Changes: []*git.FileChange{},
	}

	result, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	output, ok := result.(*Output)
	if !ok {
		t.Errorf("Expected *Output, got %T", result)
	}

	if output.Summaries == nil {
		t.Error("Expected summaries to be non-nil")
	}

	_ = mockFactory
}
