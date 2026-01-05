// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package summarizechanges

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

type fakeSummarizeFactory struct {
	err   error
	calls []*summarizefilechanges.Input
}

func (f *fakeSummarizeFactory) NewActivity() executor.Activity[*summarizefilechanges.Input, *summarizefilechanges.Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *summarizefilechanges.Input) (*summarizefilechanges.Output, error) {
		_ = ctx
		_ = execCtx
		f.calls = append(f.calls, input)
		if f.err != nil {
			return nil, f.err
		}
		return &summarizefilechanges.Output{Filename: input.Filename, Summary: "ok"}, nil
	}
}

func TestNewFactory(t *testing.T) {
	mockFactory := (*summarizefilechanges.Factory)(nil)
	factory, err := NewFactory(mockFactory)
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
	factory, err := NewFactory((*summarizefilechanges.Factory)(nil))
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
	factory, err := NewFactory((*summarizefilechanges.Factory)(nil))
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Changes: []*git.FileChange{},
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	if output.Summaries == nil {
		t.Error("Expected summaries to be non-nil")
	}
}

func TestActivityNoExecutor(t *testing.T) {
	factory, err := NewFactory(&fakeSummarizeFactory{})
	require.NoError(t, err)

	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Changes: []*git.FileChange{{Filename: "a.txt", Change: "diff"}},
	}
	_, err = activity(ctx, execCtx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executor not available")
}

func TestActivitySummariesSuccess(t *testing.T) {
	fake := &fakeSummarizeFactory{}
	factory, err := NewFactory(fake)
	require.NoError(t, err)

	activity := factory.NewActivity()
	ctx := context.Background()
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	input := &Input{
		Changes: []*git.FileChange{
			{Filename: "a.txt", Change: "diff-a"},
			{Filename: "b.txt", Change: "diff-b"},
		},
	}
	output, err := activity(ctx, execCtx, input)
	require.NoError(t, err)
	require.Len(t, output.Summaries, 2)
	assert.Equal(t, "a.txt", output.Summaries[0].Filename)
	assert.Len(t, fake.calls, 2)
}

func TestActivityFactoryNil(t *testing.T) {
	var factory *Factory
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, &Input{Changes: []*git.FileChange{}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "summarize all factory is nil")
}

func TestActivitySummariesError(t *testing.T) {
	fake := &fakeSummarizeFactory{err: errors.New("summarize error")}
	factory, err := NewFactory(fake)
	require.NoError(t, err)

	activity := factory.NewActivity()
	ctx := context.Background()
	exec := executor.NewExecutor(0)
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	input := &Input{
		Changes: []*git.FileChange{{Filename: "a.txt", Change: "diff-a"}},
	}
	_, err = activity(ctx, execCtx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to summarize changes")
}
