// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package pr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/draftprflat"
	"github.com/retran/meowg1k/internal/activities/draftpr"
	"github.com/retran/meowg1k/internal/activities/fetchbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/listbranchchanges"
	"github.com/retran/meowg1k/internal/activities/summarizechanges"
	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	domainpullrequest "github.com/retran/meowg1k/internal/domain/pullrequest"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestFactory_NewFlow_EmptyBaseBranch(t *testing.T) {
	mockReader := &mockCommandParametersReader{
		baseBranch: "",
	}

	factory, _ := NewFactory(
		&mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{},
		&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
		&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
		t.Fatal("expected error for empty base branch")
	}

	if !strings.Contains(err.Error(), "base branch is required") {
		t.Errorf("expected error about base branch, got: %v", err)
	}
}

func TestFactory_NewFlow_ListBranchFilesError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return nil, errors.New("list files error")
			}
		},
	}

	mockReader := &mockCommandParametersReader{
		baseBranch: "main",
	}

	factory, _ := NewFactory(
		mockListFactory,
		&mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{},
		&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go", "file2.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
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
		&mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{},
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockFlatPRFactory := &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{
		newActivityFunc: func() executor.Activity[*draftprflat.Input, *draftprflat.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *draftprflat.Input) (*draftprflat.Output, error) {
				return &draftprflat.Output{Description: "PR Description from flat strategy"}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockFlatPRFactory := &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{
		newActivityFunc: func() executor.Activity[*draftprflat.Input, *draftprflat.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *draftprflat.Input) (*draftprflat.Output, error) {
				return nil, errors.New("compose flat PR error")
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockSummarizeFactory := &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{
		newActivityFunc: func() executor.Activity[*summarizechanges.Input, *summarizechanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *summarizechanges.Input) (*summarizechanges.Output, error) {
				return &summarizechanges.Output{Summaries: []*summarizefilechanges.Output{{Summary: "Added feature X"}}}, nil
			}
		},
	}

	mockComposePRFactory := &mockActivityFactory[*draftpr.Input, *draftpr.Output]{
		newActivityFunc: func() executor.Activity[*draftpr.Input, *draftpr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *draftpr.Input) (*draftpr.Output, error) {
				return &draftpr.Output{PRDescription: "PR Description from summarize strategy"}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockSummarizeFactory := &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{
		newActivityFunc: func() executor.Activity[*summarizechanges.Input, *summarizechanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *summarizechanges.Input) (*summarizechanges.Output, error) {
				return nil, errors.New("summarize error")
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockSummarizeFactory := &mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{
		newActivityFunc: func() executor.Activity[*summarizechanges.Input, *summarizechanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *summarizechanges.Input) (*summarizechanges.Output, error) {
				return &summarizechanges.Output{Summaries: []*summarizefilechanges.Output{}}, nil
			}
		},
	}

	mockComposePRFactory := &mockActivityFactory[*draftpr.Input, *draftpr.Output]{
		newActivityFunc: func() executor.Activity[*draftpr.Input, *draftpr.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *draftpr.Input) (*draftpr.Output, error) {
				return nil, errors.New("compose PR error")
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{},
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

// Mock output writer that returns error.
type mockOutputWriterWithError struct{}

func (m *mockOutputWriterWithError) PrintLine(line string) error {
	return errors.New("output writer error")
}

func TestFactory_NewFlow_OutputWriterError(t *testing.T) {
	mockListFactory := &mockActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]{
		newActivityFunc: func() executor.Activity[*listbranchchanges.Input, *listbranchchanges.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *listbranchchanges.Input) (*listbranchchanges.Output, error) {
				return &listbranchchanges.Output{Files: []string{"file1.go"}}, nil
			}
		},
	}

	mockFilterFactory := &mockActivityFactory[*filterfiles.Input, *filterfiles.Output]{
		newActivityFunc: func() executor.Activity[*filterfiles.Input, *filterfiles.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *filterfiles.Input) (*filterfiles.Output, error) {
				return &filterfiles.Output{Files: input.Files}, nil
			}
		},
	}

	mockDiffFactory := &mockActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]{
		newActivityFunc: func() executor.Activity[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *fetchbranchdiffs.Input) (*fetchbranchdiffs.Output, error) {
				return &fetchbranchdiffs.Output{}, nil
			}
		},
	}

	mockFlatPRFactory := &mockActivityFactory[*draftprflat.Input, *draftprflat.Output]{
		newActivityFunc: func() executor.Activity[*draftprflat.Input, *draftprflat.Output] {
			return func(ctx context.Context, activityCtx *executor.Context, input *draftprflat.Input) (*draftprflat.Output, error) {
				return &draftprflat.Output{Description: "PR Description"}, nil
			}
		},
	}

	mockConfigProvider := &mockPRConfigProvider{
		config: &domainpullrequest.ResolvedConfig{
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
		&mockActivityFactory[*summarizechanges.Input, *summarizechanges.Output]{},
		&mockActivityFactory[*draftpr.Input, *draftpr.Output]{},
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
