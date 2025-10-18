// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package pullrequest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composeflatpr"
	"github.com/retran/meowg1k/internal/activities/composepr"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/domain/pullrequest"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestFactory_NewFlow_EmptyBaseBranch(t *testing.T) {
	mockReader := &mockCommandParametersReader{
		baseBranch: "",
	}

	factory, _ := NewFactory(
		&mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{},
		&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
		&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		&mockPRConfigProvider{},
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	flowCtx := executor.NewContext("test", nil, nil)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error for empty base branch")
	}

	if !strings.Contains(err.Error(), "base branch is required") {
		t.Errorf("expected error about base branch, got: %v", err)
	}
}

func TestFactory_NewFlow_ListBranchFilesError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return nil, errors.New("list files error")
			}
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
	}

	factory, _ := NewFactory(
		mockListFactory,
		&mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{},
		&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		&mockPRConfigProvider{},
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from list branch files")
	}

	if !strings.Contains(err.Error(), "failed to list branch files") {
		t.Errorf("expected error about listing branch files, got: %v", err)
	}
}

func TestFactory_NewFlow_ApplyFiltersError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go", "file2.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return nil, errors.New("filter error")
			}
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		&mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{},
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		&mockPRConfigProvider{},
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from apply filters")
	}

	if !strings.Contains(err.Error(), "failed to apply filters") {
		t.Errorf("expected error about applying filters, got: %v", err)
	}
}

func TestFactory_NewFlow_FetchBranchDiffsError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return nil, errors.New("fetch diffs error")
			}
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		&mockPRConfigProvider{},
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from fetch branch diffs")
	}

	if !strings.Contains(err.Error(), "failed to fetch branch diffs") {
		t.Errorf("expected error about fetching branch diffs, got: %v", err)
	}
}

func TestFactory_NewFlow_PRConfigError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		err: errors.New("config error"),
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		mockConfigProvider,
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from PR config provider")
	}

	if !strings.Contains(err.Error(), "failed to resolve PR configuration") {
		t.Errorf("expected error about PR configuration, got: %v", err)
	}
}

func TestFactory_NewFlow_GetIntentFlagError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "summarize",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intentErr:  errors.New("intent error"),
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		mockConfigProvider,
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from get intent flag")
	}

	if !strings.Contains(err.Error(), "failed to get intent flag") {
		t.Errorf("expected error about intent flag, got: %v", err)
	}
}

func TestFactory_NewFlow_GetStdInError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "summarize",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intent:     "", // Empty intent, should try stdin
		stdinErr:   errors.New("stdin error"),
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		mockConfigProvider,
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from get stdin")
	}

	if !strings.Contains(err.Error(), "failed to get stdin") {
		t.Errorf("expected error about stdin, got: %v", err)
	}
}

func TestFactory_NewFlow_FlatStrategy_Success(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockFlatPRFactory := &mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{
		newActivityFunc: func() executor.Activity[*composeflatpr.Input, *composeflatpr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *composeflatpr.Input) (*composeflatpr.Output, error) {
				return &composeflatpr.Output{Description: "PR Description from flat strategy"}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "flat",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intent:     "Add new feature",
	}

	mockWriter := &mockOutputWriter{}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		mockFlatPRFactory,
		mockConfigProvider,
		mockReader,
		mockWriter,
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mockWriter.outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(mockWriter.outputs))
	}

	if mockWriter.outputs[0] != "PR Description from flat strategy" {
		t.Errorf("expected 'PR Description from flat strategy', got %q", mockWriter.outputs[0])
	}
}

func TestFactory_NewFlow_FlatStrategy_ComposeError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockFlatPRFactory := &mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{
		newActivityFunc: func() executor.Activity[*composeflatpr.Input, *composeflatpr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *composeflatpr.Input) (*composeflatpr.Output, error) {
				return nil, errors.New("compose flat PR error")
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "flat",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intent:     "Add new feature",
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		mockFlatPRFactory,
		mockConfigProvider,
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from compose flat PR")
	}

	if !strings.Contains(err.Error(), "failed to compose PR description using flat strategy") {
		t.Errorf("expected error about flat strategy, got: %v", err)
	}
}

func TestFactory_NewFlow_SummarizeStrategy_Success(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockSummarizeFactory := &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{
		newActivityFunc: func() executor.Activity[*summarizeall.Input, *summarizeall.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *summarizeall.Input) (*summarizeall.Output, error) {
				return &summarizeall.Output{Summaries: []*summarizefile.Output{{Summary: "Added feature X"}}}, nil
			}
		},
	}

	mockComposePRFactory := &mockActivityFactory[*composepr.Input, *composepr.Output]{
		newActivityFunc: func() executor.Activity[*composepr.Input, *composepr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *composepr.Input) (*composepr.Output, error) {
				return &composepr.Output{PRDescription: "PR Description from summarize strategy"}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "summarize",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		stdin:      "Add new feature from stdin",
	}

	mockWriter := &mockOutputWriter{}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		mockSummarizeFactory,
		mockComposePRFactory,
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		mockConfigProvider,
		mockReader,
		mockWriter,
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mockWriter.outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(mockWriter.outputs))
	}

	if mockWriter.outputs[0] != "PR Description from summarize strategy" {
		t.Errorf("expected 'PR Description from summarize strategy', got %q", mockWriter.outputs[0])
	}
}

func TestFactory_NewFlow_SummarizeStrategy_SummarizeError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockSummarizeFactory := &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{
		newActivityFunc: func() executor.Activity[*summarizeall.Input, *summarizeall.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *summarizeall.Input) (*summarizeall.Output, error) {
				return nil, errors.New("summarize error")
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "summarize",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intent:     "Add feature",
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		mockSummarizeFactory,
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		mockConfigProvider,
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from summarize all")
	}

	if !strings.Contains(err.Error(), "failed to summarize changes") {
		t.Errorf("expected error about summarizing changes, got: %v", err)
	}
}

func TestFactory_NewFlow_SummarizeStrategy_ComposeError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockSummarizeFactory := &mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{
		newActivityFunc: func() executor.Activity[*summarizeall.Input, *summarizeall.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *summarizeall.Input) (*summarizeall.Output, error) {
				return &summarizeall.Output{Summaries: []*summarizefile.Output{}}, nil
			}
		},
	}

	mockComposePRFactory := &mockActivityFactory[*composepr.Input, *composepr.Output]{
		newActivityFunc: func() executor.Activity[*composepr.Input, *composepr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *composepr.Input) (*composepr.Output, error) {
				return nil, errors.New("compose PR error")
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "summarize",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intent:     "Add feature",
	}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		mockSummarizeFactory,
		mockComposePRFactory,
		&mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{},
		mockConfigProvider,
		mockReader,
		&mockOutputWriter{},
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from compose PR")
	}

	if !strings.Contains(err.Error(), "failed to compose PR description") {
		t.Errorf("expected error about composing PR description, got: %v", err)
	}
}

// Mock output writer that returns error
type mockOutputWriterWithError struct{}

func (m *mockOutputWriterWithError) PrintLine(line string) error {
	return errors.New("output writer error")
}

func TestFactory_NewFlow_OutputWriterError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]{
		newActivityFunc: func() executor.Activity[*listbranchfiles.Input, *listbranchfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchfiles.Input) (*listbranchfiles.Output, error) {
				return &listbranchfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*applyfilters.Input, *applyfilters.Output]{
		newActivityFunc: func() executor.Activity[*applyfilters.Input, *applyfilters.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *applyfilters.Input) (*applyfilters.Output, error) {
				return &applyfilters.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchallbranchdiffs.Input) (*fetchallbranchdiffs.Output, error) {
				return &fetchallbranchdiffs.Output{}, nil
			}
		},
	}

	mockFlatPRFactory := &mockActivityFactory[*composeflatpr.Input, *composeflatpr.Output]{
		newActivityFunc: func() executor.Activity[*composeflatpr.Input, *composeflatpr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *composeflatpr.Input) (*composeflatpr.Output, error) {
				return &composeflatpr.Output{Description: "PR Description"}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &pullrequest.ResolvedConfig{
			Strategy: "flat",
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
		intent:     "Add feature",
	}

	mockWriter := &mockOutputWriterWithError{}

	factory, _ := NewFactory(
		mockListFactory,
		mockFilterFactory,
		mockDiffFactory,
		&mockActivityFactory[*summarizeall.Input, *summarizeall.Output]{},
		&mockActivityFactory[*composepr.Input, *composepr.Output]{},
		mockFlatPRFactory,
		mockConfigProvider,
		mockReader,
		mockWriter,
	)

	ctx := context.Background()
	exec := executor.NewExecutor(1)
	flowCtx := executor.NewContext("test", nil, exec)

	flow := factory.NewFlow()
	err := flow(ctx, flowCtx)

	if err == nil {
		t.Fatal("expected error from output writer")
	}

	if !strings.Contains(err.Error(), "failed to print PR description") {
		t.Errorf("expected error about printing PR description, got: %v", err)
	}
}
